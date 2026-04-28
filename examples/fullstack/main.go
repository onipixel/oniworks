// Example: OniWorks Fullstack App
// Demonstrates HTTP + WebSocket + Vite frontend + Oni Memory + Queue + Mail.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/onipixel/oniworks/framework/app"
	onihttp "github.com/onipixel/oniworks/framework/http"
	"github.com/onipixel/oniworks/framework/mail"
	"github.com/onipixel/oniworks/framework/memory"
	"github.com/onipixel/oniworks/framework/middleware"
	"github.com/onipixel/oniworks/framework/queue"
	queuedrivers "github.com/onipixel/oniworks/framework/queue/drivers"
	"github.com/onipixel/oniworks/framework/realtime"
	"github.com/onipixel/oniworks/framework/routing"
	"github.com/onipixel/oniworks/framework/scheduler"
	"github.com/onipixel/oniworks/framework/session"
	sessiondrv "github.com/onipixel/oniworks/framework/session/drivers"
)

// WelcomeEmailJob sends a welcome email asynchronously.
type WelcomeEmailJob struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}

func (j *WelcomeEmailJob) Handle(ctx context.Context) error {
	slog.Info("sending welcome email", "to", j.Email, "user_id", j.UserID)
	// mailer.Send(...)
	return nil
}

func main() {
	oni := app.New()
	oni.Load(".env", "config/app.yaml")

	// ── Oni Memory ──────────────────────────────────────────────────
	mem := memory.New(memory.Options{
		GracefulSave: true,
		SnapshotPath: "./storage/memory.snap",
	})

	// ── Sessions (backed by Oni Memory) ─────────────────────────────
	sessions := session.NewManager(sessiondrv.NewMemoryStore(), session.Config{
		CookieName: "oni_session",
		TTL:        24 * time.Hour,
	})

	// ── Queue ────────────────────────────────────────────────────────
	queue.Register("WelcomeEmailJob", func() queue.Job { return &WelcomeEmailJob{} })
	qdriver := queuedrivers.NewMemory()
	qmgr := queue.NewManager(qdriver, queue.Options{
		Queues:  []string{"default", "mail"},
		Workers: 4,
	})
	go qmgr.Work()
	defer qmgr.Stop()

	// ── Mailer ───────────────────────────────────────────────────────
	mailer := mail.New(mail.Config{
		Host:     os.Getenv("MAIL_HOST"),
		Port:     587,
		Username: os.Getenv("MAIL_USERNAME"),
		Password: os.Getenv("MAIL_PASSWORD"),
		FromAddr: "noreply@example.com",
		FromName: "OniWorks",
	})
	_ = mailer.LoadTemplates("resources/views/emails")

	// ── Scheduler ────────────────────────────────────────────────────
	sched := scheduler.New()
	sched.Daily("03:00", "nightly-cleanup", func(ctx context.Context) error {
		slog.Info("running nightly cleanup")
		return nil
	})
	_ = sched.Start()
	defer sched.Stop()

	// ── Realtime ─────────────────────────────────────────────────────
	hub := realtime.New(realtime.Options{Memory: mem})
	hub.Channel("notifications.{userID}", func(c *realtime.Conn, e *realtime.Event) error {
		return nil
	})

	// ── Middleware ────────────────────────────────────────────────────
	oni.Use(
		middleware.Logger(),
		middleware.Recovery(),
		middleware.CORS(),
		middleware.Compress(),
		middleware.SessionMiddleware(sessions),
	)

	// ── Routes ───────────────────────────────────────────────────────
	oni.Route(func(r *routing.Router) {
		r.Get("/ws", func(c *onihttp.Context) error {
			hub.ServeHTTP(c.Response, c.Request.Request)
			return nil
		})

		r.Get("/", func(c *onihttp.Context) error {
			return c.JSON(200, map[string]any{
				"app":  "OniWorks Fullstack",
				"docs": "https://oniworks.dev/docs",
			})
		})

		r.Group("/api/v1", func(api *routing.Group) {
			api.Post("/register", func(c *onihttp.Context) error {
				var in struct {
					Email    string `json:"email"`
					Password string `json:"password"`
				}
				if err := c.Bind(&in); err != nil {
					return c.JSON(422, map[string]any{"error": err.Error()})
				}

				// Dispatch welcome email job
				_ = qmgr.Dispatch(context.Background(), "mail", &WelcomeEmailJob{
					UserID: 1,
					Email:  in.Email,
				}, 0)

				// Notify via realtime
				_ = hub.Broadcast("notifications.1", map[string]any{
					"type":    "welcome",
					"message": "Account created",
				})

				return c.JSON(201, map[string]any{"message": "registered"})
			})
		})
	})

	oni.Serve()
}
