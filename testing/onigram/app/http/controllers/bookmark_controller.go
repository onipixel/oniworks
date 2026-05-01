package controllers

import (
	"strconv"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

type BookmarkController struct{}

// Store bookmarks a post.
// POST /api/posts/:id/bookmark
func (ctrl *BookmarkController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	_ = database.Raw(
		`INSERT INTO bookmarks (user_id, post_id, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		userID, postID, time.Now(),
	).Exec()

	return c.JSON(200, map[string]any{"bookmarked": true})
}

// Destroy removes a bookmark.
// DELETE /api/posts/:id/bookmark
func (ctrl *BookmarkController) Destroy(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	_ = database.Table("bookmarks").Where("user_id = ? AND post_id = ?", userID, postID).Delete()
	return c.JSON(200, map[string]any{"bookmarked": false})
}

// Index returns the authenticated user's bookmarked posts.
// GET /api/bookmarks
func (ctrl *BookmarkController) Index(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	posts := make([]models.Post, 0)
	err := database.Table("posts").
		Select("posts.*").
		Join("bookmarks ON bookmarks.post_id = posts.id").
		Where("bookmarks.user_id = ?", userID).
		OrderBy("bookmarks.created_at DESC").
		Limit(60).
		All(&posts)
	if err != nil {
		return err
	}

	enrichPosts(posts, userID)
	return c.JSON(200, map[string]any{"posts": posts})
}
