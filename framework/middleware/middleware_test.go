package middleware_test

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	onihttp "github.com/oniworks/oniworks/framework/http"
	"github.com/oniworks/oniworks/framework/middleware"
	"github.com/oniworks/oniworks/framework/routing"
)

// helper: build a single-route test server
func serve(t *testing.T, mw onihttp.MiddlewareFunc, handler onihttp.HandlerFunc) http.Handler {
	t.Helper()
	r := routing.New()
	r.Use(mw)
	r.Get("/test", handler)
	r.Post("/test", handler)
	return r
}

// ─── Logger ─────────────────────────────────────────────────────────────

func TestLoggerPassthrough(t *testing.T) {
	h := serve(t, middleware.Logger(), func(c *onihttp.Context) error {
		return c.JSON(200, map[string]any{"ok": true})
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ─── Recovery ───────────────────────────────────────────────────────────

func TestRecoveryCatchesPanic(t *testing.T) {
	h := serve(t, middleware.Recovery(), func(c *onihttp.Context) error {
		panic("test panic")
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic the test itself
	h.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500 after panic, got %d", w.Code)
	}
}

// ─── CORS ────────────────────────────────────────────────────────────────

func TestCORSPreflightHeaders(t *testing.T) {
	cfg := middleware.DefaultCORSConfig
	cfg.AllowOrigins = []string{"https://example.com"}

	h := serve(t, middleware.CORS(cfg), func(c *onihttp.Context) error {
		return c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("expected Access-Control-Allow-Origin header")
	}
}

func TestCORSAllowsAll(t *testing.T) {
	h := serve(t, middleware.CORS(), func(c *onihttp.Context) error {
		return c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://anything.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("expected CORS header to be present")
	}
}

// ─── Compress ────────────────────────────────────────────────────────────

func TestCompressGzip(t *testing.T) {
	bigBody := strings.Repeat("hello OniWorks! ", 200) // >1KB

	h := serve(t, middleware.Compress(), func(c *onihttp.Context) error {
		return c.String(200, "%s", bigBody)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	enc := w.Header().Get("Content-Encoding")
	if enc != "gzip" {
		t.Errorf("expected gzip encoding, got %q", enc)
	}
	// Decompress and verify
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
}

// ─── RateLimit ───────────────────────────────────────────────────────────

func TestRateLimitAllowsUnderLimit(t *testing.T) {
	h := serve(t, middleware.RateLimit(5, time.Minute), func(c *onihttp.Context) error {
		return c.JSON(200, nil)
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "1.2.3.4:1000"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitBlocksOverLimit(t *testing.T) {
	h := serve(t, middleware.RateLimit(3, time.Minute), func(c *onihttp.Context) error {
		return c.JSON(200, nil)
	})

	// First 3 should pass
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
	// 4th should be limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}
