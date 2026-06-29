package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

func runTimeout(t *testing.T, d time.Duration, h onihttp.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	c := onihttp.NewContext(rr, httptest.NewRequest("GET", "/", nil), nil)
	wrapped := Timeout(d)(h)
	if err := wrapped(c); err != nil {
		// Fast handlers may return an error that the error handler would render;
		// for these tests handlers write their own responses and return nil.
		t.Logf("handler returned err: %v", err)
	}
	return rr
}

// TestTimeoutFastHandlerPassesThrough verifies a handler that finishes in time
// has its response replayed to the client unchanged.
func TestTimeoutFastHandlerPassesThrough(t *testing.T) {
	rr := runTimeout(t, 200*time.Millisecond, func(c *onihttp.Context) error {
		return c.JSON(http.StatusCreated, onihttp.Map{"ok": true})
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("got status %d, want 201", rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("expected buffered body to be flushed")
	}
}

// TestTimeoutSlowHandlerReturns503 verifies a handler exceeding the deadline
// yields a 503 and the slow handler's eventual write does not corrupt it.
func TestTimeoutSlowHandlerReturns503(t *testing.T) {
	wrote := make(chan struct{})
	rr := runTimeout(t, 30*time.Millisecond, func(c *onihttp.Context) error {
		time.Sleep(120 * time.Millisecond)
		// This write happens AFTER the timeout; it must be discarded.
		_ = c.JSON(http.StatusOK, onihttp.Map{"late": true})
		close(wrote)
		return nil
	})
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want 503", rr.Code)
	}
	// Wait for the slow handler to finish writing into the dead buffer.
	select {
	case <-wrote:
	case <-time.After(time.Second):
		t.Fatal("slow handler never completed")
	}
	if got := rr.Body.String(); got != `{"error":"request timeout"}` {
		t.Fatalf("body was corrupted by the late write: %q", got)
	}
}

// TestTimeoutConcurrentNoPanic runs many timing-out and fast handlers
// concurrently to shake out write races/panics.
func TestTimeoutConcurrentNoPanic(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			d := 20 * time.Millisecond
			runTimeout(t, d, func(c *onihttp.Context) error {
				if n%2 == 0 {
					time.Sleep(60 * time.Millisecond) // times out
				}
				return c.JSON(http.StatusOK, onihttp.Map{"n": n})
			})
		}(i)
	}
	wg.Wait()
}
