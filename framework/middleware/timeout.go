package middleware

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"time"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

// Timeout returns a middleware that cancels the request context after d and, if
// the handler has not finished by then, responds 503 Service Unavailable.
//
// It follows http.TimeoutHandler semantics: the handler writes into an
// in-memory buffer rather than straight to the socket. If the handler finishes
// in time, the buffered response is replayed to the real writer; if it times
// out, a 503 is written and the handler's continued writes are discarded. This
// guarantees the handler goroutine and the timeout path never write to the
// underlying ResponseWriter concurrently (the previous implementation raced and
// could corrupt the response). As with the standard library, a handler that
// ignores context cancellation may keep running in the background, and response
// streaming/Hijack are not supported under Timeout.
func Timeout(d time.Duration) onihttp.MiddlewareFunc {
	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			ctx, cancel := context.WithTimeout(c.Ctx(), d)
			defer cancel()

			real := c.Response.Unwrap()
			tw := &timeoutWriter{header: make(http.Header), status: http.StatusOK}

			// The handler goroutine shares c.Response; point it at the buffer so
			// nothing it writes reaches the real socket until we decide to flush.
			c.Response.Wrap(tw)
			tc := c.WithContext(ctx)

			done := make(chan error, 1)
			go func() { done <- next(tc) }()

			select {
			case err := <-done:
				// Handler has fully returned — safe to flush its buffered output.
				tw.mu.Lock()
				timedOut := tw.timedOut
				tw.mu.Unlock()
				if timedOut {
					return err // 503 already sent below in a prior tick — shouldn't happen, but be safe
				}
				c.Response.Wrap(real)
				if tw.wroteHeader {
					dst := real.Header()
					for k, vv := range tw.header {
						dst[k] = vv
					}
					c.Response.WriteHeader(tw.status)
					if tw.buf.Len() > 0 {
						_, _ = c.Response.Write(tw.buf.Bytes())
					}
				}
				return err

			case <-ctx.Done():
				// Timed out. Mark the buffer dead so the still-running handler's
				// writes are dropped, then write 503 straight to the real writer
				// (NOT via the shared c.Response, which the handler still holds).
				tw.mu.Lock()
				tw.timedOut = true
				tw.mu.Unlock()
				real.Header().Set("Content-Type", "application/json; charset=utf-8")
				real.WriteHeader(http.StatusServiceUnavailable)
				_, _ = real.Write([]byte(`{"error":"request timeout"}`))
				return nil
			}
		}
	}
}

// timeoutWriter is an in-memory http.ResponseWriter used by Timeout. All writes
// are buffered and guarded by mu; once timedOut is set, further writes are
// dropped so they can never reach the real socket.
type timeoutWriter struct {
	mu          sync.Mutex
	header      http.Header
	buf         bytes.Buffer
	status      int
	wroteHeader bool
	timedOut    bool
}

func (tw *timeoutWriter) Header() http.Header { return tw.header }

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.status = code
	tw.wroteHeader = true
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if !tw.wroteHeader {
		tw.status = http.StatusOK
		tw.wroteHeader = true
	}
	return tw.buf.Write(b)
}
