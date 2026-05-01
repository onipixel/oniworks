package channels

import (
	"fmt"

	"github.com/onipixel/oniworks/framework/realtime"
)

// RegisterNotifyChannel registers the per-user notification channel on the hub.
// Clients subscribe by sending an event to "notify.{user_id}".
// The hub verifies that the connecting user owns the requested channel.
func RegisterNotifyChannel(hub *realtime.Hub) {
	hub.Channel("notify.{user_id}", func(c *realtime.Conn, e *realtime.Event) error {
		channelUserID := e.Params["user_id"]
		// Only allow users to subscribe to their own notification channel
		if fmt.Sprintf("%d", c.UserID) != channelUserID {
			return fmt.Errorf("unauthorized: cannot subscribe to another user's notification channel")
		}
		c.Subscribe("notify." + channelUserID)
		return nil
	})
}
