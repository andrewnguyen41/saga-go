package saga

import (
	"context"
	"time"
)

// Step status
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCompensated Status = "compensated"
)

// Step represents a single step in a saga
type Step struct {
	ID           string                 `json:"id"`
	SagaID       string                 `json:"saga_id"`
	Name         string                 `json:"name"`
	Status       Status                 `json:"status"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Error        string                 `json:"error,omitempty"`
	CompensateID string                 `json:"compensate_id,omitempty"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Saga represents a saga transaction
type Saga struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    Status    `json:"status"`
	Steps     []Step    `json:"steps"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StepHandler defines how to execute and compensate a step
type StepHandler interface {
	Execute(ctx context.Context, data map[string]interface{}) error
	Compensate(ctx context.Context, data map[string]interface{}) error
}

// StepFunc is a simple function-based step handler
type StepFunc struct {
	ExecFn       func(ctx context.Context, data map[string]interface{}) error
	CompensateFn func(ctx context.Context, data map[string]interface{}) error
}

func (f StepFunc) Execute(ctx context.Context, data map[string]interface{}) error {
	if f.ExecFn == nil {
		return nil
	}
	return f.ExecFn(ctx, data)
}

func (f StepFunc) Compensate(ctx context.Context, data map[string]interface{}) error {
	if f.CompensateFn == nil {
		return nil
	}
	return f.CompensateFn(ctx, data)
}

// Message represents pub/sub messages
type Message struct {
	Type   string                 `json:"type"`
	SagaID string                 `json:"saga_id"`
	StepID string                 `json:"step_id"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

// Storage interface for saga persistence
type Storage interface {
	SaveSaga(ctx context.Context, saga *Saga) error
	GetSaga(ctx context.Context, id string) (*Saga, error)
	UpdateStep(ctx context.Context, step *Step) error
	GetStep(ctx context.Context, id string) (*Step, error)
	GetPendingSteps(ctx context.Context) ([]Step, error)
	GetStuckSteps(ctx context.Context, timeout time.Duration) ([]Step, error)
}

// PubSub interface for messaging
type PubSub interface {
	Publish(ctx context.Context, topic string, msg Message) error
	Subscribe(ctx context.Context, topic string, handler func(Message)) error
	Close() error
}