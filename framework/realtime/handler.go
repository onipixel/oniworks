package realtime

import (
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// Handler returns an onihttp.HandlerFunc that upgrades HTTP connections to WebSocket.
// Mount it with: router.Get("/ws", hub.Handler())
func (h *Hub) Handler() onihttp.HandlerFunc {
	return func(c *onihttp.Context) error {
		h.ServeHTTP(c.Response, c.Request.Request)
		return nil
	}
}
