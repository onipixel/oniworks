package scheduler_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onipixel/oniworks/framework/scheduler"
)

func TestEveryFires(t *testing.T) {
	s := scheduler.New()

	var count atomic.Int32
	s.Every("*/1 * * * * *", "every-second", func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	// Wait up to 2.5 seconds for at least one firing
	deadline := time.After(2500 * time.Millisecond)
	for {
		select {
		case <-deadline:
			if count.Load() == 0 {
				t.Error("scheduler did not fire within 2.5 seconds")
			}
			return
		case <-time.After(100 * time.Millisecond):
			if count.Load() > 0 {
				return // fired, test passes
			}
		}
	}
}

func TestStopHaltsFiring(t *testing.T) {
	s := scheduler.New()
	var count atomic.Int32

	s.Every("*/1 * * * * *", "stop-test", func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Let it fire once
	time.Sleep(1200 * time.Millisecond)
	s.Stop()
	snapCount := count.Load()

	// Wait another second and verify count didn't increase
	time.Sleep(1200 * time.Millisecond)
	if count.Load() > snapCount+1 {
		// Allow 1 extra in-flight run, but no more
		t.Errorf("scheduler kept running after Stop: count=%d, snapshot=%d", count.Load(), snapCount)
	}
}

func TestJobTimeout(t *testing.T) {
	s := scheduler.New()
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)

	s.Every("*/1 * * * * *", "timeout-test", func(ctx context.Context) error {
		started <- struct{}{}
		select {
		case <-ctx.Done():
			cancelled <- struct{}{}
		case <-time.After(30 * time.Second):
			t.Error("job was not cancelled by timeout")
		}
		return ctx.Err()
	}).WithTimeout(100 * time.Millisecond)

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("job never started")
	}
	select {
	case <-cancelled:
	case <-time.After(500 * time.Millisecond):
		t.Error("job was not cancelled within timeout")
	}
}

func TestHourly(t *testing.T) {
	s := scheduler.New()
	j := s.Hourly("hourly-noop", func(ctx context.Context) error { return nil })
	if j == nil {
		t.Error("Hourly returned nil job")
	}
}

func TestDaily(t *testing.T) {
	s := scheduler.New()
	j, err := s.Daily("03:00", "daily-noop", func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	if j == nil {
		t.Error("Daily returned nil job")
	}
}

func TestDailyInvalidTime(t *testing.T) {
	s := scheduler.New()
	_, err := s.Daily("not-a-time", "bad", func(ctx context.Context) error { return nil })
	if err == nil {
		t.Error("expected error for invalid time format")
	}
}
