package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

func runCompress(t *testing.T, acceptGzip bool, h onihttp.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	if acceptGzip {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	c := onihttp.NewContext(rr, req, nil)
	wrapped := Compress()(h)
	if err := wrapped(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return rr
}

// TestCompressGzipsJSON verifies compressible content IS gzipped and decodes back.
func TestCompressGzipsJSON(t *testing.T) {
	body := strings.Repeat("oniworks ", 200)
	rr := runCompress(t, true, func(c *onihttp.Context) error {
		return c.JSON(200, onihttp.Map{"data": body})
	})
	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", rr.Header().Get("Content-Encoding"))
	}
	gr, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	out, _ := io.ReadAll(gr)
	if !strings.Contains(string(out), "oniworks") {
		t.Fatal("decompressed body missing content")
	}
}

// TestCompressSkipsImages is the double-encode regression: an already-compressed
// content type must NOT be gzipped or labeled Content-Encoding: gzip.
func TestCompressSkipsImages(t *testing.T) {
	rr := runCompress(t, true, func(c *onihttp.Context) error {
		c.Response.Header().Set("Content-Type", "image/png")
		_, _ = c.Response.Write([]byte("\x89PNG fake image bytes"))
		return nil
	})
	if rr.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("image/png must not be gzipped")
	}
	if !strings.Contains(rr.Body.String(), "PNG") {
		t.Fatalf("image body should pass through unchanged, got %q", rr.Body.String())
	}
}

// TestCompressSkipsNoContent verifies a 204 isn't labeled gzip.
func TestCompressSkipsNoContent(t *testing.T) {
	rr := runCompress(t, true, func(c *onihttp.Context) error {
		return c.NoContent()
	})
	if rr.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("204 No Content must not advertise gzip")
	}
}

// TestCompressSkipsWithoutAcceptEncoding verifies no compression when the client
// doesn't accept gzip.
func TestCompressSkipsWithoutAcceptEncoding(t *testing.T) {
	rr := runCompress(t, false, func(c *onihttp.Context) error {
		return c.JSON(200, onihttp.Map{"x": strings.Repeat("y", 500)})
	})
	if rr.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("must not gzip when client does not accept it")
	}
}

// TestCompressLevelHonored verifies the configured level produces a valid
// gzip stream (and doesn't panic) — the level arg was previously a no-op.
func TestCompressLevelHonored(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	c := onihttp.NewContext(rr, req, nil)
	h := Compress(gzip.BestCompression)(func(c *onihttp.Context) error {
		return c.JSON(200, onihttp.Map{"d": strings.Repeat("z", 1000)})
	})
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected gzip with BestCompression")
	}
	if _, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes())); err != nil {
		t.Fatalf("invalid gzip stream at BestCompression: %v", err)
	}
}

var _ http.Flusher = (*gzipWriter)(nil) // gzipWriter must remain a Flusher
