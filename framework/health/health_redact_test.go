package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPublicHandlerRedactsMessages verifies the public health endpoint does not
// leak per-check message detail (e.g. raw DB error strings).
func TestPublicHandlerRedactsMessages(t *testing.T) {
	r := New("1.0")
	r.Register("db", func(context.Context) Result {
		return Result{Status: StatusFail, Message: "dial tcp 10.0.0.5:5432: connection refused"}
	})

	rr := httptest.NewRecorder()
	r.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/health", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "connection refused") || strings.Contains(body, "10.0.0.5") {
		t.Fatalf("public health endpoint leaked internal detail: %s", body)
	}
	// Status itself should still be reported.
	var resp Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Checks["db"].Status != StatusFail {
		t.Fatal("expected db check status to be reported")
	}
}

// TestDetailedHandlerFailsClosed verifies the detailed endpoint denies access
// without an authorizer.
func TestDetailedHandlerFailsClosed(t *testing.T) {
	r := New("1.0")
	rr := httptest.NewRecorder()
	r.DetailedHandler(nil).ServeHTTP(rr, httptest.NewRequest("GET", "/health/detail", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", rr.Code)
	}
}

// TestDetailedHandlerShowsMessages verifies authorized detailed access includes
// the message.
func TestDetailedHandlerShowsMessages(t *testing.T) {
	r := New("1.0")
	r.Register("db", func(context.Context) Result {
		return Result{Status: StatusFail, Message: "boom-detail"}
	})
	rr := httptest.NewRecorder()
	r.DetailedHandler(func(*http.Request) bool { return true }).
		ServeHTTP(rr, httptest.NewRequest("GET", "/health/detail", nil))
	if !strings.Contains(rr.Body.String(), "boom-detail") {
		t.Fatalf("detailed handler should include message, got %s", rr.Body.String())
	}
}
