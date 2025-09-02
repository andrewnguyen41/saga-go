package saga

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MemoryStorage implements Storage interface using in-memory maps
type MemoryStorage struct {
	mu    sync.RWMutex
	sagas map[string]*Saga
	steps map[string]*Step
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		sagas: make(map[string]*Saga),
		steps: make(map[string]*Step),
	}
}

func (m *MemoryStorage) SaveSaga(ctx context.Context, saga *Saga) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	saga.UpdatedAt = time.Now()
	if saga.CreatedAt.IsZero() {
		saga.CreatedAt = time.Now()
	}
	
	m.sagas[saga.ID] = saga
	
	// Also save steps
	for i := range saga.Steps {
		step := &saga.Steps[i]
		if step.CreatedAt.IsZero() {
			step.CreatedAt = time.Now()
		}
		step.UpdatedAt = time.Now()
		m.steps[step.ID] = step
	}
	
	return nil
}

func (m *MemoryStorage) GetSaga(ctx context.Context, id string) (*Saga, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	saga, exists := m.sagas[id]
	if !exists {
		return nil, errors.New("saga not found")
	}
	
	return saga, nil
}

func (m *MemoryStorage) UpdateStep(ctx context.Context, step *Step) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	step.UpdatedAt = time.Now()
	m.steps[step.ID] = step
	
	// Update step in saga
	if saga, exists := m.sagas[step.SagaID]; exists {
		for i := range saga.Steps {
			if saga.Steps[i].ID == step.ID {
				saga.Steps[i] = *step
				break
			}
		}
		saga.UpdatedAt = time.Now()
	}
	
	return nil
}

func (m *MemoryStorage) GetStep(ctx context.Context, id string) (*Step, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	step, exists := m.steps[id]
	if !exists {
		return nil, errors.New("step not found")
	}
	
	return step, nil
}

func (m *MemoryStorage) GetPendingSteps(ctx context.Context) ([]Step, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var pending []Step
	for _, step := range m.steps {
		if step.Status == StatusPending {
			pending = append(pending, *step)
		}
	}
	
	return pending, nil
}

func (m *MemoryStorage) GetStuckSteps(ctx context.Context, timeout time.Duration) ([]Step, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var stuck []Step
	now := time.Now()
	
	for _, step := range m.steps {
		switch step.Status {
		case StatusPending:
			// Step never started processing
			if now.Sub(step.UpdatedAt) > timeout {
				stuck = append(stuck, *step)
			}
		case StatusProcessing:
			// Step started but may have crashed
			if step.StartedAt != nil && now.Sub(*step.StartedAt) > timeout {
				stuck = append(stuck, *step)
			}
		}
	}
	
	return stuck, nil
}