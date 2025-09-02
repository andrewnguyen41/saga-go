package saga

import (
	"context"
	"fmt"
)

// Builder allows defining handlers inline with steps
type Builder struct {
	name         string
	steps        []builderStep
	data         map[string]interface{}
	orchestrator *Orchestrator
}

type builderStep struct {
	name    string
	handler StepHandler
}

// NewBuilder creates a builder that registers handlers automatically
func NewBuilder(name string, orchestrator *Orchestrator) *Builder {
	return &Builder{
		name:         name,
		data:         make(map[string]interface{}),
		orchestrator: orchestrator,
	}
}

// Step adds a step with inline handler definition
func (b *Builder) Step(
	name string,
	execute func(ctx context.Context, data map[string]interface{}) error,
	compensate func(ctx context.Context, data map[string]interface{}) error,
) *Builder {
	handler := NewStepHandler(execute, compensate)
	b.steps = append(b.steps, builderStep{
		name:    name,
		handler: handler,
	})
	return b
}

// WithData adds data to the saga context
func (b *Builder) WithData(key string, value interface{}) *Builder {
	b.data[key] = value
	return b
}

// Execute registers all handlers and starts the saga
func (b *Builder) Execute(ctx context.Context) (*Saga, error) {
	if len(b.steps) == 0 {
		return nil, fmt.Errorf("saga must have at least one step")
	}

	// Auto-register all handlers
	stepNames := make([]string, len(b.steps))
	for i, step := range b.steps {
		b.orchestrator.RegisterHandler(step.name, step.handler)
		stepNames[i] = step.name
	}

	// Start the saga
	return b.orchestrator.StartSaga(ctx, b.name, stepNames, b.data)
}

// Helper function to create a simple step handler
func NewStepHandler(
	execute func(ctx context.Context, data map[string]interface{}) error,
	compensate func(ctx context.Context, data map[string]interface{}) error,
) StepHandler {
	return StepFunc{
		ExecFn:       execute,
		CompensateFn: compensate,
	}
}
