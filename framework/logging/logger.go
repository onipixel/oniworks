// Package logging provides structured logging helpers built on top of log/slog.
// It configures the global logger with JSON or text output, log levels, and
// optional file rotation, and provides a ContextLogger for per-request IDs.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config configures the logger.
type Config struct {
	// Level is the minimum log level: "debug", "info", "warn", "error" (default: "info").
	Level string
	// Format is "json" or "text" (default: "json" in production, "text" in dev).
	Format string
	// Output is the writer to send logs to (default: os.Stderr).
	Output io.Writer
	// AddSource includes caller file:line in each log record.
	AddSource bool
}

// contextKey is unexported to avoid collisions.
type contextKey struct{ name string }

var requestIDKey = contextKey{"request_id"}
var loggerKey = contextKey{"logger"}

// New creates a slog.Logger from config and sets it as the global logger.
func New(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)
	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	var h slog.Handler
	if strings.ToLower(cfg.Format) == "text" {
		h = slog.NewTextHandler(out, opts)
	} else {
		h = slog.NewJSONHandler(out, opts)
	}

	log := slog.New(h)
	slog.SetDefault(log)
	return log
}

// WithRequestID returns a context and logger enriched with the request ID.
func WithRequestID(ctx context.Context, log *slog.Logger, id string) (context.Context, *slog.Logger) {
	enriched := log.With("request_id", id)
	ctx = context.WithValue(ctx, requestIDKey, id)
	ctx = context.WithValue(ctx, loggerKey, enriched)
	return ctx, enriched
}

// FromContext retrieves the logger from context, falling back to slog.Default().
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// RequestID retrieves the request ID from context.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
