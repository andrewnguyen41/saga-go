package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	saga "github.com/andrewnguyen41/saga-go"
)

func main() {
	// Create storage and pubsub
	storage := saga.NewMemoryStorage()
	pubsub := saga.NewMemoryPubSub()
	defer pubsub.Close()

	// Create orchestrator
	orchestrator := saga.NewOrchestrator(storage, pubsub)

	// Start listener
	ctx := context.Background()
	orchestrator.StartListener(ctx)

	fmt.Println("Starting saga with planned failure...")

	// Create and execute saga that will fail
	sagaInstance, err := saga.NewBuilder("failed_order", orchestrator).
		Step("reserve_inventory",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Println("✓ Reserving inventory")
				data["reserved_items"] = []string{"item1", "item2"}
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Println("✗ Releasing reserved inventory")
				return nil
			},
		).
		Step("charge_card",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Println("✓ Charging credit card")
				data["charge_id"] = "ch_12345"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Println("✗ Refunding credit card charge")
				return nil
			},
		).
		Step("ship_items",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Println("✗ Shipping failed - warehouse is closed!")
				return errors.New("warehouse closed")
			},
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Println("✗ Canceling shipment (nothing to cancel)")
				return nil
			},
		).
		WithData("order_id", "order_789").
		Execute(ctx)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Started saga: %s\n", sagaInstance.ID)

	// Wait for saga to complete (with failure and compensation)
	time.Sleep(2 * time.Second)

	// Check final status
	finalSaga, _ := storage.GetSaga(ctx, sagaInstance.ID)
	fmt.Printf("\n📊 Final saga status: %s\n", finalSaga.Status)

	fmt.Println("\n📝 Step results:")
	for _, step := range finalSaga.Steps {
		statusIcon := "✓"
		if step.Status == saga.StatusFailed {
			statusIcon = "✗"
		} else if step.Status == saga.StatusCompensated {
			statusIcon = "↩"
		}
		fmt.Printf("  %s %s: %s", statusIcon, step.Name, step.Status)
		if step.Error != "" {
			fmt.Printf(" (error: %s)", step.Error)
		}
		fmt.Println()
	}
}
