package main

import (
	"context"
	"fmt"
	"log"
	"time"

	saga "github.com/andrewnguyen41/saga-go"
)

func main() {
	// Shared storage (in real world, this would be a database)
	storage := saga.NewMemoryStorage()
	pubsub := saga.NewMemoryPubSub()
	defer pubsub.Close()

	fmt.Println("🚀 Starting Service Instance 1")
	orchestrator1 := saga.NewOrchestrator(storage, pubsub)

	// Instance 1 only handles step1
	orchestrator1.RegisterHandler("step1", saga.NewStepHandler(
		func(ctx context.Context, data map[string]interface{}) error {
			fmt.Printf("✓ [Instance 1] Executing step1\n")
			data["step1_result"] = "completed by instance 1"
			return nil
		},
		func(ctx context.Context, data map[string]interface{}) error {
			return nil
		},
	))

	orchestrator1.StartListener(context.Background())

	// Create recovery manager with short timeout for demo
	recovery1 := saga.NewRecoveryManager(storage, pubsub)
	recovery1.Start(context.Background())

	// Start saga
	fmt.Println("📝 Creating saga with 3 steps...")
	sagaInstance, err := saga.NewBuilder("recovery_test", orchestrator1).
		Step("step1",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("✓ Executing step1\n")
				data["step1_result"] = "completed"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				return nil
			},
		).
		Step("step2",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("✓ Executing step2\n")
				data["step2_result"] = "completed"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				return nil
			},
		).
		Step("step3",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("✓ Executing step3\n")
				data["step3_result"] = "completed"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				return nil
			},
		).
		Execute(context.Background())

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Started saga: %s\n", sagaInstance.ID)

	// Wait for step1 to complete
	time.Sleep(2 * time.Second)

	// Check saga status
	midSaga, _ := storage.GetSaga(context.Background(), sagaInstance.ID)
	fmt.Printf("\n� After step1 - saga status: %s\n", midSaga.Status)
	for _, step := range midSaga.Steps {
		fmt.Printf("  %s: %s\n", step.Name, step.Status)
	}

	// Simulate instance 1 going down (stop listening)
	fmt.Println("\n💥 Instance 1 stops responding (no handlers for step2/step3)")

	// Step2 will be stuck in pending state since no handler
	fmt.Println("⏳ Waiting for recovery timeout...")
	time.Sleep(8 * time.Second)

	// Start instance 2
	fmt.Println("\n🔄 Starting Service Instance 2 with handlers for step2 and step3")
	orchestrator2 := saga.NewOrchestrator(storage, pubsub)

	orchestrator2.RegisterHandler("step2", saga.NewStepHandler(
		func(ctx context.Context, data map[string]interface{}) error {
			fmt.Printf("✓ [Instance 2] Executing step2 (recovered!)\n")
			data["step2_result"] = "completed by instance 2"
			return nil
		},
		func(ctx context.Context, data map[string]interface{}) error {
			return nil
		},
	))

	orchestrator2.RegisterHandler("step3", saga.NewStepHandler(
		func(ctx context.Context, data map[string]interface{}) error {
			fmt.Printf("✓ [Instance 2] Executing step3\n")
			data["step3_result"] = "completed by instance 2"
			return nil
		},
		func(ctx context.Context, data map[string]interface{}) error {
			return nil
		},
	))

	orchestrator2.StartListener(context.Background())

	// Recovery will trigger and instance 2 will pick up the work
	fmt.Println("⏳ Waiting for recovery and completion...")
	time.Sleep(5 * time.Second)

	// Check final status
	finalSaga, _ := storage.GetSaga(context.Background(), sagaInstance.ID)
	fmt.Printf("\n📊 Final saga status: %s\n", finalSaga.Status)

	for _, step := range finalSaga.Steps {
		statusIcon := "✓"
		if step.Status == saga.StatusFailed {
			statusIcon = "✗"
		}
		fmt.Printf("  %s %s: %s\n", statusIcon, step.Name, step.Status)
	}

	recovery1.Stop()
}
