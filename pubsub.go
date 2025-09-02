package saga

import (
	"context"
	"sync"
)

// MemoryPubSub implements PubSub interface using in-memory channelss
type MemoryPubSub struct {
	mu          sync.RWMutex
	subscribers map[string][]func(Message)
	closed      bool
}

func NewMemoryPubSub() *MemoryPubSub {
	return &MemoryPubSub{
		subscribers: make(map[string][]func(Message)),
	}
}

func (m *MemoryPubSub) Publish(ctx context.Context, topic string, msg Message) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil
	}

	handlers, exists := m.subscribers[topic]
	if !exists {
		return nil
	}

	// Call handlers in separate goroutines to avoid blocking
	for _, handler := range handlers {
		go handler(msg)
	}

	return nil
}

func (m *MemoryPubSub) Subscribe(ctx context.Context, topic string, handler func(Message)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.subscribers[topic] = append(m.subscribers[topic], handler)
	return nil
}

func (m *MemoryPubSub) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	m.subscribers = make(map[string][]func(Message))
	return nil
}
