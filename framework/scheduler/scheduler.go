// Package scheduler wraps robfig/cron with a fluent API for defining
// scheduled tasks, named jobs, and graceful shutdown.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Job is a named scheduled task.
type Job struct {
	name     string
	schedule string
	fn       func(ctx context.Context) error
	timeout  time.Duration
}

// Scheduler manages cron-based scheduled tasks.
type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	jobs    []*Job
	logger  *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
}

// New creates a Scheduler. Start must be called to begin processing.
func New(logger ...*slog.Logger) *Scheduler {
	log := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		log = logger[0]
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		logger: log,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Every schedules fn using a cron expression (supports seconds: "*/5 * * * * *").
//
//	s.Every("0 * * * * *", "hourly-cleanup", func(ctx context.Context) error { ... })
func (s *Scheduler) Every(expr, name string, fn func(ctx context.Context) error) *Job {
	j := &Job{name: name, schedule: expr, fn: fn, timeout: 5 * time.Minute}
	s.mu.Lock()
	s.jobs = append(s.jobs, j)
	s.mu.Unlock()
	return j
}

// Daily schedules fn to run at a specific time each day (HH:MM, 24h).
//
//	s.Daily("03:00", "nightly-backup", func(ctx context.Context) error { ... })
func (s *Scheduler) Daily(at, name string, fn func(ctx context.Context) error) (*Job, error) {
	var hh, mm int
	if _, err := fmt.Sscanf(at, "%d:%d", &hh, &mm); err != nil {
		return nil, fmt.Errorf("scheduler: invalid time %q (want HH:MM)", at)
	}
	expr := fmt.Sprintf("0 %d %d * * *", mm, hh)
	return s.Every(expr, name, fn), nil
}

// Hourly schedules fn at the top of every hour.
func (s *Scheduler) Hourly(name string, fn func(ctx context.Context) error) *Job {
	return s.Every("0 0 * * * *", name, fn)
}

// WithTimeout sets the per-run timeout for this job.
func (j *Job) WithTimeout(d time.Duration) *Job {
	j.timeout = d
	return j
}

// Start registers all jobs and starts the cron engine.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		job := j // capture
		_, err := s.cron.AddFunc(job.schedule, func() {
			s.run(job)
		})
		if err != nil {
			return fmt.Errorf("scheduler: register %q (%s): %w", job.name, job.schedule, err)
		}
		s.logger.Info("scheduler: registered job", "name", job.name, "schedule", job.schedule)
	}
	s.cron.Start()
	return nil
}

// Stop waits for running jobs to finish and halts the cron engine.
func (s *Scheduler) Stop() context.Context {
	s.cancel()
	return s.cron.Stop()
}

func (s *Scheduler) run(j *Job) {
	ctx, cancel := context.WithTimeout(s.ctx, j.timeout)
	defer cancel()

	start := time.Now()
	s.logger.Debug("scheduler: running job", "name", j.name)

	if err := j.fn(ctx); err != nil {
		s.logger.Error("scheduler: job error", "name", j.name, "duration", time.Since(start), "error", err)
		return
	}
	s.logger.Debug("scheduler: job completed", "name", j.name, "duration", time.Since(start))
}
