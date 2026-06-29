package http

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestWithContextSharesLockAndStore is the regression for the lock-copy data
// race: a Context produced by WithContext must share the SAME mutex pointer and
// the SAME store map as the original, so concurrent access is synchronized by a
// single lock rather than two independent copies.
func TestWithContextSharesLockAndStore(t *testing.T) {
	c := NewContext(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), nil)
	clone := c.WithContext(context.Background())

	if c.mu != clone.mu {
		t.Fatal("clone must share the original's *sync.RWMutex, not a copy")
	}

	// Store is shared: a write through one is visible through the other.
	c.Set("a", 1)
	if v, ok := clone.Get("a"); !ok || v != 1 {
		t.Fatalf("clone should see original's store write, got %v ok=%v", v, ok)
	}
	clone.Set("b", 2)
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Fatalf("original should see clone's store write, got %v ok=%v", v, ok)
	}
}

// TestConcurrentStoreAccess exercises concurrent Set/Get across a Context and
// its WithContext clone. With a shared lock this completes cleanly; under the
// old copied-lock bug it was a data race (best caught with -race, but this also
// guards against deadlocks/panics).
func TestConcurrentStoreAccess(t *testing.T) {
	c := NewContext(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), nil)
	clone := c.WithContext(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) { defer wg.Done(); c.Set("k", n) }(i)
		go func() { defer wg.Done(); _, _ = clone.Get("k") }()
	}
	wg.Wait()
}
