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

// notifRow is a flat scan target that avoids nullable pointer fields.
type notifRow struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	ActorID   int64     `db:"actor_id"`
	Type      string    `db:"type"`
	PostIDRaw int64     `db:"post_id"`
	Read      bool      `db:"read"`
	CreatedAt time.Time `db:"created_at"`
}

// Index returns unread (and recent) notifications for the authenticated user.
// GET /api/notifications
func (ctrl *NotificationController) Index(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	var rows []notifRow
	if err := database.Raw(
		`SELECT id, user_id, actor_id, type, COALESCE(post_id, 0) AS post_id, read, created_at
		 FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`, userID,
	).All(&rows); err != nil {
		return err
	}

	// Convert to model slice
	notifs := make([]models.Notification, 0, len(rows))
	for _, r := range rows {
		n := models.Notification{
			ID:        r.ID,
			UserID:    r.UserID,
			ActorID:   r.ActorID,
			Type:      r.Type,
			Read:      r.Read,
			CreatedAt: r.CreatedAt,
		}
		if r.PostIDRaw > 0 {
			pid := r.PostIDRaw
			n.PostID = &pid
		}
		notifs = append(notifs, n)
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
		Update(database.Map{"read": true}); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"message": "all marked as read"})
}
