// Example: OniWorks Realtime Chat
// Demonstrates Oni Socket + Oni Memory for a cross-node realtime chat.
// Architecture: event → socket → memory → broadcast → UI
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/onipixel/oniworks/framework/app"
	onihttp "github.com/onipixel/oniworks/framework/http"
	"github.com/onipixel/oniworks/framework/memory"
	"github.com/onipixel/oniworks/framework/middleware"
	"github.com/onipixel/oniworks/framework/realtime"
	"github.com/onipixel/oniworks/framework/routing"
)

func main() {
	oni := app.New()
	oni.Load(".env", "config/app.yaml")

	// Oni Memory — distributed state + pub/sub
	mem := memory.New(memory.Options{
		NodeID:      os.Getenv("NODE_ID"),
		GracefulSave: true,
		SnapshotPath: "./storage/memory.snap",
	})

	// Oni Socket hub — realtime nervous system
	hub := realtime.New(realtime.Options{
		Memory: mem,
		AuthFunc: func(r *http.Request) (int64, error) {
			// Simple token-based auth
			token := r.URL.Query().Get("token")
			if token == "" {
				return 0, nil // anonymous
			}
			// TODO: validate token, return user ID
			return 42, nil
		},
	})

	// Register presence for chat rooms
	hub.Presence("chat.{room}")

	// Handle chat messages
	hub.Channel("chat.{room}", func(c *realtime.Conn, e *realtime.Event) error {
		room := e.Params["room"]
		slog.Info("chat message", "room", room, "user", c.UserID)

		// Broadcast to everyone in the room (cross-node via Oni Memory)
		return hub.Broadcast("chat."+room, map[string]any{
			"from":    c.UserID,
			"payload": e.Payload,
		})
	})

	// Handle typing indicators
	hub.Channel("chat.{room}.typing", func(c *realtime.Conn, e *realtime.Event) error {
		room := e.Params["room"]
		return hub.Broadcast("chat."+room+".typing", map[string]any{
			"user": c.UserID,
		})
	})

	// Track online users globally
	hub.OnConnect(func(c *realtime.Conn) error {
		if c.UserID > 0 {
			mem.Set(fmt.Sprintf("user:%d:status", c.UserID), "online", 0)
			_ = hub.BroadcastEvent("presence", &realtime.Event{
				Type: "user:online",
			})
		}
		return nil
	})

	hub.OnDisconnect(func(c *realtime.Conn) {
		if c.UserID > 0 {
			mem.Set(fmt.Sprintf("user:%d:status", c.UserID), "offline", 0)
			_ = hub.BroadcastEvent("presence", &realtime.Event{
				Type: "user:offline",
			})
		}
	})

	oni.Use(middleware.Logger(), middleware.Recovery(), middleware.CORS())

	oni.Route(func(r *routing.Router) {
		r.Get("/ws", func(c *onihttp.Context) error {
			hub.ServeHTTP(c.Response, c.Request.Request)
			return nil
		})

		r.Get("/api/rooms/{room}/presence", func(c *onihttp.Context) error {
			room := c.Param("room")
			members := hub.Members("chat." + room)
			return c.JSON(200, map[string]any{
				"room":    room,
				"count":   len(members),
				"members": members,
			})
		})

		r.Get("/api/stats", func(c *onihttp.Context) error {
			return c.JSON(200, map[string]any{
				"connections": hub.ConnCount(),
			})
		})
	})

	oni.Serve()
}
