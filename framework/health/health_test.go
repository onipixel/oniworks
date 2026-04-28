package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onipixel/oniworks/framework/health"
)

func TestAllPass(t *testing.T) {
	r := health.New("1.0.0")
	r.Register("db", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusPass}
	})
	r.Register("cache", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusPass}
	})

	resp := r.Run(context.Background())
	if resp.Status != health.StatusPass {
		t.Errorf("overall: got %q, want %q", resp.Status, health.StatusPass)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("version: got %q", resp.Version)
	}
	if len(resp.Checks) != 2 {
		t.Errorf("checks count: got %d", len(resp.Checks))
	}
}

func TestOneFails(t *testing.T) {
	r := health.New("1.0.0")
	r.Register("passing", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusPass}
	})
	r.Register("failing", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusFail, Message: "connection refused"}
	})

	resp := r.Run(context.Background())
	if resp.Status != health.StatusFail {
		t.Errorf("overall: got %q, want %q", resp.Status, health.StatusFail)
	}
}

func TestWarning(t *testing.T) {
	r := health.New("1.0.0")
	r.Register("warn-check", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusWarn, Message: "high latency"}
	})

	resp := r.Run(context.Background())
	if resp.Status != health.StatusWarn {
		t.Errorf("overall: got %q, want %q", resp.Status, health.StatusWarn)
	}
}

func TestFailOverridesWarn(t *testing.T) {
	r := health.New("1.0.0")
	r.Register("warn", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusWarn}
	})
	r.Register("fail", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusFail}
	})

	resp := r.Run(context.Background())
	if resp.Status != health.StatusFail {
		t.Errorf("fail should override warn, got %q", resp.Status)
	}
}

func TestHTTPHandler200(t *testing.T) {
	r := health.New("2.0.0")
	r.Register("ping", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusPass}
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}
}

func TestHTTPHandler503(t *testing.T) {
	r := health.New("2.0.0")
	r.Register("db", func(ctx context.Context) health.Result {
		return health.Result{Status: health.StatusFail, Message: "down"}
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

// ─── Mock Pinger ──────────────────────────────────────────────────

type mockPinger struct{ err error }

func (m *mockPinger) Ping(ctx context.Context) error { return m.err }

func TestDatabaseCheckPass(t *testing.T) {
	check := health.Database("db", &mockPinger{err: nil})
	result := check.Fn(context.Background())
	if result.Status != health.StatusPass {
		t.Errorf("expected pass, got %q: %s", result.Status, result.Message)
	}
}

func TestDatabaseCheckFail(t *testing.T) {
	check := health.Database("db", &mockPinger{err: errors.New("connection refused")})
	result := check.Fn(context.Background())
	if result.Status != health.StatusFail {
		t.Errorf("expected fail, got %q", result.Status)
	}
	if result.Message == "" {
		t.Error("expected error message")
	}
}

func TestPingCheck(t *testing.T) {
	check := health.Ping()
	result := check.Fn(context.Background())
	if result.Status != health.StatusPass {
		t.Errorf("ping: got %q", result.Status)
	}
}
