package queue

import (
	"context"
	"errors"
	"testing"
)

type panicJob struct{}

func (panicJob) Handle(ctx context.Context) error { panic("kaboom") }

type errJob struct{ err error }

func (j errJob) Handle(ctx context.Context) error { return j.err }

type okJob struct{ ran *bool }

func (j okJob) Handle(ctx context.Context) error { *j.ran = true; return nil }

// TestSafeHandleRecoversPanic verifies a panicking job is converted to an error
// instead of unwinding and killing the worker goroutine.
func TestSafeHandleRecoversPanic(t *testing.T) {
	err := safeHandle(context.Background(), panicJob{})
	if err == nil {
		t.Fatal("expected error from panicking job, got nil")
	}
}

// TestSafeHandlePassesError verifies normal job errors propagate unchanged.
func TestSafeHandlePassesError(t *testing.T) {
	sentinel := errors.New("boom")
	if err := safeHandle(context.Background(), errJob{err: sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("got %v, want sentinel error", err)
	}
}

// TestSafeHandleSuccess verifies a successful job returns nil and actually runs.
func TestSafeHandleSuccess(t *testing.T) {
	ran := false
	if err := safeHandle(context.Background(), okJob{ran: &ran}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ran {
		t.Fatal("job did not run")
	}
}
