// Package http provides the HTTP kernel, context, request, and response abstractions.
package http

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	"github.com/onipixel/oniworks/framework/validation"
)

// HandlerFunc is an OniWorks HTTP handler. Handlers return an error to signal failure;
// the error is dispatched to the registered error handler automatically.
type HandlerFunc func(*Context) error

// MiddlewareFunc wraps a HandlerFunc, returning a new HandlerFunc.
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// Map is a convenience alias for map[string]any used in JSON/template responses.
type Map = map[string]any

// Context holds the request, response writer, URL params, and a per-request store.
// It is the single argument to every handler and middleware.
type Context struct {
	Request  *Request
	Response *Response

	// handler is the matched route handler (set by router before dispatching).
	handler HandlerFunc

	// store is a per-request key-value bag for passing data between middleware/handlers.
	mu    sync.RWMutex
	store map[string]any

	// ctx is the underlying context.Context (carries deadlines, cancellations, values).
	ctx context.Context
}

// NewContext creates a Context from a raw http.Request and ResponseWriter.
// This is used by the router and should not be called directly in application code.
func NewContext(w http.ResponseWriter, r *http.Request, params map[string]string) *Context {
	return &Context{
		Request:  newRequest(r, params),
		Response: newResponse(w),
		store:    make(map[string]any),
		ctx:      r.Context(),
	}
}

// --- Context propagation ---

// Ctx returns the request's context.Context.
func (c *Context) Ctx() context.Context { return c.ctx }

// WithContext returns a shallow copy of the Context with a new context.Context.
func (c *Context) WithContext(ctx context.Context) *Context {
	clone := *c
	clone.ctx = ctx
	return &clone
}

// --- Per-request store ---

// Set stores a value in the per-request key-value store.
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	c.store[key] = value
	c.mu.Unlock()
}

// Get retrieves a value from the per-request store.
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	v, ok := c.store[key]
	c.mu.RUnlock()
	return v, ok
}

// MustGet retrieves a value and panics if the key is absent.
func (c *Context) MustGet(key string) any {
	v, ok := c.Get(key)
	if !ok {
		panic(fmt.Sprintf("context: key %q not found in store", key))
	}
	return v
}

// --- Response helpers ---

// JSON writes a JSON-encoded body with the given status code.
func (c *Context) JSON(status int, v any) error {
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Response.WriteHeader(status)
	return json.NewEncoder(c.Response).Encode(v)
}

// XML writes an XML-encoded body with the given status code.
func (c *Context) XML(status int, v any) error {
	c.Response.Header().Set("Content-Type", "application/xml; charset=utf-8")
	c.Response.WriteHeader(status)
	_, err := fmt.Fprint(c.Response, xml.Header)
	if err != nil {
		return err
	}
	return xml.NewEncoder(c.Response).Encode(v)
}

// String writes a plain-text body.
func (c *Context) String(status int, format string, args ...any) error {
	c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response.WriteHeader(status)
	_, err := fmt.Fprintf(c.Response, format, args...)
	return err
}

// HTML writes an HTML body.
func (c *Context) HTML(status int, html string) error {
	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response.WriteHeader(status)
	_, err := fmt.Fprint(c.Response, html)
	return err
}

// NoContent sends a 204 No Content response.
func (c *Context) NoContent() error {
	c.Response.WriteHeader(http.StatusNoContent)
	return nil
}

// Redirect sends a redirect response.
func (c *Context) Redirect(status int, url string) error {
	http.Redirect(c.Response, c.Request.Request, url, status)
	return nil
}

// File serves a file from the local filesystem.
func (c *Context) File(path string) error {
	http.ServeFile(c.Response, c.Request.Request, path)
	return nil
}

// --- Request helpers (delegated for ergonomics) ---

// Param returns a URL path parameter by name (e.g. ":id").
func (c *Context) Param(name string) string { return c.Request.Param(name) }

// Query returns a URL query parameter by name.
func (c *Context) Query(name string) string { return c.Request.URL.Query().Get(name) }

// QueryD returns a URL query parameter, or def if absent.
func (c *Context) QueryD(name, def string) string {
	if v := c.Request.URL.Query().Get(name); v != "" {
		return v
	}
	return def
}

// Header returns a request header by name.
func (c *Context) Header(name string) string { return c.Request.Header.Get(name) }

// Bind decodes the request body (JSON, XML, or form) into dest.
func (c *Context) Bind(dest any) error { return c.Request.Bind(dest) }

// Validate binds the request body into dest and then validates it against
// `validate` struct tags. On validation failure it returns a 422 HTTPError
// whose JSON body contains the field-level error map.
//
//	var req struct {
//	    Email    string `json:"email"    validate:"required,email"`
//	    Password string `json:"password" validate:"required,min=8"`
//	}
//	if err := c.Validate(&req); err != nil {
//	    return err // automatic 422 with {"errors":{"email":["..."]}}
//	}
func (c *Context) Validate(dest any) error {
	if err := c.Request.Bind(dest); err != nil {
		return NewHTTPError(http.StatusBadRequest, "invalid request body: "+err.Error())
	}
	if err := validation.Default().Validate(dest); err != nil {
		if verr, ok := err.(validation.Errors); ok {
			_ = c.JSON(http.StatusUnprocessableEntity, Map{
				"message": "validation failed",
				"errors":  verr,
			})
			return err
		}
		return NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}
	return nil
}

// FormValue returns a form field value.
func (c *Context) FormValue(name string) string { return c.Request.FormValue(name) }

// FormFile returns the first file uploaded under the given field name.
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	_, fh, err := c.Request.FormFile(name)
	return fh, err
}

// IsJSON reports whether the request content type is application/json.
func (c *Context) IsJSON() bool {
	return strings.Contains(c.Request.Header.Get("Content-Type"), "application/json")
}

// IsAJAX reports whether the request was made via XMLHttpRequest.
func (c *Context) IsAJAX() bool {
	return c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// IP returns the client's real IP address, respecting X-Forwarded-For if set.
func (c *Context) IP() string { return c.Request.IP() }

// Method returns the HTTP method in uppercase.
func (c *Context) Method() string { return c.Request.Method }

// Path returns the request URL path.
func (c *Context) Path() string { return c.Request.URL.Path }

// --- Error helpers ---

// Abort stops middleware chain propagation with a JSON error response.
func (c *Context) Abort(status int, message string) error {
	return NewHTTPError(status, message)
}

// HTTPError represents an HTTP error with a status code and message.
type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Message)
}

// NewHTTPError creates an HTTPError.
func NewHTTPError(code int, message string) *HTTPError {
	return &HTTPError{Code: code, Message: message}
}
