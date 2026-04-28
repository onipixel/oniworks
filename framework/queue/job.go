package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Job is the interface every queueable job must implement.
type Job interface {
	// Handle executes the job. Returning a non-nil error triggers retries.
	Handle(ctx context.Context) error
}

// Payload wraps a Job for wire transport (JSON encoding).
type Payload struct {
	// Class is the fully-qualified job type name (used by the registry to reconstruct it).
	Class string `json:"class"`
	// Data is the JSON-encoded job struct.
	Data json.RawMessage `json:"data"`
	// ID is a unique job identifier.
	ID string `json:"id"`
	// Attempts is the number of times this job has been attempted.
	Attempts int `json:"attempts"`
	// MaxAttempts is the maximum number of attempts before moving to dead-letter.
	MaxAttempts int `json:"max_attempts"`
	// Queue is the queue name this job was dispatched on.
	Queue string `json:"queue"`
	// AvailableAt is the UTC time after which the job may be picked up.
	AvailableAt time.Time `json:"available_at"`
}

// registry maps class names to factory functions for deserializing jobs.
var registry = map[string]func() Job{}

// Register maps a class name to a factory for that job type.
// Call this once per job type, typically in an init() function.
//
//	queue.Register("SendWelcomeEmail", func() queue.Job { return &SendWelcomeEmail{} })
func Register(class string, factory func() Job) {
	registry[class] = factory
}

// Unmarshal reconstructs a Job from a Payload using the registry.
func (p *Payload) Unmarshal() (Job, error) {
	factory, ok := registry[p.Class]
	if !ok {
		return nil, fmt.Errorf("queue: unknown job class %q", p.Class)
	}
	j := factory()
	if err := json.Unmarshal(p.Data, j); err != nil {
		return nil, fmt.Errorf("queue: unmarshal job %q: %w", p.Class, err)
	}
	return j, nil
}
