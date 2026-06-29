// Package metrics provides a Prometheus-compatible metrics endpoint and
// built-in framework metrics (HTTP request duration, active connections, etc.).
package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry wraps a Prometheus registry with framework-specific metrics.
type Registry struct {
	reg *prometheus.Registry

	// HTTP metrics
	RequestDuration *prometheus.HistogramVec
	RequestsTotal   *prometheus.CounterVec
	ActiveRequests  prometheus.Gauge

	// Queue metrics
	JobsProcessed *prometheus.CounterVec
	JobsFailed    *prometheus.CounterVec

	// WebSocket metrics
	WebSocketConns prometheus.Gauge
}

// New creates a Registry with built-in framework metrics registered.
func New() *Registry {
	reg := prometheus.NewRegistry()

	// Standard Go runtime + process metrics
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	r := &Registry{reg: reg}

	r.RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "oni",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latencies in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	r.RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "oni",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	r.ActiveRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "oni",
		Subsystem: "http",
		Name:      "active_requests",
		Help:      "Number of HTTP requests currently being handled.",
	})

	r.JobsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "oni",
		Subsystem: "queue",
		Name:      "jobs_processed_total",
		Help:      "Total number of queue jobs processed.",
	}, []string{"queue", "class"})

	r.JobsFailed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "oni",
		Subsystem: "queue",
		Name:      "jobs_failed_total",
		Help:      "Total number of queue jobs that failed.",
	}, []string{"queue", "class"})

	r.WebSocketConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "oni",
		Subsystem: "realtime",
		Name:      "websocket_connections",
		Help:      "Number of active WebSocket connections.",
	})

	reg.MustRegister(
		r.RequestDuration,
		r.RequestsTotal,
		r.ActiveRequests,
		r.JobsProcessed,
		r.JobsFailed,
		r.WebSocketConns,
	)

	return r
}

// Handler returns the Prometheus /metrics HTTP handler, guarded by authorize.
//
// /metrics discloses internal route names, traffic volumes, and process detail,
// so it must not be exposed unauthenticated. authorize runs on every scrape;
// return true to allow. Passing nil fails closed (every request gets 403) — to
// intentionally expose metrics without app-level auth (e.g. when the endpoint
// is isolated at the network layer), pass a function that always returns true.
func (r *Registry) Handler(authorize func(*http.Request) bool) http.Handler {
	prom := promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{Registry: r.reg})
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if authorize == nil || !authorize(req) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		prom.ServeHTTP(w, req)
	})
}

// Middleware returns an HTTP middleware that records request duration and count.
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		r.ActiveRequests.Inc()
		defer r.ActiveRequests.Dec()

		rw := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, req)

		dur := time.Since(start).Seconds()
		status := strconv.Itoa(rw.status)
		path := normalizePath(req.URL.Path)

		r.RequestDuration.WithLabelValues(req.Method, path, status).Observe(dur)
		r.RequestsTotal.WithLabelValues(req.Method, path, status).Inc()
	})
}

// normalizePath collapses high-cardinality path segments (numeric IDs, UUIDs,
// long hex/tokens) into ":id" so an attacker cannot explode Prometheus label
// cardinality — and memory — by hitting many unique paths. This is a heuristic
// stand-in for the matched route pattern.
func normalizePath(p string) string {
	if len(p) > 512 {
		p = p[:512]
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if isVariableSegment(s) {
			segs[i] = ":id"
		}
	}
	return strings.Join(segs, "/")
}

func isVariableSegment(s string) bool {
	if s == "" {
		return false
	}
	digits, hexish := 0, 0
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			digits++
			hexish++
		case (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-':
			hexish++
		}
	}
	// All-numeric segment (an ID), or a long hex/UUID-like token.
	if digits == len(s) {
		return true
	}
	if len(s) >= 16 && hexish == len(s) {
		return true
	}
	return false
}

// MustRegister registers additional Prometheus collectors.
func (r *Registry) MustRegister(cs ...prometheus.Collector) {
	r.reg.MustRegister(cs...)
}

// ─────────────────────────── Response recorder ───────────────────

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}
