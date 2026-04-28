package http

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

// Response wraps http.ResponseWriter with status tracking and body size counting.
type Response struct {
	http.ResponseWriter
	status    int
	size      int64
	committed bool
}

func newResponse(w http.ResponseWriter) *Response {
	return &Response{ResponseWriter: w, status: http.StatusOK}
}

// WriteHeader captures the status code and delegates to the underlying writer.
// Calling this more than once is a no-op (the header is sent only once).
func (r *Response) WriteHeader(code int) {
	if r.committed {
		return
	}
	r.status = code
	r.ResponseWriter.WriteHeader(code)
	r.committed = true
}

// Write sends bytes to the client.
func (r *Response) Write(b []byte) (int, error) {
	if !r.committed {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += int64(n)
	return n, err
}

// Status returns the HTTP status code sent (or 200 if WriteHeader was never called).
func (r *Response) Status() int { return r.status }

// Size returns the number of bytes written to the response body.
func (r *Response) Size() int64 { return r.size }

// Committed reports whether the response headers have been sent.
func (r *Response) Committed() bool { return r.committed }

// Flush implements http.Flusher if the underlying writer supports it.
func (r *Response) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker if the underlying writer supports it (needed for WebSocket upgrades).
func (r *Response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("response: underlying writer does not implement http.Hijacker")
}

// Unwrap returns the underlying http.ResponseWriter for type-assertion chains.
func (r *Response) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Wrap replaces the underlying http.ResponseWriter. Used by middleware like Compress
// that need to intercept writes. The status/size counters are reset.
func (r *Response) Wrap(w http.ResponseWriter) {
	r.ResponseWriter = w
	r.size = 0
	r.committed = false
	r.status = http.StatusOK
}
