package saga

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSagaSuccess(t *testing.T) {
	storage := NewMemoryStorage()
	pubsub := NewMemoryPubSub()
	defer pubsub.Close()

	orchestrator := NewOrchestrator(storage, pubsub)
	orchestrator.StartListener(context.Background())

	// Execute saga using inline builder
	sagaInstance, err := NewBuilder("test_saga", orchestrator).
		Step("step1",
			func(ctx context.Context, data map[string]interface{}) error {
				data["step1_result"] = "done"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				return nil
			},
		).
		Step("step2",
			func(ctx context.Context, data map[string]interface{}) error {
				data["step2_result"] = "done"
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				return nil
			},
		).
		WithData("input", "test").
		Execute(context.Background())

	if err != nil {
		t.Fatalf("Failed to start saga: %v", err)
	}

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	finalSaga, err := storage.GetSaga(context.Background(), sagaInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get saga: %v", err)
	}

	if finalSaga.Status != StatusCompleted {
		t.Errorf("Expected saga status to be completed, got %s", finalSaga.Status)
	}

	if len(finalSaga.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(finalSaga.Steps))
	}

	for _, step := range finalSaga.Steps {
		if step.Status != StatusCompleted {
			t.Errorf("Expected step %s to be completed, got %s", step.Name, step.Status)
		}
	}
}

func TestSagaFailureAndCompensation(t *testing.T) {
	storage := NewMemoryStorage()
	pubsub := NewMemoryPubSub()
	defer pubsub.Close()

	orchestrator := NewOrchestrator(storage, pubsub)
	orchestrator.StartListener(context.Background())

	// Execute saga that will fail
	sagaInstance, err := NewBuilder("failing_saga", orchestrator).
		Step("step1",
			func(ctx context.Context, data map[string]interface{}) error {
				data["step1_done"] = true
				return nil
			},
			func(ctx context.Context, data map[string]interface{}) error {
				data["step1_compensated"] = true
				return nil
			},
		).
		Step("failing_step",
			func(ctx context.Context, data map[string]interface{}) error {
				return errors.New("intentional failure")
			},
			func(ctx context.Context, data map[string]interface{}) error {
				return nil
			},
		).
		WithData("input", "test").
		Execute(context.Background())

	if err != nil {
		t.Fatalf("Failed to start saga: %v", err)
	}

	// Wait for completion and compensation
	time.Sleep(500 * time.Millisecond)

	finalSaga, err := storage.GetSaga(context.Background(), sagaInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get saga: %v", err)
	}

	if finalSaga.Status != StatusFailed {
		t.Errorf("Expected saga status to be failed, got %s", finalSaga.Status)
	}

	if len(finalSaga.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(finalSaga.Steps))
	}

	// First step should be compensated
	if finalSaga.Steps[0].Status != StatusCompensated {
		t.Errorf("Expected first step to be compensated, got %s", finalSaga.Steps[0].Status)
	}

	// Second step should be failed
	if finalSaga.Steps[1].Status != StatusFailed {
		t.Errorf("Expected second step to be failed, got %s", finalSaga.Steps[1].Status)
	}
}
