package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestRunRecoversPanic verifies a panicking scheduled job does not crash the
// process — run must return normally.
func TestRunRecoversPanic(t *testing.T) {
	s := New()
	j := &Job{name: "panic", timeout: time.Second, fn: func(ctx context.Context) error {
		panic("scheduled boom")
	}}
	s.run(j) // must not panic
}

// TestRunSkipsOverlap verifies that if a job is still running when its next
// tick fires, the overlapping run is skipped.
func TestRunSkipsOverlap(t *testing.T) {
	s := New()
	var calls int32
	release := make(chan struct{})
	j := &Job{name: "overlap", timeout: time.Minute, fn: func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		<-release
		return nil
	}}

	go s.run(j) // first run blocks inside fn
	time.Sleep(30 * time.Millisecond)
	s.run(j) // overlapping run — should be skipped, returns immediately
	close(release)
	time.Sleep(30 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 invocation (overlap skipped), got %d", got)
	}
}

// TestRunExecutesSequentially verifies non-overlapping runs all execute.
func TestRunExecutesSequentially(t *testing.T) {
	s := New()
	var calls int32
	j := &Job{name: "seq", timeout: time.Second, fn: func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}}
	s.run(j)
	s.run(j)
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 invocations, got %d", got)
	}
}
