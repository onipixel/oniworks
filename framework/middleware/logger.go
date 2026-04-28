// Package middleware provides built-in OniWorks middleware.
package middleware

import (
	"log/slog"
	"time"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

// LoggerConfig configures the Logger middleware.
type LoggerConfig struct {
	// Logger is the slog.Logger to write to. Defaults to slog.Default().
	Logger *slog.Logger
	// SkipPaths is a list of request paths to skip logging for (e.g. "/health").
	SkipPaths []string
}

// Logger returns a middleware that logs every request using structured slog output.
// It records method, path, status, latency, bytes written, and client IP.
func Logger(opts ...LoggerConfig) onihttp.MiddlewareFunc {
	cfg := LoggerConfig{Logger: slog.Default()}
	if len(opts) > 0 {
		cfg = opts[0]
		if cfg.Logger == nil {
			cfg.Logger = slog.Default()
		}
	}

	skip := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = true
	}

	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			if skip[c.Path()] {
				return next(c)
			}

			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			level := slog.LevelInfo
			if c.Response.Status() >= 500 {
				level = slog.LevelError
			} else if c.Response.Status() >= 400 {
				level = slog.LevelWarn
			}

			cfg.Logger.Log(c.Ctx(), level, "request",
				"method", c.Method(),
				"path", c.Path(),
				"status", c.Response.Status(),
				"latency", latency,
				"bytes", c.Response.Size(),
				"ip", c.IP(),
			)

			return err
		}
	}
}
