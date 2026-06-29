package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

// Compress returns gzip compression middleware. It only compresses responses
// when the client advertises "gzip" in Accept-Encoding AND the response is a
// compressible type (text/JSON/XML/JS or unknown) with content — already-encoded
// bodies, images, and empty 204/304 responses are passed through untouched. The
// optional level (gzip.BestSpeed … gzip.BestCompression) is honored.
func Compress(level ...int) onihttp.MiddlewareFunc {
	lvl := gzip.DefaultCompression
	if len(level) > 0 {
		lvl = level[0]
	}
	// One pool per Compress instance so the configured level is actually used.
	pool := &sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, lvl)
			return w
		},
	}

	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				return next(c)
			}
			c.Response.Header().Add("Vary", "Accept-Encoding")

			orig := c.Response.Unwrap()
			gw := &gzipWriter{ResponseWriter: orig, pool: pool}
			c.Response.Wrap(gw)
			defer gw.close()

			return next(c)
		}
	}
}

// gzipWriter decides at first write whether to gzip the body, based on the
// response status and Content-Type, so Content-Encoding is never advertised for
// a body that isn't actually gzipped.
type gzipWriter struct {
	http.ResponseWriter
	pool        *sync.Pool
	gz          *gzip.Writer
	decided     bool
	compress    bool
	wroteHeader bool
}

func (g *gzipWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.decide(code)
	g.wroteHeader = true
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.compress {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// decide chooses whether to compress and, if so, sets the headers and acquires a
// gzip.Writer. Called once, before the status line is written.
func (g *gzipWriter) decide(code int) {
	if g.decided {
		return
	}
	g.decided = true

	h := g.ResponseWriter.Header()
	noBody := code == http.StatusNoContent || code == http.StatusNotModified
	alreadyEncoded := h.Get("Content-Encoding") != ""
	if noBody || alreadyEncoded || !shouldCompress(h.Get("Content-Type")) {
		g.compress = false
		return
	}

	g.compress = true
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length") // length changes after compression
	gz := g.pool.Get().(*gzip.Writer)
	gz.Reset(g.ResponseWriter)
	g.gz = gz
}

// Flush flushes any buffered gzip data, then the underlying writer.
func (g *gzipWriter) Flush() {
	if g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipWriter) close() {
	if g.gz != nil {
		_ = g.gz.Close()
		g.pool.Put(g.gz)
		g.gz = nil
	}
}

func shouldCompress(ct string) bool {
	if ct == "" {
		return true
	}
	for _, prefix := range []string{"text/", "application/json", "application/xml", "application/javascript"} {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}
