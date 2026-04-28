package middleware

import (
	"net/http"

	"github.com/oniworks/oniworks/framework/auth"
	onihttp "github.com/oniworks/oniworks/framework/http"
	"github.com/oniworks/oniworks/framework/session"
)

const ctxKeyUser = "_oni_user"
const ctxKeySession = "_oni_session"

// Auth returns a session-based authentication middleware.
// Unauthenticated requests receive a 401 JSON response (or redirect for HTML clients).
func Auth(guard *auth.Guard, sessions *session.Manager) onihttp.MiddlewareFunc {
	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			sess, err := sessions.Start(c.Ctx(), c.Request.Request, c.Response)
			if err != nil {
				return onihttp.NewHTTPError(http.StatusInternalServerError, "session error")
			}
			c.Set(ctxKeySession, sess)

			user, err := guard.UserFromSession(c.Ctx(), sess)
			if err != nil {
				return onihttp.NewHTTPError(http.StatusInternalServerError, "auth error")
			}
			if user == nil {
				if c.IsJSON() || c.IsAJAX() {
					return c.JSON(http.StatusUnauthorized, onihttp.Map{"error": "unauthenticated"})
				}
				return c.Redirect(http.StatusFound, "/login")
			}

			c.Set(ctxKeyUser, user)
			return next(c)
		}
	}
}

// AuthJWT returns a JWT bearer token authentication middleware.
// The token must be provided as "Authorization: Bearer <token>".
func AuthJWT(guard *auth.Guard) onihttp.MiddlewareFunc {
	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			token := c.Request.BearerToken()
			if token == "" {
				return c.JSON(http.StatusUnauthorized, onihttp.Map{"error": "missing bearer token"})
			}
			user, err := guard.UserFromToken(c.Ctx(), token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, onihttp.Map{"error": "invalid or expired token"})
			}
			if user == nil {
				return c.JSON(http.StatusUnauthorized, onihttp.Map{"error": "unauthenticated"})
			}
			c.Set(ctxKeyUser, user)
			return next(c)
		}
	}
}

// SessionMiddleware loads the session for every request (without enforcing auth).
// Use this globally; use Auth/AuthJWT on protected routes only.
func SessionMiddleware(sessions *session.Manager) onihttp.MiddlewareFunc {
	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			sess, err := sessions.Start(c.Ctx(), c.Request.Request, c.Response)
			if err != nil {
				return next(c) // continue even if session fails
			}
			c.Set(ctxKeySession, sess)
			err = next(c)
			_ = sessions.Save(c.Ctx(), c.Response, sess)
			return err
		}
	}
}

// CurrentUser retrieves the authenticated user from the context store.
// Returns nil if not authenticated.
func CurrentUser(c *onihttp.Context) auth.User {
	u, _ := c.Get(ctxKeyUser)
	if u == nil {
		return nil
	}
	user, _ := u.(auth.User)
	return user
}

// CurrentSession retrieves the active session from the context store.
func CurrentSession(c *onihttp.Context) *session.Session {
	s, _ := c.Get(ctxKeySession)
	if s == nil {
		return nil
	}
	sess, _ := s.(*session.Session)
	return sess
}
