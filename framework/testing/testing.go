// Package testing provides test helpers for OniWorks applications.
// It spins up an httptest.Server backed by a real OniWorks router,
// offers a fluent request builder, and includes assertion helpers.
package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// App is a test application wrapping httptest.Server.
type App struct {
	t      *testing.T
	server *httptest.Server
	client *http.Client
	header http.Header
}

// NewApp creates a test application with the given handler.
// If handler is nil, a minimal echo handler is used.
func NewApp(t *testing.T, handler ...http.Handler) *App {
	t.Helper()
	var h http.Handler
	if len(handler) > 0 && handler[0] != nil {
		h = handler[0]
	} else {
		h = http.NotFoundHandler()
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &App{
		t:      t,
		server: srv,
		client: &http.Client{Timeout: 10 * time.Second},
		header: make(http.Header),
	}
}

// Close shuts down the test server.
func (a *App) Close() { a.server.Close() }

// URL returns the full URL for a path.
func (a *App) URL(path string) string {
	return a.server.URL + path
}

// WithHeader sets a default header for all requests from this App.
func (a *App) WithHeader(key, value string) *App {
	a.header.Set(key, value)
	return a
}

// WithBearerToken sets the Authorization: Bearer header.
func (a *App) WithBearerToken(token string) *App {
	return a.WithHeader("Authorization", "Bearer "+token)
}

// ─────────────────────────── Request builders ─────────────────────

// GET starts a GET request.
func (a *App) GET(path string) *Request {
	return a.newRequest(http.MethodGet, path, nil)
}

// POST starts a POST request with a JSON body.
func (a *App) POST(path string, body any) *Request {
	return a.newRequest(http.MethodPost, path, body)
}

// PUT starts a PUT request with a JSON body.
func (a *App) PUT(path string, body any) *Request {
	return a.newRequest(http.MethodPut, path, body)
}

// PATCH starts a PATCH request with a JSON body.
func (a *App) PATCH(path string, body any) *Request {
	return a.newRequest(http.MethodPatch, path, body)
}

// DELETE starts a DELETE request.
func (a *App) DELETE(path string) *Request {
	return a.newRequest(http.MethodDelete, path, nil)
}

func (a *App) newRequest(method, path string, body any) *Request {
	return &Request{app: a, method: method, path: path, body: body, headers: a.header.Clone()}
}

// ─────────────────────────── Request ──────────────────────────────

// Request is a fluent HTTP test request.
type Request struct {
	app     *App
	method  string
	path    string
	body    any
	headers http.Header
}

// WithHeader adds a header to this request.
func (r *Request) WithHeader(key, value string) *Request {
	r.headers.Set(key, value)
	return r
}

// Send executes the request and returns the response.
func (r *Request) Send() *Response {
	r.app.t.Helper()

	var bodyReader io.Reader
	contentType := ""

	if r.body != nil {
		switch v := r.body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
			contentType = "text/plain"
		case []byte:
			bodyReader = bytes.NewReader(v)
			contentType = "application/octet-stream"
		case io.Reader:
			bodyReader = v
		default:
			b, err := json.Marshal(r.body)
			if err != nil {
				r.app.t.Fatalf("test: marshal body: %v", err)
			}
			bodyReader = bytes.NewReader(b)
			contentType = "application/json"
		}
	}

	req, err := http.NewRequest(r.method, r.app.URL(r.path), bodyReader)
	if err != nil {
		r.app.t.Fatalf("test: create request: %v", err)
	}

	for k, vs := range r.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := r.app.client.Do(req)
	if err != nil {
		r.app.t.Fatalf("test: do request: %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	return &Response{t: r.app.t, raw: resp, body: body}
}

// ─────────────────────────── Response ─────────────────────────────

// Response wraps an http.Response with assertion helpers.
type Response struct {
	t    *testing.T
	raw  *http.Response
	body []byte
}

// StatusCode returns the HTTP status code.
func (r *Response) StatusCode() int { return r.raw.StatusCode }

// Body returns the raw response body bytes.
func (r *Response) Body() []byte { return r.body }

// BodyString returns the response body as a string.
func (r *Response) BodyString() string { return string(r.body) }

// Header returns a response header value.
func (r *Response) Header(key string) string { return r.raw.Header.Get(key) }

// JSON decodes the body into v.
func (r *Response) JSON(v any) *Response {
	r.t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		r.t.Fatalf("test: decode JSON: %v\nbody: %s", err, r.body)
	}
	return r
}

// AssertStatus fails the test if status != expected.
func (r *Response) AssertStatus(expected int) *Response {
	r.t.Helper()
	if r.raw.StatusCode != expected {
		r.t.Errorf("expected status %d, got %d\nbody: %s", expected, r.raw.StatusCode, r.body)
	}
	return r
}

// AssertOK asserts a 200 response.
func (r *Response) AssertOK() *Response { return r.AssertStatus(http.StatusOK) }

// AssertCreated asserts a 201 response.
func (r *Response) AssertCreated() *Response { return r.AssertStatus(http.StatusCreated) }

// AssertNotFound asserts a 404 response.
func (r *Response) AssertNotFound() *Response { return r.AssertStatus(http.StatusNotFound) }

// AssertUnauthorized asserts a 401 response.
func (r *Response) AssertUnauthorized() *Response {
	return r.AssertStatus(http.StatusUnauthorized)
}

// AssertJSON checks that a JSON key equals the expected value.
//
//	resp.AssertJSON("message", "hello")
func (r *Response) AssertJSON(key string, expected any) *Response {
	r.t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		r.t.Errorf("test: AssertJSON: body is not JSON: %v", err)
		return r
	}
	actual, ok := m[key]
	if !ok {
		r.t.Errorf("test: AssertJSON: key %q not found in response\nbody: %s", key, r.body)
		return r
	}
	if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected) {
		r.t.Errorf("test: AssertJSON[%q]: expected %v, got %v", key, expected, actual)
	}
	return r
}

// AssertContains asserts the body contains the given substring.
func (r *Response) AssertContains(substr string) *Response {
	r.t.Helper()
	if !strings.Contains(string(r.body), substr) {
		r.t.Errorf("test: body does not contain %q\nbody: %s", substr, r.body)
	}
	return r
}

// AssertHeader asserts a response header equals the expected value.
func (r *Response) AssertHeader(key, expected string) *Response {
	r.t.Helper()
	if got := r.raw.Header.Get(key); got != expected {
		r.t.Errorf("test: header %q: expected %q, got %q", key, expected, got)
	}
	return r
}

