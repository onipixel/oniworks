package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	// Side-effect import: registers all migrations via init()
	_ "onigram/database/migrations"

	"onigram/app/channels"
	ctrl "onigram/app/http/controllers"
	mw "onigram/app/http/middleware"
	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/app"
	"github.com/onipixel/oniworks/framework/auth"
	"github.com/onipixel/oniworks/framework/config"
	"github.com/onipixel/oniworks/framework/database"
	"github.com/onipixel/oniworks/framework/frontend"
	onihttp "github.com/onipixel/oniworks/framework/http"
	"github.com/onipixel/oniworks/framework/middleware"
	"github.com/onipixel/oniworks/framework/migrations"
	"github.com/onipixel/oniworks/framework/realtime"
	"github.com/onipixel/oniworks/framework/routing"
)

var (
	hub   *realtime.Hub
	guard *auth.Guard
)

func main() {
	// Dispatch oni CLI commands forwarded via --oni-cmd=<cmd>
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--oni-cmd=") {
			cmd := strings.TrimPrefix(arg, "--oni-cmd=")
			if err := handleOniCmd(cmd); err != nil {
				fmt.Fprintln(os.Stderr, "oni error:", err)
				os.Exit(1)
			}
			return
		}
	}

	oni := app.New()
	oni.Load(".env", "config/app.yaml")

	// Open database
	dbPort, _ := strconv.Atoi(oniEnv("DB_PORT", "5432"))
	db, err := database.Open(database.Config{
		Driver:   database.DriverPostgres,
		Host:     oniEnv("DB_HOST", "127.0.0.1"),
		Port:     dbPort,
		Name:     oniEnv("DB_NAME", "onigram"),
		User:     oniEnv("DB_USER", "postgres"),
		Password: oniEnv("DB_PASSWORD", "password"),
		SSLMode:  "disable",
		MaxOpen:  25,
		MaxIdle:  5,
	})
	if err != nil {
		oni.Logger.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	database.SetDefault(db)

	// Set up JWT guard (provider=nil since we only need IssueToken/ParseToken)
	jwtSecret := oniEnv("APP_KEY", "change-me-in-production")
	guard = auth.NewGuard(nil, nil, jwtSecret)

	// Set up realtime hub with JWT auth
	hub = realtime.New(realtime.Options{
		AuthFunc: func(r *http.Request) (int64, error) {
			token := r.Header.Get("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			if token == "" {
				return 0, nil // anonymous WebSocket allowed; channel auth enforced per-channel
			}
			claims, err := guard.ParseToken(token)
			if err != nil {
				return 0, err
			}
			return claims.UserID, nil
		},
	})
	channels.RegisterNotifyChannel(hub)
	channels.RegisterDMChannel(hub)

	// Build notify callback used by controllers
	notifyFn := func(notif *models.Notification) {
		channel := fmt.Sprintf("notify.%d", notif.UserID)
		_ = hub.Broadcast(channel, notif)
	}

	// DM notify callback
	dmNotifyFn := func(userID int64, event string, payload any) {
		channel := fmt.Sprintf("dm.%d", userID)
		_ = hub.Broadcast(channel, map[string]any{"event": event, "data": payload})
	}

	// Set up frontend asset manager
	appMode := oniEnv("APP_ENV", "local")
	feMode := "dev"
	if appMode == "production" {
		feMode = "production"
	}
	fe := frontend.New(feMode)
	if feMode == "production" {
		_ = fe.LoadManifest("public/build/.vite/manifest.json")
	}

	// Middleware stack
	authMW := mw.Auth(guard)
	optionalAuthMW := mw.OptionalAuth(guard)

	// Controllers
	authCtrl := &ctrl.AuthController{Guard: guard}
	userCtrl := &ctrl.UserController{}
	postCtrl := &ctrl.PostController{}
	likeCtrl := &ctrl.LikeController{NotifyFn: notifyFn}
	commentCtrl := &ctrl.CommentController{NotifyFn: notifyFn}
	notifCtrl := &ctrl.NotificationController{}
	storyCtrl := &ctrl.StoryController{}
	bookmarkCtrl := &ctrl.BookmarkController{}
	msgCtrl := &ctrl.MessageController{NotifyFn: dmNotifyFn}

	oni.Use(
		middleware.Logger(),
		middleware.Recovery(),
		middleware.CORS(),
	)

	oni.Route(func(r *routing.Router) {
		// WebSocket endpoint
		r.Get("/ws", func(c *onihttp.Context) error {
			hub.ServeHTTP(c.Response, c.Request.Request)
			return nil
		})

		// Serve uploaded media (posts, avatars, stories)
		r.Get("/storage/*", func(c *onihttp.Context) error {
			http.ServeFile(c.Response, c.Request.Request, c.Request.URL.Path[1:])
			return nil
		})

		// Serve Vite production build assets (CSS, JS, source maps)
		r.Get("/assets/*", func(c *onihttp.Context) error {
			http.ServeFile(c.Response, c.Request.Request, "public/build"+c.Request.URL.Path)
			return nil
		})

		// Auth routes
		r.Group("/api/auth", func(g *routing.Group) {
			g.Post("/register", authCtrl.Register)
			g.Post("/login", authCtrl.Login)
			g.Post("/logout", authCtrl.Logout)
			g.Get("/me", authMW(authCtrl.Me))
		})

		// User routes
		r.Group("/api/users", func(g *routing.Group) {
			g.Get("/suggestions", authMW(userCtrl.Suggestions))
			g.Get("/search", optionalAuthMW(userCtrl.Search))
			g.Put("/me", authMW(userCtrl.Update))
			g.Post("/me/avatar", authMW(userCtrl.UpdateAvatar))
			g.Get("/:username", optionalAuthMW(userCtrl.Show))
			g.Get("/:username/posts", optionalAuthMW(postCtrl.UserPosts))
			g.Get("/:username/followers", userCtrl.Followers)
			g.Get("/:username/following", userCtrl.Following)
			g.Post("/:username/follow", authMW(userCtrl.Follow))
			g.Delete("/:username/follow", authMW(userCtrl.Unfollow))
		})

		// Feed & posts
		r.Get("/api/feed", authMW(postCtrl.Feed))
		r.Get("/api/explore", optionalAuthMW(postCtrl.Explore))
		r.Group("/api/posts", func(g *routing.Group) {
			g.Post("/", authMW(postCtrl.Store))
			g.Get("/:id", optionalAuthMW(postCtrl.Show))
			g.Delete("/:id", authMW(postCtrl.Destroy))
			g.Post("/:id/like", authMW(likeCtrl.Store))
			g.Delete("/:id/like", authMW(likeCtrl.Destroy))
			g.Post("/:id/bookmark", authMW(bookmarkCtrl.Store))
			g.Delete("/:id/bookmark", authMW(bookmarkCtrl.Destroy))
			g.Get("/:id/comments", optionalAuthMW(commentCtrl.Index))
			g.Post("/:id/comments", authMW(commentCtrl.Store))
		})

		// Comments
		r.Group("/api/comments", func(g *routing.Group) {
			g.Delete("/:id", authMW(commentCtrl.Delete))
			g.Post("/:id/like", authMW(commentCtrl.LikeComment))
			g.Delete("/:id/like", authMW(commentCtrl.UnlikeComment))
		})

		// Stories
		r.Group("/api/stories", func(g *routing.Group) {
			g.Get("/feed", authMW(storyCtrl.Feed))
			g.Post("/", authMW(storyCtrl.Store))
			g.Post("/:id/view", authMW(storyCtrl.MarkViewed))
			g.Delete("/:id", authMW(storyCtrl.Destroy))
		})

		// Direct Messages
		r.Group("/api/messages", func(g *routing.Group) {
			g.Get("/", authMW(msgCtrl.Inbox))
			g.Get("/:username", authMW(msgCtrl.Thread))
			g.Post("/:username", authMW(msgCtrl.Send))
		})

		// Bookmarks
		r.Get("/api/bookmarks", authMW(bookmarkCtrl.Index))

		// Notifications
		r.Group("/api/notifications", func(g *routing.Group) {
			g.Get("/", authMW(notifCtrl.Index))
			g.Post("/read-all", authMW(notifCtrl.MarkAllRead))
			g.Post("/:id/read", authMW(notifCtrl.MarkRead))
		})

		// Catch-all: serve the SPA shell for all non-API routes
		r.Get("/*", func(c *onihttp.Context) error {
			return serveIndex(c, fe, appMode)
		})
	})

	oni.Serve()
}

// serveIndex serves the SPA shell HTML with Vite asset tags injected.
func serveIndex(c *onihttp.Context, fe *frontend.Manager, mode string) error {
	viteTag := fe.ViteTag("resources/ts/app.ts")
	html := `<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>OniGram</title>
  ` + viteTag + `
</head>
<body class="bg-gray-950 text-white min-h-screen">
  <div id="app"></div>
</body>
</html>`
	return c.HTML(200, html)
}

// handleOniCmd dispatches framework CLI commands (migrate, rollback, etc.).
func handleOniCmd(cmd string) error {
	_ = config.LoadEnv(".env")
	port, _ := strconv.Atoi(oniEnv("DB_PORT", "5432"))
	db, err := database.Open(database.Config{
		Driver:   database.Driver(oniEnv("DB_DRIVER", "postgres")),
		Host:     oniEnv("DB_HOST", "127.0.0.1"),
		Port:     port,
		Name:     oniEnv("DB_NAME", "onigram"),
		User:     oniEnv("DB_USER", "postgres"),
		Password: oniEnv("DB_PASSWORD", "password"),
		SSLMode:  "disable",
		MaxOpen:  5,
		MaxIdle:  2,
	})
	if err != nil {
		return fmt.Errorf("database connect: %w", err)
	}
	defer db.Close()

	m := migrations.New(db.SQLDB(), string(db.Driver()))
	m.LoadRegistry()

	ctx := context.Background()
	switch cmd {
	case "migrate":
		return m.Migrate(ctx)
	case "migrate:rollback":
		return m.Rollback(ctx)
	case "migrate:fresh":
		return m.Fresh(ctx)
	case "migrate:status":
		statuses, err := m.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("%-60s %s\n", "Migration", "Status")
		fmt.Println(strings.Repeat("-", 72))
		for _, s := range statuses {
			status := "pending"
			if s.Ran {
				status = fmt.Sprintf("ran (batch %d, at %s)", s.Batch, s.RanAt.Format(time.RFC3339))
			}
			fmt.Printf("%-60s %s\n", s.Name, status)
		}
	default:
		fmt.Printf("[oni] command not handled in app: %s\n", cmd)
	}
	return nil
}

func oniEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
