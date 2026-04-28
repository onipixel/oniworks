// Example: OniWorks REST API
// Demonstrates a minimal CRUD API with JWT auth, validation, and middleware.
package main

import (
	"log/slog"
	"os"

	"github.com/onipixel/oniworks/framework/app"
	onihttp "github.com/onipixel/oniworks/framework/http"
	"github.com/onipixel/oniworks/framework/logging"
	"github.com/onipixel/oniworks/framework/middleware"
	"github.com/onipixel/oniworks/framework/routing"
	"github.com/onipixel/oniworks/framework/validation"
)

type CreatePostInput struct {
	Title   string `json:"title"   validate:"required,min:3,max:120"`
	Content string `json:"content" validate:"required,min:10"`
}

func main() {
	logging.New(logging.Config{
		Level:  os.Getenv("LOG_LEVEL"),
		Format: "json",
	})

	oni := app.New()
	oni.Load(".env", "config/app.yaml")

	oni.Use(
		middleware.Logger(),
		middleware.Recovery(),
		middleware.CORS(),
		middleware.Compress(),
	)

	v := validation.New()
	_ = v // used below in validation calls
	// guard := auth.NewGuard(userProvider, sessionMgr, os.Getenv("APP_KEY"))
	// For this example we use a minimal JWT-only setup

	oni.Route(func(r *routing.Router) {
		r.Get("/health", func(c *onihttp.Context) error {
			return c.JSON(200, map[string]any{"status": "ok"})
		})

		r.Group("/api/v1", func(g *routing.Group) {
			// Public auth routes
			g.Post("/auth/login", func(c *onihttp.Context) error {
				var in struct {
					Email    string `json:"email"    validate:"required,email"`
					Password string `json:"password" validate:"required,min:6"`
				}
				if err := c.Bind(&in); err != nil {
					return c.JSON(422, map[string]any{"error": err.Error()})
				}
				if errs := v.Validate(&in); errs != nil {
					return c.JSON(422, map[string]any{"errors": errs})
				}
				// TODO: validate credentials with auth.Guard.Attempt
				// token, _ := guard.IssueToken(user, 24*time.Hour)
				return c.JSON(200, map[string]any{"token": "example-jwt-token"})
			})

			// Protected routes
			g.Group("/posts", func(posts *routing.Group) {
				// posts.Use(middleware.AuthJWT(guard))
				_ = posts

				posts.Get("/", func(c *onihttp.Context) error {
					return c.JSON(200, map[string]any{"data": []any{}})
				})

				posts.Post("/", func(c *onihttp.Context) error {
					var in CreatePostInput
					if err := c.Bind(&in); err != nil {
						return c.JSON(422, map[string]any{"error": err.Error()})
					}
					if errs := v.Validate(&in); errs != nil {
						return c.JSON(422, map[string]any{"errors": errs})
					}
					slog.Info("post created", "title", in.Title)
					return c.JSON(201, map[string]any{"message": "created", "title": in.Title})
				})

				posts.Get("/{id}", func(c *onihttp.Context) error {
					return c.JSON(200, map[string]any{"id": c.Param("id")})
				})
			})
		})
	})

	oni.Serve()
}
