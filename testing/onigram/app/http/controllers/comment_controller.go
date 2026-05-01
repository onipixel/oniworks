package controllers

import (
	"strconv"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// Delete removes a comment (owner only).
// DELETE /api/comments/:id
func (ctrl *CommentController) Delete(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid comment id")
	}

	var comment models.Comment
	if err := database.Table("comments").Where("id = ?", commentID).First(&comment); err != nil {
		return c.Abort(404, "comment not found")
	}
	if comment.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	_ = database.Table("comments").Where("id = ?", commentID).Delete()
	return c.JSON(200, map[string]any{"message": "deleted"})
}

// LikeComment likes a comment.
// POST /api/comments/:id/like
func (ctrl *CommentController) LikeComment(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid comment id")
	}

	_ = database.Raw(
		`INSERT INTO comment_likes (user_id, comment_id, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		userID, commentID, time.Now(),
	).Exec()

	count, _ := database.Table("comment_likes").Where("comment_id = ?", commentID).Count()
	return c.JSON(200, map[string]any{"like_count": count})
}

// UnlikeComment removes a comment like.
// DELETE /api/comments/:id/like
func (ctrl *CommentController) UnlikeComment(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid comment id")
	}

	_ = database.Table("comment_likes").Where("user_id = ? AND comment_id = ?", userID, commentID).Delete()
	count, _ := database.Table("comment_likes").Where("comment_id = ?", commentID).Count()
	return c.JSON(200, map[string]any{"like_count": count})
}

// CommentController manages comments on posts.
type CommentController struct {
	NotifyFn func(notif *models.Notification)
}

// Index lists comments for a post.
// GET /api/posts/:id/comments
func (ctrl *CommentController) Index(c *onihttp.Context) error {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	comments := make([]models.Comment, 0)
	if err := database.Table("comments").
		Where("post_id = ?", postID).
		OrderBy("created_at ASC").
		Limit(100).
		All(&comments); err != nil {
		return err
	}

	// Batch-load comment authors
	if len(comments) > 0 {
		userIDs := make([]any, 0, len(comments))
		seen := map[int64]bool{}
		for _, cmt := range comments {
			if !seen[cmt.UserID] {
				userIDs = append(userIDs, cmt.UserID)
				seen[cmt.UserID] = true
			}
		}
		var users []models.User
		_ = database.Table("users").
			Select("id", "username", "avatar_path").
			WhereIn("id", userIDs...).All(&users)
		userMap := make(map[int64]*models.User, len(users))
		for i := range users {
			userMap[users[i].ID] = &users[i]
		}
		for i := range comments {
			if u, ok := userMap[comments[i].UserID]; ok {
				comments[i].User = u
			}
		}
	}

	return c.JSON(200, map[string]any{"comments": comments})
}

// Store adds a comment to a post.
// POST /api/posts/:id/comments
func (ctrl *CommentController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := c.Bind(&req); err != nil {
		return c.Abort(400, "invalid body")
	}
	if len(req.Body) == 0 {
		return c.Abort(422, "comment body is required")
	}

	var post models.Post
	if err := database.Table("posts").Where("id = ?", postID).First(&post); err != nil {
		return c.Abort(404, "post not found")
	}

	comment := &models.Comment{
		UserID:    userID,
		PostID:    postID,
		Body:      req.Body,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.Table("comments").Insert(comment); err != nil {
		return err
	}

	// Notify post author
	if post.UserID != userID {
		notif := &models.Notification{
			UserID:    post.UserID,
			ActorID:   userID,
			Type:      models.NotifComment,
			PostID:    &postID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := database.Table("notifications").Insert(notif); err == nil && ctrl.NotifyFn != nil {
			ctrl.NotifyFn(notif)
		}
	}

	// Load author for the response
	var author models.User
	_ = database.Table("users").Select("id", "username", "avatar_path").
		Where("id = ?", userID).First(&author)
	comment.User = &author

	return c.JSON(201, comment)
}
