package drivers

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/onipixel/oniworks/framework/queue"
)

// Memory is a non-durable in-process queue driver. Suitable for development.
type Memory struct {
	mu     sync.Mutex
	queues map[string][]*queue.Payload
	dead   []*queue.Payload
}

// NewMemory creates an in-memory queue driver.
func NewMemory() *Memory {
	return &Memory{queues: make(map[string][]*queue.Payload)}
}

func (m *Memory) Push(_ context.Context, q string, p *queue.Payload) error {
	m.mu.Lock()
	m.queues[q] = append(m.queues[q], p)
	m.mu.Unlock()
	return nil
}

func (m *Memory) Pop(_ context.Context, q string) (*queue.Payload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.queues[q]
	now := time.Now()
	for i, p := range items {
		if p.AvailableAt.Before(now) || p.AvailableAt.Equal(now) {
			m.queues[q] = append(items[:i], items[i+1:]...)
			return p, nil
		}
	}
	return nil, nil
}

func (m *Memory) Dead(_ context.Context, p *queue.Payload) error {
	m.mu.Lock()
	m.dead = append(m.dead, p)
	m.mu.Unlock()
	return nil
}

func (m *Memory) Release(_ context.Context, p *queue.Payload, delay time.Duration) error {
	p.AvailableAt = time.Now().Add(delay)
	m.mu.Lock()
	m.queues[p.Queue] = append(m.queues[p.Queue], p)
	m.mu.Unlock()
	return nil
}

// DeadLetters returns all dead-lettered payloads (for testing/inspection).
func (m *Memory) DeadLetters() []*queue.Payload {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*queue.Payload, len(m.dead))
	copy(out, m.dead)
	return out
}

// Size returns the number of pending jobs in a queue.
func (m *Memory) Size(q string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queues[q])
}

// ensure json is imported (for future payload inspection helpers)
var _ = json.Marshal
