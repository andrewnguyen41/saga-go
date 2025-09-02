package saga

import (
	"context"
	"log"
	"time"
)

// RecoveryManager handles recovery of stuck/failed steps
type RecoveryManager struct {
	storage     Storage
	pubsub      PubSub
	interval    time.Duration
	stepTimeout time.Duration
	running     bool
	stopCh      chan struct{}
}

func NewRecoveryManager(storage Storage, pubsub PubSub) *RecoveryManager {
	return &RecoveryManager{
		storage:     storage,
		pubsub:      pubsub,
		interval:    5 * time.Second,  // Check every 5 seconds for demo
		stepTimeout: 10 * time.Second, // Consider step stuck after 10 seconds for demo
		stopCh:      make(chan struct{}),
	}
}

// Start begins the recovery process
func (r *RecoveryManager) Start(ctx context.Context) {
	if r.running {
		return
	}

	r.running = true
	go r.recoveryLoop(ctx)
}

// Stop stops the recovery process
func (r *RecoveryManager) Stop() {
	if !r.running {
		return
	}

	r.running = false
	close(r.stopCh)
}

func (r *RecoveryManager) recoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.recoverStuckSteps(ctx)
		}
	}
}

func (r *RecoveryManager) recoverStuckSteps(ctx context.Context) {
	stuckSteps, err := r.storage.GetStuckSteps(ctx, r.stepTimeout)
	if err != nil {
		log.Printf("Failed to get stuck steps: %v", err)
		return
	}

	for _, step := range stuckSteps {
		var reason string

		switch step.Status {
		case StatusPending:
			reason = "pending too long"
		case StatusProcessing:
			reason = "processing too long"

			// Reset to pending so it can be picked up again
			step.Status = StatusPending
			step.StartedAt = nil
			r.storage.UpdateStep(ctx, &step)
		}

		log.Printf("Recovering stuck step: %s (saga: %s) - %s", step.ID, step.SagaID, reason)

		// Re-publish the step execution message
		msg := Message{
			Type:   "step_execute",
			SagaID: step.SagaID,
			StepID: step.ID,
		}

		if err := r.pubsub.Publish(ctx, "saga_events", msg); err != nil {
			log.Printf("Failed to republish step %s: %v", step.ID, err)
		}
	}
}
