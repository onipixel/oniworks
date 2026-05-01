package channels

import (
	"fmt"

	"github.com/onipixel/oniworks/framework/realtime"
)

// RegisterDMChannel registers the per-user direct message channel.
// Clients subscribe by sending an event to "dm.{user_id}".
func RegisterDMChannel(hub *realtime.Hub) {
	hub.Channel("dm.{user_id}", func(c *realtime.Conn, e *realtime.Event) error {
		channelUserID := e.Params["user_id"]
		if fmt.Sprintf("%d", c.UserID) != channelUserID {
			return fmt.Errorf("unauthorized: cannot subscribe to another user's DM channel")
		}
		c.Subscribe("dm." + channelUserID)
		return nil
	})
}
