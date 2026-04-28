// Package metrics provides a Prometheus-compatible metrics endpoint and
// built-in framework metrics (HTTP request duration, active connections, etc.).
package metrics

import (
	"net/http"
	"strconv"
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

// Handler returns the Prometheus /metrics HTTP handler.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{
		Registry: r.reg,
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
		path := req.URL.Path

		r.RequestDuration.WithLabelValues(req.Method, path, status).Observe(dur)
		r.RequestsTotal.WithLabelValues(req.Method, path, status).Inc()
	})
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
