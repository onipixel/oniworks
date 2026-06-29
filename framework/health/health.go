// Package health provides an HTTP health-check endpoint and a registry
// for application-defined checks (database, queue, external services, etc.).
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the result of a health check.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check is a named health check function.
type Check struct {
	Name string
	Fn   func(ctx context.Context) Result
}

// Result is the outcome of a single health check.
type Result struct {
	Status  Status        `json:"status"`
	Message string        `json:"message,omitempty"`
	Latency time.Duration `json:"latency_ms"`
}

// Response is the full health report.
type Response struct {
	Status  Status            `json:"status"`
	Checks  map[string]Result `json:"checks"`
	Version string            `json:"version,omitempty"`
}

// Registry holds all registered health checks.
type Registry struct {
	mu      sync.RWMutex
	checks  []Check
	version string
}

// New creates a Registry.
func New(version string) *Registry {
	return &Registry{version: version}
}

// Register adds a health check.
func (r *Registry) Register(name string, fn func(ctx context.Context) Result) {
	r.mu.Lock()
	r.checks = append(r.checks, Check{Name: name, Fn: fn})
	r.mu.Unlock()
}

// Run executes all checks concurrently and returns the aggregated response.
func (r *Registry) Run(ctx context.Context) Response {
	r.mu.RLock()
	checks := make([]Check, len(r.checks))
	copy(checks, r.checks)
	r.mu.RUnlock()

	results := make(map[string]Result, len(checks))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, c := range checks {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			res := c.Fn(ctx)
			res.Latency = time.Since(start)
			mu.Lock()
			results[c.Name] = res
			mu.Unlock()
		}()
	}
	wg.Wait()

	overall := StatusPass
	for _, res := range results {
		if res.Status == StatusFail {
			overall = StatusFail
			break
		}
		if res.Status == StatusWarn && overall != StatusFail {
			overall = StatusWarn
		}
	}

	return Response{Status: overall, Checks: results, Version: r.version}
}

// Handler returns a PUBLIC health endpoint: it reports overall status and each
// check's pass/fail, but strips per-check Message text (which may contain
// database errors, hostnames, or other internal detail). Responds 200 if all
// checks pass, 503 otherwise. For full detail use DetailedHandler behind auth.
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
		defer cancel()

		resp := r.Run(ctx)
		// Redact detail so the public endpoint never leaks internals.
		for name, res := range resp.Checks {
			res.Message = ""
			resp.Checks[name] = res
		}

		writeHealth(w, resp)
	}
}

// DetailedHandler returns a health endpoint that includes per-check Message
// detail (errors, latencies). Because that detail is sensitive, the handler is
// guarded by authorize — passing nil fails closed (403).
func (r *Registry) DetailedHandler(authorize func(*http.Request) bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if authorize == nil || !authorize(req) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
		defer cancel()
		writeHealth(w, r.Run(ctx))
	}
}

func writeHealth(w http.ResponseWriter, resp Response) {
	statusCode := http.StatusOK
	if resp.Status == StatusFail {
		statusCode = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

// ─────────────────────────── Built-in checks ──────────────────────

// Ping returns a simple always-passing check for liveness probes.
func Ping() Check {
	return Check{
		Name: "ping",
		Fn: func(_ context.Context) Result {
			return Result{Status: StatusPass, Message: "pong"}
		},
	}
}

// DatabaseCheck wraps a db.Pinger in a health check.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Database returns a health check that pings the database.
func Database(name string, db Pinger) Check {
	return Check{
		Name: name,
		Fn: func(ctx context.Context) Result {
			if err := db.Ping(ctx); err != nil {
				return Result{Status: StatusFail, Message: err.Error()}
			}
			return Result{Status: StatusPass}
		},
	}
}
