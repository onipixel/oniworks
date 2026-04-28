package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"

	onihttp "github.com/oniworks/oniworks/framework/http"
)

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(http.ResponseWriter(nil), gzip.DefaultCompression)
		return w
	},
}

// gzipWriter wraps http.ResponseWriter to gzip all written bytes.
type gzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipWriter) Write(b []byte) (int, error) { return g.gz.Write(b) }
func (g *gzipWriter) WriteHeader(code int) {
	g.ResponseWriter.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

// Compress returns gzip compression middleware. Only compresses responses
// for clients that advertise "gzip" in Accept-Encoding.
func Compress(level ...int) onihttp.MiddlewareFunc {
	compressionLevel := gzip.DefaultCompression
	if len(level) > 0 {
		compressionLevel = level[0]
	}
	_ = compressionLevel

	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			ae := c.Request.Header.Get("Accept-Encoding")
			if !strings.Contains(ae, "gzip") {
				return next(c)
			}

			gz := gzipPool.Get().(*gzip.Writer)
			gz.Reset(c.Response.Unwrap())
			defer func() {
				_ = gz.Close()
				gzipPool.Put(gz)
			}()

			gw := &gzipWriter{ResponseWriter: c.Response.Unwrap(), gz: gz}
			c.Response.Wrap(gw)
			c.Response.Header().Set("Content-Encoding", "gzip")
			c.Response.Header().Del("Content-Length")

			return next(c)
		}
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
