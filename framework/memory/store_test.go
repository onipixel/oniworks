package memory_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/oniworks/oniworks/framework/memory"
)

func newTestStore(t *testing.T) *memory.Store {
	t.Helper()
	s := memory.New(memory.Options{NodeID: "test-node"})
	t.Cleanup(func() { _ = s.Shutdown() })
	return s
}

func TestSetGet(t *testing.T) {
	s := newTestStore(t)
	s.Set("hello", "world", 0)
	val, ok := s.Get("hello")
	if !ok {
		t.Fatal("Get: key not found")
	}
	if val != "world" {
		t.Errorf("Get: got %v, want %q", val, "world")
	}
}

func TestGetMissing(t *testing.T) {
	s := newTestStore(t)
	_, ok := s.Get("missing")
	if ok {
		t.Error("expected missing key to return false")
	}
}

func TestTTLExpiry(t *testing.T) {
	s := newTestStore(t)
	s.Set("expiring", "value", 30*time.Millisecond)

	// Should be present immediately
	if _, ok := s.Get("expiring"); !ok {
		t.Fatal("key should exist before TTL")
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := s.Get("expiring"); ok {
		t.Error("key should have expired")
	}
}

func TestIncr(t *testing.T) {
	s := newTestStore(t)
	n1 := s.Incr("counter")
	n2 := s.Incr("counter")
	n3 := s.Incr("counter")

	if n1 != 1 {
		t.Errorf("first Incr: got %d, want 1", n1)
	}
	if n2 != 2 {
		t.Errorf("second Incr: got %d, want 2", n2)
	}
	if n3 != 3 {
		t.Errorf("third Incr: got %d, want 3", n3)
	}
}

func TestIncrConcurrent(t *testing.T) {
	s := newTestStore(t)
	var wg atomic.Int32
	const goroutines = 50

	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			s.Incr("concurrent_counter")
			if wg.Add(1) == goroutines {
				close(done)
			}
		}()
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for goroutines")
	}

	val, ok := s.Get("concurrent_counter")
	if !ok {
		t.Fatal("counter key not found")
	}
	n, _ := val.(int64)
	if n != goroutines {
		t.Errorf("concurrent Incr: got %d, want %d", n, goroutines)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	s.Set("temp", "data", 0)
	s.Delete("temp")
	if _, ok := s.Get("temp"); ok {
		t.Error("deleted key should not exist")
	}
}

func TestPubSub(t *testing.T) {
	s := newTestStore(t)
	received := make(chan any, 1)

	cancel := s.Subscribe("test.topic", func(topic string, payload any) {
		received <- payload
	})
	defer cancel()

	s.Publish("test.topic", "hello-event")

	select {
	case got := <-received:
		if got != "hello-event" {
			t.Errorf("got %v, want %q", got, "hello-event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for pub/sub message")
	}
}

func TestPubSubWildcard(t *testing.T) {
	s := newTestStore(t)
	received := make(chan string, 3)

	cancel := s.Subscribe("events.*", func(topic string, payload any) {
		received <- topic
	})
	defer cancel()

	s.Publish("events.created", "a")
	s.Publish("events.updated", "b")
	s.Publish("other.topic", "c") // should NOT match

	got := make([]string, 0)
	timeout := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case t := <-received:
			got = append(got, t)
		case <-timeout:
			break loop
		}
	}

	if len(got) != 2 {
		t.Errorf("expected 2 wildcard matches, got %d: %v", len(got), got)
	}
}

func TestSnapshot(t *testing.T) {
	dir := t.TempDir()
	snapPath := dir + "/oni_test_snap.gob"

	// Write data
	s1 := memory.New(memory.Options{
		NodeID:       "node1",
		Persist:      true,
		SnapshotPath: snapPath,
	})
	s1.Set("persisted", "value123", 0)
	s1.Set("also", "here", 0)
	if err := s1.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Restore data
	s2 := memory.New(memory.Options{
		NodeID:       "node2",
		Persist:      true,
		SnapshotPath: snapPath,
	})
	defer s2.Shutdown()

	val, ok := s2.Get("persisted")
	if !ok {
		t.Fatal("restored key 'persisted' not found")
	}
	if val != "value123" {
		t.Errorf("restored value: got %v, want %q", val, "value123")
	}
}
