package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

// RecoveryConfig configures the Recovery middleware.
type RecoveryConfig struct {
	// Logger is used to log recovered panics. Defaults to slog.Default().
	Logger *slog.Logger
	// Handler is called after recovery; if nil the default 500 JSON response is sent.
	Handler func(c *onihttp.Context, recovered any, stack []byte) error
}

// Recovery returns a middleware that recovers from panics and returns a 500 response.
// The panic value and stack trace are logged via slog.
func Recovery(opts ...RecoveryConfig) onihttp.MiddlewareFunc {
	cfg := RecoveryConfig{Logger: slog.Default()}
	if len(opts) > 0 {
		cfg = opts[0]
		if cfg.Logger == nil {
			cfg.Logger = slog.Default()
		}
	}

	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					cfg.Logger.Error("panic recovered",
						"error", fmt.Sprintf("%v", r),
						"stack", string(stack),
						"method", c.Method(),
						"path", c.Path(),
					)

					if cfg.Handler != nil {
						err = cfg.Handler(c, r, stack)
						return
					}

					if !c.Response.Committed() {
						err = c.JSON(http.StatusInternalServerError, onihttp.Map{
							"error": "internal server error",
						})
					}
				}
			}()
			return next(c)
		}
	}
}
