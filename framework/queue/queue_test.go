package queue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/onipixel/oniworks/framework/queue"
	"github.com/onipixel/oniworks/framework/queue/drivers"
)

// ─── Test jobs ────────────────────────────────────────────────────

type EchoJob struct {
	Message string `json:"message"`
	Called  bool   `json:"-"`
}

func (j *EchoJob) Handle(ctx context.Context) error {
	j.Called = true
	return nil
}

type FailingJob struct {
	Attempts int `json:"attempts"`
}

func (j *FailingJob) Handle(ctx context.Context) error {
	return errors.New("intentional failure")
}

func init() {
	queue.Register("queue_test.EchoJob", func() queue.Job { return &EchoJob{} })
	queue.Register("queue_test.FailingJob", func() queue.Job { return &FailingJob{} })
}

// ─── Tests ────────────────────────────────────────────────────────

func TestDispatchAndPop(t *testing.T) {
	d := drivers.NewMemory()
	m := queue.NewManager(d, queue.Options{Queues: []string{"default"}})

	job := &EchoJob{Message: "hello"}
	if err := m.Dispatch(context.Background(), "default", job, 0); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	p, err := d.Pop(context.Background(), "default")
	if err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if p == nil {
		t.Fatal("expected a payload, got nil")
	}
	if p.Queue != "default" {
		t.Errorf("queue: got %q", p.Queue)
	}
}

func TestDelayedJobNotReady(t *testing.T) {
	d := drivers.NewMemory()
	m := queue.NewManager(d, queue.Options{Queues: []string{"default"}})

	_ = m.Dispatch(context.Background(), "default", &EchoJob{}, 10*time.Second)

	p, _ := d.Pop(context.Background(), "default")
	if p != nil {
		t.Error("delayed job should not be available yet")
	}
}

func TestDelayedJobBecomesReady(t *testing.T) {
	d := drivers.NewMemory()
	m := queue.NewManager(d, queue.Options{Queues: []string{"default"}})

	_ = m.Dispatch(context.Background(), "default", &EchoJob{}, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	p, err := d.Pop(context.Background(), "default")
	if err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if p == nil {
		t.Fatal("expected job to be available after delay")
	}
}

func TestDeadLetter(t *testing.T) {
	d := drivers.NewMemory()
	_ = queue.NewManager(d, queue.Options{
		Queues:       []string{"default"},
		Workers:      1,
		PollInterval: 10 * time.Millisecond,
	})

	// Dispatch a FailingJob with max 1 attempt so it dead-letters fast
	p := &queue.Payload{
		Class:       "queue_test.FailingJob",
		Data:        []byte(`{}`),
		ID:          "test-fail-1",
		Queue:       "default",
		MaxAttempts: 1,
		Attempts:    0,
		AvailableAt: time.Now(),
	}
	if err := d.Push(context.Background(), "default", p); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Manually process it
	popped, _ := d.Pop(context.Background(), "default")
	if popped == nil {
		t.Fatal("expected job to be poppable")
	}
	popped.Attempts = 1 // simulate one attempt
	_ = d.Dead(context.Background(), popped)

	dead := d.DeadLetters()
	if len(dead) != 1 {
		t.Errorf("expected 1 dead letter, got %d", len(dead))
	}
	if dead[0].ID != "test-fail-1" {
		t.Errorf("dead letter ID: got %q", dead[0].ID)
	}
}

func TestReleaseRequeues(t *testing.T) {
	d := drivers.NewMemory()
	p := &queue.Payload{
		ID:          "requeue-test",
		Queue:       "default",
		AvailableAt: time.Now().Add(-time.Second),
	}
	_ = d.Push(context.Background(), "default", p)
	popped, _ := d.Pop(context.Background(), "default")
	if popped == nil {
		t.Fatal("initial pop failed")
	}

	// Release with 50ms backoff
	_ = d.Release(context.Background(), popped, 50*time.Millisecond)

	// Should not be available immediately
	notReady, _ := d.Pop(context.Background(), "default")
	if notReady != nil {
		t.Error("released job should not be immediately available")
	}

	time.Sleep(100 * time.Millisecond)
	ready, _ := d.Pop(context.Background(), "default")
	if ready == nil {
		t.Error("released job should be available after backoff")
	}
}

func TestQueueSize(t *testing.T) {
	d := drivers.NewMemory()
	mgr := queue.NewManager(d, queue.Options{Queues: []string{"jobs"}})

	for i := 0; i < 5; i++ {
		_ = mgr.Dispatch(context.Background(), "jobs", &EchoJob{Message: "hi"}, 0)
	}
	if got := d.Size("jobs"); got != 5 {
		t.Errorf("expected 5 jobs, got %d", got)
	}
}
