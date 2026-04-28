package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Host            string
	Port            int
	TLSCertFile     string
	TLSKeyFile      string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DefaultServerConfig returns production-ready defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:            "",
		Port:            8080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// Kernel is the HTTP server kernel. It wraps the standard library's http.Server
// and adds graceful shutdown, TLS support, and signal handling.
type Kernel struct {
	cfg    ServerConfig
	server *http.Server
	logger *slog.Logger
}

// NewKernel creates a Kernel with the given config and handler.
func NewKernel(cfg ServerConfig, handler http.Handler) *Kernel {
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
	}

	return &Kernel{
		cfg:    cfg,
		server: srv,
		logger: slog.Default(),
	}
}

// WithLogger sets a custom slog.Logger on the kernel.
func (k *Kernel) WithLogger(l *slog.Logger) *Kernel {
	k.logger = l
	return k
}

// Serve starts the HTTP server and blocks until a shutdown signal is received.
// It handles SIGINT and SIGTERM for graceful shutdown.
func (k *Kernel) Serve() error {
	errCh := make(chan error, 1)

	go func() {
		k.logger.Info("oniworks server starting",
			"addr", k.server.Addr,
			"tls", k.cfg.TLSCertFile != "",
		)

		var err error
		if k.cfg.TLSCertFile != "" && k.cfg.TLSKeyFile != "" {
			err = k.server.ListenAndServeTLS(k.cfg.TLSCertFile, k.cfg.TLSKeyFile)
		} else {
			err = k.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for OS signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		k.logger.Info("shutdown signal received", "signal", sig)
	}

	return k.Shutdown()
}

// Shutdown performs a graceful shutdown with the configured timeout.
func (k *Kernel) Shutdown() error {
	timeout := k.cfg.ShutdownTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	k.logger.Info("shutting down server", "timeout", timeout)
	if err := k.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("kernel: graceful shutdown failed: %w", err)
	}
	k.logger.Info("server stopped cleanly")
	return nil
}

// Addr returns the listening address (useful for tests that pick a random port).
func (k *Kernel) Addr() string { return k.server.Addr }

// ListenAndServeBackground starts the server in a goroutine and returns the
// bound address. Useful for integration tests.
func (k *Kernel) ListenAndServeBackground() (string, error) {
	ln, err := net.Listen("tcp", k.server.Addr)
	if err != nil {
		return "", fmt.Errorf("kernel: listen: %w", err)
	}
	addr := ln.Addr().String()
	k.server.Addr = addr

	go func() { _ = k.server.Serve(ln) }()
	return addr, nil
}
