// Package errors provides custom HTTP error pages and centralized error handling.
package errors

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	onihttp "github.com/oniworks/oniworks/framework/http"
)

// HTTPError is a structured HTTP error that carries a status code and message.
// Handlers return this to trigger a specific HTTP response.
type HTTPError = onihttp.HTTPError

// New creates a new HTTPError — shorthand for onihttp.NewHTTPError.
func New(code int, msg string) *HTTPError { return onihttp.NewHTTPError(code, msg) }

// Handler is the application-wide error handler. It inspects the error and
// sends an appropriate HTTP response (JSON for API requests, HTML otherwise).
type Handler struct {
	debug     bool
	htmlPages map[int]string // status code → HTML template string
}

// NewHandler creates an error Handler.
func NewHandler(debug bool) *Handler {
	h := &Handler{
		debug:     debug,
		htmlPages: make(map[int]string),
	}
	h.htmlPages[http.StatusNotFound] = defaultNotFoundPage
	h.htmlPages[http.StatusInternalServerError] = defaultErrorPage
	return h
}

// SetPage registers a custom HTML template for a specific status code.
func (h *Handler) SetPage(code int, tmpl string) {
	h.htmlPages[code] = tmpl
}

// Handle is the error dispatch function — use this with Router.OnError.
func (h *Handler) Handle(c *onihttp.Context, err error) {
	if err == nil {
		return
	}

	code := http.StatusInternalServerError
	msg := "internal server error"

	if he, ok := err.(*HTTPError); ok {
		code = he.Code
		msg = he.Message
	}

	// JSON for API / AJAX requests
	if c.IsJSON() || c.IsAJAX() || strings.HasPrefix(c.Path(), "/api/") {
		_ = c.JSON(code, onihttp.Map{"error": msg, "code": code})
		return
	}

	// HTML page
	if page, ok := h.htmlPages[code]; ok {
		h.renderPage(c, code, msg, page)
		return
	}

	// Generic HTML fallback
	h.renderPage(c, code, msg, defaultErrorPage)
}

func (h *Handler) renderPage(c *onihttp.Context, code int, msg, tmplStr string) {
	tmpl, err := template.New("error").Parse(tmplStr)
	if err != nil {
		_ = c.String(code, "%s", msg)
		return
	}

	data := map[string]any{
		"Code":    code,
		"Message": msg,
		"Debug":   h.debug,
	}

	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response.WriteHeader(code)
	_ = tmpl.Execute(c.Response, data)
}

// JSON writes a plain JSON error — helper for handlers.
func JSON(c *onihttp.Context, code int, msg string) error {
	return c.JSON(code, onihttp.Map{"error": msg, "code": code})
}

// Abort returns an HTTPError that causes the middleware chain to stop.
func Abort(code int, msg string) error {
	return onihttp.NewHTTPError(code, msg)
}

// MustJSON panics with an HTTPError (use inside handlers to short-circuit).
func MustJSON(c *onihttp.Context, code int, msg string) {
	_ = JSON(c, code, msg)
}

// ValidationError writes a 422 Unprocessable Entity response with field errors.
func ValidationError(c *onihttp.Context, fields map[string][]string) error {
	return c.JSON(http.StatusUnprocessableEntity, onihttp.Map{
		"error":  "validation failed",
		"fields": fields,
	})
}

// FromJSON decodes an error response body.
func FromJSON(body []byte) (code int, msg string) {
	var v struct {
		Code    int    `json:"code"`
		Message string `json:"error"`
	}
	if err := json.Unmarshal(body, &v); err == nil {
		return v.Code, v.Message
	}
	return 500, string(body)
}

const defaultNotFoundPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>404 Not Found — OniWorks</title>
<style>*{box-sizing:border-box}body{font-family:system-ui,sans-serif;background:#0f0f0f;color:#e5e5e5;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}.card{text-align:center;padding:3rem}h1{font-size:6rem;font-weight:800;color:#ff4757;margin:0}p{font-size:1.25rem;color:#aaa}a{color:#ff4757;text-decoration:none}</style>
</head>
<body><div class="card"><h1>404</h1><p>{{.Message}}</p><a href="/">← Back to home</a></div></body>
</html>`

const defaultErrorPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>{{.Code}} Error — OniWorks</title>
<style>*{box-sizing:border-box}body{font-family:system-ui,sans-serif;background:#0f0f0f;color:#e5e5e5;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}.card{text-align:center;padding:3rem}h1{font-size:6rem;font-weight:800;color:#ff4757;margin:0}p{font-size:1.25rem;color:#aaa}</style>
</head>
<body><div class="card"><h1>{{.Code}}</h1><p>{{.Message}}</p>{{if .Debug}}<pre style="text-align:left;color:#aaa;font-size:.875rem;margin-top:2rem"></pre>{{end}}</div></body>
</html>`
