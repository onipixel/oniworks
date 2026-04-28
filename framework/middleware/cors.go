package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	onihttp "github.com/oniworks/oniworks/framework/http"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	// AllowOrigins is a list of origins that are allowed (e.g. "https://example.com").
	// Use ["*"] to allow all origins (not recommended for credentialed requests).
	AllowOrigins []string
	// AllowMethods specifies which HTTP methods are allowed.
	AllowMethods []string
	// AllowHeaders specifies which request headers are allowed.
	AllowHeaders []string
	// ExposeHeaders lists headers the browser is allowed to access.
	ExposeHeaders []string
	// AllowCredentials indicates that the request can include user credentials.
	AllowCredentials bool
	// MaxAge is how long the preflight result can be cached (0 = browser default).
	MaxAge time.Duration
}

// DefaultCORSConfig is a permissive default suitable for development.
var DefaultCORSConfig = CORSConfig{
	AllowOrigins: []string{"*"},
	AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
	AllowHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
	MaxAge:       12 * time.Hour,
}

// CORS returns a middleware that adds Cross-Origin Resource Sharing headers.
func CORS(cfg ...CORSConfig) onihttp.MiddlewareFunc {
	c := DefaultCORSConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if len(c.AllowMethods) == 0 {
		c.AllowMethods = DefaultCORSConfig.AllowMethods
	}

	allowMethods := strings.Join(c.AllowMethods, ", ")
	allowHeaders := strings.Join(c.AllowHeaders, ", ")
	exposeHeaders := strings.Join(c.ExposeHeaders, ", ")

	maxAge := ""
	if c.MaxAge > 0 {
		maxAge = strconv.Itoa(int(c.MaxAge.Seconds()))
	}

	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(ctx *onihttp.Context) error {
			origin := ctx.Request.Header.Get("Origin")
			if origin == "" {
				return next(ctx)
			}

			allowed := isOriginAllowed(origin, c.AllowOrigins)
			h := ctx.Response.Header()

			if allowed {
				h.Set("Access-Control-Allow-Origin", origin)
				if c.AllowCredentials {
					h.Set("Access-Control-Allow-Credentials", "true")
				}
				if exposeHeaders != "" {
					h.Set("Access-Control-Expose-Headers", exposeHeaders)
				}
			}

			// Handle preflight
			if ctx.Method() == http.MethodOptions {
				if allowHeaders != "" {
					h.Set("Access-Control-Allow-Headers", allowHeaders)
				}
				h.Set("Access-Control-Allow-Methods", allowMethods)
				if maxAge != "" {
					h.Set("Access-Control-Max-Age", maxAge)
				}
				ctx.Response.WriteHeader(http.StatusNoContent)
				return nil
			}

			return next(ctx)
		}
	}
}

func isOriginAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
