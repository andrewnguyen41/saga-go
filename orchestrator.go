package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Orchestrator manages saga execution
type Orchestrator struct {
	storage  Storage
	pubsub   PubSub
	handlers map[string]StepHandler
}

func NewOrchestrator(storage Storage, pubsub PubSub) *Orchestrator {
	return &Orchestrator{
		storage:  storage,
		pubsub:   pubsub,
		handlers: make(map[string]StepHandler),
	}
}

// RegisterHandler registers a step handler
func (o *Orchestrator) RegisterHandler(stepName string, handler StepHandler) {
	o.handlers[stepName] = handler
}

// StartSaga creates and starts a new saga
func (o *Orchestrator) StartSaga(ctx context.Context, name string, steps []string, data map[string]interface{}) (*Saga, error) {
	sagaID := uuid.New().String()

	saga := &Saga{
		ID:        sagaID,
		Name:      name,
		Status:    StatusPending,
		Data:      data,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create steps
	for _, stepName := range steps {
		stepID := uuid.New().String()
		step := Step{
			ID:        stepID,
			SagaID:    sagaID,
			Name:      stepName,
			Status:    StatusPending,
			Data:      make(map[string]interface{}),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		saga.Steps = append(saga.Steps, step)
	}

	if err := o.storage.SaveSaga(ctx, saga); err != nil {
		return nil, fmt.Errorf("failed to save saga: %w", err)
	}

	// Start executing first step
	if len(saga.Steps) > 0 {
		msg := Message{
			Type:   "step_execute",
			SagaID: sagaID,
			StepID: saga.Steps[0].ID,
			Data:   data,
		}
		o.pubsub.Publish(ctx, "saga_events", msg)
	}

	return saga, nil
}

// ExecuteStep executes a specific step
func (o *Orchestrator) ExecuteStep(ctx context.Context, stepID string) error {
	step, err := o.storage.GetStep(ctx, stepID)
	if err != nil {
		return fmt.Errorf("failed to get step: %w", err)
	}

	if step.Status != StatusPending {
		return nil // Already processed or processing
	}

	handler, exists := o.handlers[step.Name]
	if !exists {
		return fmt.Errorf("no handler for step: %s", step.Name)
	}

	// Mark step as processing
	now := time.Now()
	step.Status = StatusProcessing
	step.StartedAt = &now
	if err := o.storage.UpdateStep(ctx, step); err != nil {
		return fmt.Errorf("failed to mark step as processing: %w", err)
	}

	saga, err := o.storage.GetSaga(ctx, step.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get saga: %w", err)
	}

	// Merge saga data with step data
	execData := make(map[string]interface{})
	for k, v := range saga.Data {
		execData[k] = v
	}
	for k, v := range step.Data {
		execData[k] = v
	}

	err = handler.Execute(ctx, execData)
	if err != nil {
		// Mark step as failed
		step.Status = StatusFailed
		step.Error = err.Error()
		o.storage.UpdateStep(ctx, step)

		// Start compensation
		o.startCompensation(ctx, saga)
		return nil
	}

	// Mark step as completed and save any data changes
	step.Status = StatusCompleted
	step.Data = execData
	o.storage.UpdateStep(ctx, step)

	// Update saga data with step results
	for k, v := range execData {
		saga.Data[k] = v
	}
	o.storage.SaveSaga(ctx, saga)

	// Continue to next step or complete saga
	o.continueOrComplete(ctx, saga)

	return nil
}

// CompensateStep compensates a specific step
func (o *Orchestrator) CompensateStep(ctx context.Context, stepID string) error {
	step, err := o.storage.GetStep(ctx, stepID)
	if err != nil {
		return fmt.Errorf("failed to get step: %w", err)
	}

	if step.Status != StatusCompleted {
		return nil // Nothing to compensate
	}

	handler, exists := o.handlers[step.Name]
	if !exists {
		return fmt.Errorf("no handler for step: %s", step.Name)
	}

	saga, err := o.storage.GetSaga(ctx, step.SagaID)
	if err != nil {
		return fmt.Errorf("failed to get saga: %w", err)
	}

	// Merge saga data with step data
	execData := make(map[string]interface{})
	for k, v := range saga.Data {
		execData[k] = v
	}
	for k, v := range step.Data {
		execData[k] = v
	}

	err = handler.Compensate(ctx, execData)
	if err != nil {
		step.Error = err.Error()
	}

	step.Status = StatusCompensated
	o.storage.UpdateStep(ctx, step)

	return nil
}

// StartListener starts listening for saga events
func (o *Orchestrator) StartListener(ctx context.Context) error {
	return o.pubsub.Subscribe(ctx, "saga_events", func(msg Message) {
		switch msg.Type {
		case "step_execute":
			o.ExecuteStep(ctx, msg.StepID)
		case "step_compensate":
			o.CompensateStep(ctx, msg.StepID)
		}
	})
}

func (o *Orchestrator) continueOrComplete(ctx context.Context, saga *Saga) {
	// Find current step index
	currentIndex := -1
	for i, step := range saga.Steps {
		if step.Status == StatusCompleted {
			currentIndex = i
		} else {
			break
		}
	}

	// Check if there's a next step
	if currentIndex+1 < len(saga.Steps) {
		nextStep := saga.Steps[currentIndex+1]
		msg := Message{
			Type:   "step_execute",
			SagaID: saga.ID,
			StepID: nextStep.ID,
			Data:   saga.Data,
		}
		o.pubsub.Publish(ctx, "saga_events", msg)
	} else {
		// All steps completed, mark saga as completed
		saga.Status = StatusCompleted
		o.storage.SaveSaga(ctx, saga)
	}
}

func (o *Orchestrator) startCompensation(ctx context.Context, saga *Saga) {
	saga.Status = StatusFailed
	o.storage.SaveSaga(ctx, saga)

	// Compensate completed steps in reverse order
	for i := len(saga.Steps) - 1; i >= 0; i-- {
		step := saga.Steps[i]
		if step.Status == StatusCompleted {
			msg := Message{
				Type:   "step_compensate",
				SagaID: saga.ID,
				StepID: step.ID,
				Data:   saga.Data,
			}
			o.pubsub.Publish(ctx, "saga_events", msg)
		}
	}
}
