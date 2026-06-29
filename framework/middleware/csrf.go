package middleware

import (
	"crypto/rand"
	"crypto/subtle"
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
	tok, err := generateCSRFToken()
	if err != nil {
		// Fail closed: never store a predictable token. With no stored token the
		// CSRF middleware rejects mutating requests until generation succeeds.
		return ""
	}
	sess.Set(csrfSessionKey, tok)
	return tok
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// secureCompare compares two strings in constant time (prevent timing attacks).
// An empty stored or request token never matches, so a fail-closed (empty)
// token can never validate.
func secureCompare(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
