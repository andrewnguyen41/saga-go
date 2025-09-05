package main

import (
	"context"
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

	// Create saga with inline handlers
	sagaInstance, err := saga.NewBuilder("order_fulfillment", orchestrator).
		Step("create_order",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("Creating order for user: %v\n", data["user_id"])
				// Simulate work
				time.Sleep(100 * time.Millisecond)
				data["order_id"] = "12345"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("Canceling order: %v\n", data["order_id"])
				return nil
			},
		).
		Step("charge_payment",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("Charging payment for order: %v\n", data["order_id"])
				// Simulate work
				time.Sleep(100 * time.Millisecond)
				data["payment_id"] = "pay_67890"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("Refunding payment: %v\n", data["payment_id"])
				return nil
			},
		).
		Step("ship_order",
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("Shipping order: %v\n", data["order_id"])
				// Simulate work
				time.Sleep(100 * time.Millisecond)
				data["tracking_id"] = "track_abc123"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				fmt.Printf("Canceling shipment: %v\n", data["tracking_id"])
				return nil
			},
		).
		WithData("user_id", "user_123").
		WithData("amount", 99.99).
		Execute(ctx)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Started saga: %s\n", sagaInstance.ID)

	// Wait for saga to complete
	time.Sleep(2 * time.Second)

	// Check final status
	finalSaga, _ := storage.GetSaga(ctx, sagaInstance.ID)
	fmt.Printf("Final saga status: %s\n", finalSaga.Status)

	for _, step := range finalSaga.Steps {
		fmt.Printf("Step %s: %s\n", step.Name, step.Status)
	}
}
