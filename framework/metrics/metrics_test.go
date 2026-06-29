package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMetricsHandlerFailsClosed verifies a nil authorizer denies all scrapes.
func TestMetricsHandlerFailsClosed(t *testing.T) {
	rr := httptest.NewRecorder()
	New().Handler(nil).ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403 with nil authorizer", rr.Code)
	}
}

// TestMetricsHandlerRejects verifies a rejecting authorizer blocks the scrape.
func TestMetricsHandlerRejects(t *testing.T) {
	rr := httptest.NewRecorder()
	h := New().Handler(func(*http.Request) bool { return false })
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", rr.Code)
	}
}

// TestMetricsHandlerAllows verifies an accepting authorizer serves metrics.
func TestMetricsHandlerAllows(t *testing.T) {
	rr := httptest.NewRecorder()
	h := New().Handler(func(*http.Request) bool { return true })
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rr.Code)
	}
}

// TestNormalizePath verifies high-cardinality segments are collapsed.
func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"/users/12345":                         "/users/:id",
		"/posts/42/comments/9":                 "/posts/:id/comments/:id",
		"/orders/550e8400-e29b-41d4-a716-446655440000": "/orders/:id",
		"/api/health":                          "/api/health",
		"/":                                    "/",
	}
	for in, want := range cases {
		if got := normalizePath(in); got != want {
			t.Fatalf("normalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}
