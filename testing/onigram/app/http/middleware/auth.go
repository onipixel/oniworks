package middleware

import (
	"strings"

	"github.com/onipixel/oniworks/framework/auth"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// Auth returns a middleware that validates a JWT Bearer token.
// On success it stores user_id (int64) in the request context under "user_id".
func Auth(guard *auth.Guard) onihttp.MiddlewareFunc {
	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			token := c.Request.BearerToken()
			if token == "" {
				// Also accept ?token= query param (WebSocket handshake)
				token = c.Query("token")
			}
			if token == "" {
				return c.Abort(401, "unauthenticated")
			}
			claims, err := guard.ParseToken(token)
			if err != nil {
				return c.Abort(401, "invalid or expired token")
			}
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			return next(c)
		}
	}
}

// OptionalAuth reads the JWT if present but does not reject anonymous requests.
func OptionalAuth(guard *auth.Guard) onihttp.MiddlewareFunc {
	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			token := c.Request.BearerToken()
			if token == "" {
				token = c.Query("token")
			}
			if token != "" {
				if claims, err := guard.ParseToken(token); err == nil {
					c.Set("user_id", claims.UserID)
					c.Set("user_email", claims.Email)
				}
			}
			return next(c)
		}
	}
}

// userID extracts the authenticated user ID from the context store.
// Returns 0 if not authenticated.
func userID(c *onihttp.Context) int64 {
	v, ok := c.Get("user_id")
	if !ok {
		return 0
	}
	id, _ := v.(int64)
	return id
}

// stripBearer is a helper for websocket token extraction.
func stripBearer(h string) string {
	return strings.TrimPrefix(h, "Bearer ")
}

// Expose userID so controllers can call middleware.UserID(c).
var UserID = userID
var StripBearer = stripBearer
