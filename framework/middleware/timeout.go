package middleware

import (
	"context"
	"net/http"
	"time"

	onihttp "github.com/oniworks/oniworks/framework/http"
)

// Timeout returns a middleware that cancels the request context after d duration.
// If the handler does not finish within d, the context is cancelled and a 503
// Service Unavailable response is written (unless the response has already been started).
func Timeout(d time.Duration) onihttp.MiddlewareFunc {
	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			ctx, cancel := context.WithTimeout(c.Ctx(), d)
			defer cancel()

			c = c.WithContext(ctx)

			done := make(chan error, 1)
			go func() { done <- next(c) }()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				if !c.Response.Committed() {
					return c.JSON(http.StatusServiceUnavailable, onihttp.Map{
						"error": "request timeout",
					})
				}
				return ctx.Err()
			}
		}
	}
}
