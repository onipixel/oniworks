package http

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Request wraps *http.Request with additional helpers.
type Request struct {
	*http.Request
	params map[string]string
}

func newRequest(r *http.Request, params map[string]string) *Request {
	return &Request{Request: r, params: params}
}

// Param returns a path parameter value (e.g. for route "/users/:id", Param("id")).
func (r *Request) Param(name string) string {
	if r.params == nil {
		return ""
	}
	return r.params[name]
}

// Params returns all path parameters as a map.
func (r *Request) Params() map[string]string {
	out := make(map[string]string, len(r.params))
	for k, v := range r.params {
		out[k] = v
	}
	return out
}

// IP returns the real client IP, considering X-Forwarded-For and X-Real-IP headers.
func (r *Request) IP() string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// Bind decodes the request body based on the Content-Type header.
// Supports application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data.
func (r *Request) Bind(dest any) error {
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "application/json"):
		return r.bindJSON(dest)
	case strings.Contains(ct, "application/xml"), strings.Contains(ct, "text/xml"):
		return r.bindXML(dest)
	case strings.Contains(ct, "application/x-www-form-urlencoded"),
		strings.Contains(ct, "multipart/form-data"):
		return r.bindForm(dest)
	default:
		return r.bindJSON(dest)
	}
}

func (r *Request) bindJSON(dest any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20)) // 32 MB limit
	if err != nil {
		return fmt.Errorf("request: reading body: %w", err)
	}
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("request: JSON decode: %w", err)
	}
	return nil
}

func (r *Request) bindXML(dest any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("request: reading body: %w", err)
	}
	if err := xml.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("request: XML decode: %w", err)
	}
	return nil
}

func (r *Request) bindForm(dest any) error {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err2 := r.ParseForm(); err2 != nil {
			return fmt.Errorf("request: parse form: %w", err2)
		}
	}
	return nil
}

// BodyBytes reads and returns the request body as raw bytes (does not consume Body for later reads).
func (r *Request) BodyBytes() ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, 32<<20))
}

// IsSecure reports whether the request was made over HTTPS.
func (r *Request) IsSecure() bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// WantsJSON reports whether the client prefers a JSON response (via Accept header).
func (r *Request) WantsJSON() bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") || strings.Contains(accept, "*/*")
}

// BearerToken extracts the Bearer token from the Authorization header.
func (r *Request) BearerToken() string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
