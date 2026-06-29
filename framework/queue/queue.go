// Package queue provides a lightweight job queue with in-memory and Redis drivers.
// Jobs are dispatched, stored in the driver, and processed by workers.
// Dead-letter handling and exponential back-off retries are built in.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"crypto/rand"
	"encoding/hex"
)

// Driver is the backend for storing and retrieving queue payloads.
type Driver interface {
	// Push enqueues a payload on the named queue.
	Push(ctx context.Context, queue string, p *Payload) error
	// Pop removes and returns the next ready payload, or nil if empty/not ready.
	Pop(ctx context.Context, queue string) (*Payload, error)
	// Dead moves a failed payload to the dead-letter queue.
	Dead(ctx context.Context, p *Payload) error
	// Release re-queues a payload with an updated AvailableAt for retry.
	Release(ctx context.Context, p *Payload, delay time.Duration) error
}

// Options configures the Manager.
type Options struct {
	// Queues lists which queues to process and in priority order.
	Queues []string
	// Workers is the number of concurrent goroutines polling for jobs (default: 5).
	Workers int
	// PollInterval is how often workers poll for new jobs (default: 1s).
	PollInterval time.Duration
	// Logger (defaults to slog.Default).
	Logger *slog.Logger
}

// Manager dispatches and processes jobs.
type Manager struct {
	driver  Driver
	opts    Options
	logger  *slog.Logger
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewManager creates a Manager backed by the given driver.
func NewManager(driver Driver, opts Options) *Manager {
	if opts.Workers <= 0 {
		opts.Workers = 5
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = time.Second
	}
	if len(opts.Queues) == 0 {
		opts.Queues = []string{"default"}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		driver: driver,
		opts:   opts,
		logger: opts.Logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Dispatch pushes a job onto the named queue with optional delay.
//
//	m.Dispatch(ctx, "default", &SendEmail{To: "user@example.com"}, 0)
//	m.Dispatch(ctx, "default", &SendEmail{...}, 5*time.Minute) // delayed
func (m *Manager) Dispatch(ctx context.Context, queue string, job Job, delay time.Duration, opts ...DispatchOpts) error {
	class := fmt.Sprintf("%T", job)
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("queue: marshal job: %w", err)
	}

	maxAttempts := 3
	if len(opts) > 0 && opts[0].MaxAttempts > 0 {
		maxAttempts = opts[0].MaxAttempts
	}

	p := &Payload{
		Class:       class,
		Data:        data,
		ID:          newID(),
		Queue:       queue,
		MaxAttempts: maxAttempts,
		AvailableAt: time.Now().Add(delay),
	}
	return m.driver.Push(ctx, queue, p)
}

// DispatchOpts customises job dispatch.
type DispatchOpts struct {
	MaxAttempts int
}

// Work starts the worker goroutines and blocks until Stop is called.
func (m *Manager) Work() {
	for i := 0; i < m.opts.Workers; i++ {
		m.wg.Add(1)
		go m.worker()
	}
	m.wg.Wait()
}

// Stop gracefully drains the workers.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

func (m *Manager) worker() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.processAll()
		}
	}
}

func (m *Manager) processAll() {
	for _, q := range m.opts.Queues {
		for {
			p, err := m.driver.Pop(m.ctx, q)
			if err != nil || p == nil {
				break
			}
			m.process(p)
		}
	}
}

func (m *Manager) process(p *Payload) {
	p.Attempts++
	m.logger.Debug("queue: processing job", "id", p.ID, "class", p.Class, "attempt", p.Attempts)

	job, err := p.Unmarshal()
	if err != nil {
		m.logger.Error("queue: failed to unmarshal job, sending to dead-letter", "id", p.ID, "error", err)
		_ = m.driver.Dead(m.ctx, p)
		return
	}

	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
	defer cancel()

	if err := safeHandle(ctx, job); err != nil {
		m.logger.Warn("queue: job failed", "id", p.ID, "class", p.Class, "attempt", p.Attempts, "error", err)

		if p.Attempts >= p.MaxAttempts {
			m.logger.Error("queue: job exceeded max attempts, dead-lettering", "id", p.ID)
			_ = m.driver.Dead(m.ctx, p)
			return
		}

		// Exponential backoff: 2^attempt seconds
		backoff := time.Duration(1<<uint(p.Attempts)) * time.Second
		_ = m.driver.Release(m.ctx, p, backoff)
		return
	}

	m.logger.Debug("queue: job succeeded", "id", p.ID, "class", p.Class)
}

// safeHandle runs a job's Handle, converting a panic into an error so a
// misbehaving job is retried/dead-lettered like any other failure instead of
// unwinding and killing the worker goroutine (which would drop the job and
// shrink the worker pool).
func safeHandle(ctx context.Context, job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("queue: job panicked: %v", r)
		}
	}()
	return job.Handle(ctx)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
