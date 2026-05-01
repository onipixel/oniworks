package controllers

import (
	"strconv"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// NotificationController manages in-app notifications.
type NotificationController struct{}

// Index returns unread (and recent) notifications for the authenticated user.
// GET /api/notifications
func (ctrl *NotificationController) Index(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	notifs := make([]models.Notification, 0)
	if err := database.Table("notifications").
		Where("user_id = ?", userID).
		OrderBy("created_at DESC").
		Limit(50).All(&notifs); err != nil {
		return err
	}

	// Batch-load actors
	if len(notifs) > 0 {
		actorIDs := make([]any, 0, len(notifs))
		seen := map[int64]bool{}
		for _, n := range notifs {
			if !seen[n.ActorID] {
				actorIDs = append(actorIDs, n.ActorID)
				seen[n.ActorID] = true
			}
		}
		var actors []models.User
		_ = database.Table("users").
			Select("id", "username", "avatar_path").
			WhereIn("id", actorIDs...).All(&actors)
		actorMap := make(map[int64]*models.User, len(actors))
		for i := range actors {
			actorMap[actors[i].ID] = &actors[i]
		}
		for i := range notifs {
			if u, ok := actorMap[notifs[i].ActorID]; ok {
				notifs[i].Actor = u
			}
		}
	}

	unreadCount, _ := database.Table("notifications").
		Where("user_id = ? AND read = ?", userID, false).Count()

	return c.JSON(200, map[string]any{
		"notifications": notifs,
		"unread_count":  unreadCount,
	})
}

// MarkRead marks one notification as read.
// POST /api/notifications/:id/read
func (ctrl *NotificationController) MarkRead(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	notifID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid notification id")
	}

	if err := database.Table("notifications").
		Where("id = ? AND user_id = ?", notifID, userID).
		Update(database.Map{"read": true}); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"message": "marked as read"})
}

// MarkAllRead marks all notifications as read.
// POST /api/notifications/read-all
func (ctrl *NotificationController) MarkAllRead(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	if err := database.Table("notifications").
		Where("user_id = ? AND read = ?", userID, false).
		Update(database.Map{"read": true, "updated_at": time.Now()}); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"message": "all marked as read"})
}
