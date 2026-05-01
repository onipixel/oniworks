package controllers

import (
	"strconv"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// LikeController handles liking and unliking posts.
type LikeController struct {
	// NotifyFn is called after a like so the caller can broadcast a realtime event.
	NotifyFn func(notif *models.Notification)
}

// Store likes a post.
// POST /api/posts/:id/like
func (ctrl *LikeController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	var post models.Post
	if err := database.Table("posts").Where("id = ?", postID).First(&post); err != nil {
		return c.Abort(404, "post not found")
	}

	already, _ := database.Table("likes").
		Where("user_id = ? AND post_id = ?", userID, postID).Exists()
	if already {
		return c.JSON(200, map[string]any{"message": "already liked"})
	}

	like := &models.Like{
		UserID:    userID,
		PostID:    postID,
		CreatedAt: time.Now(),
	}
	if err := database.Table("likes").Insert(like); err != nil {
		return err
	}

	// Notify post author (skip if the author liked their own post)
	if post.UserID != userID {
		notif := &models.Notification{
			UserID:    post.UserID,
			ActorID:   userID,
			Type:      models.NotifLike,
			PostID:    &postID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := database.Table("notifications").Insert(notif); err == nil && ctrl.NotifyFn != nil {
			ctrl.NotifyFn(notif)
		}
	}

	likeCount, _ := database.Table("likes").Where("post_id = ?", postID).Count()
	return c.JSON(201, map[string]any{"like_count": likeCount})
}

// Destroy removes a like.
// DELETE /api/posts/:id/like
func (ctrl *LikeController) Destroy(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	if err := database.Table("likes").
		Where("user_id = ? AND post_id = ?", userID, postID).Delete(); err != nil {
		return err
	}

	likeCount, _ := database.Table("likes").Where("post_id = ?", postID).Count()
	return c.JSON(200, map[string]any{"like_count": likeCount})
}
