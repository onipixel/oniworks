package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

const csrfSessionKey = "_csrf_token"
const csrfHeaderName = "X-CSRF-Token"
const csrfFormField = "_token"

// CSRF returns a CSRF protection middleware.
// It validates the token on mutating requests (POST/PUT/PATCH/DELETE).
// GET, HEAD, and OPTIONS always pass through.
// The token must be sent via X-CSRF-Token header (AJAX) or _token form field (HTML forms).
func CSRF() onihttp.MiddlewareFunc {
	safeMethods := map[string]bool{
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
	}

	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			if safeMethods[c.Method()] {
				return next(c)
			}

			sess := CurrentSession(c)
			if sess == nil {
				return onihttp.NewHTTPError(http.StatusForbidden, "CSRF: no session")
			}

			storedRaw, ok := sess.Get(csrfSessionKey)
			if !ok {
				return onihttp.NewHTTPError(http.StatusForbidden, "CSRF: missing token")
			}
			stored := fmt.Sprint(storedRaw)

			requestToken := c.Header(csrfHeaderName)
			if requestToken == "" {
				requestToken = c.FormValue(csrfFormField)
			}

			if !secureCompare(stored, requestToken) {
				return onihttp.NewHTTPError(http.StatusForbidden, "CSRF token mismatch")
			}

			return next(c)
		}
	}
}

// CSRFToken returns the CSRF token for the current session, generating one if needed.
// Embed this in HTML templates:  <input type="hidden" name="_token" value="{{ .CSRFToken }}">
func CSRFToken(c *onihttp.Context) string {
	sess := CurrentSession(c)
	if sess == nil {
		return ""
	}
	if tok, ok := sess.Get(csrfSessionKey); ok {
		return fmt.Sprint(tok)
	}
	tok := generateCSRFToken()
	sess.Set(csrfSessionKey, tok)
	return tok
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// secureCompare compares two strings in constant time (prevent timing attacks).
func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
