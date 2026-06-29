package controllers

import (
	"strconv"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// CommentController manages comments on posts.
type CommentController struct {
	NotifyFn func(notif *models.Notification)
}

// Index lists top-level comments for a post, with replies nested.
// GET /api/posts/:id/comments
func (ctrl *CommentController) Index(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	viewerID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	// Load all comments for this post — pinned first, then oldest
	all := make([]models.Comment, 0)
	if err := database.Table("comments").
		Where("post_id = ?", postID).
		OrderBy("is_pinned DESC, created_at ASC").
		Limit(200).
		All(&all); err != nil {
		return err
	}

	enrichComments(all, viewerID)

	// Separate top-level and replies, then nest
	topLevel := make([]models.Comment, 0)
	replyMap := make(map[int64][]models.Comment)
	for _, cmt := range all {
		if cmt.ParentCommentID != nil {
			replyMap[*cmt.ParentCommentID] = append(replyMap[*cmt.ParentCommentID], cmt)
		} else {
			topLevel = append(topLevel, cmt)
		}
	}
	for i := range topLevel {
		if replies, ok := replyMap[topLevel[i].ID]; ok {
			topLevel[i].Replies = replies
		}
	}

	return c.JSON(200, map[string]any{"comments": topLevel})
}

// Store adds a comment (or reply) to a post.
// POST /api/posts/:id/comments
func (ctrl *CommentController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	var req struct {
		Body            string `json:"body"             validate:"required,min=1,max=2200"`
		ParentCommentID *int64 `json:"parent_comment_id"`
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	var post models.Post
	if err := database.Table("posts").Where("id = ?", postID).First(&post); err != nil {
		return c.Abort(404, "post not found")
	}

	// Validate parent comment belongs to the same post
	if req.ParentCommentID != nil {
		var parent models.Comment
		if err := database.Table("comments").Where("id = ? AND post_id = ?", *req.ParentCommentID, postID).First(&parent); err != nil {
			return c.Abort(404, "parent comment not found")
		}
	}

	comment := &models.Comment{
		UserID:          userID,
		PostID:          postID,
		ParentCommentID: req.ParentCommentID,
		Body:            req.Body,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := database.Table("comments").Insert(comment); err != nil {
		return err
	}

	// Notify post author (not for self-comments)
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

	// Notify @mentions in the comment body
	notifyMentions(userID, &postID, req.Body, ctrl.NotifyFn)

	// Load author for the response
	var author models.User
	_ = database.Table("users").Select("id", "username", "avatar_path").
		Where("id = ?", userID).First(&author)
	comment.User = &author

	return c.JSON(201, comment)
}

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

// Pin pins a comment to the top of the post (post owner only, one pin per post).
// POST /api/comments/:id/pin
func (ctrl *CommentController) Pin(c *onihttp.Context) error {
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

	// Only the post owner can pin
	var post models.Post
	if err := database.Table("posts").Where("id = ?", comment.PostID).First(&post); err != nil {
		return c.Abort(404, "post not found")
	}
	if post.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	// Unpin any currently pinned comment on this post
	_ = database.Table("comments").
		Where("post_id = ? AND is_pinned = ?", comment.PostID, true).
		Update(database.Map{"is_pinned": false})

	// Pin this one
	if err := database.Table("comments").Where("id = ?", commentID).
		Update(database.Map{"is_pinned": true}); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"message": "pinned"})
}

// Unpin removes the pin from a comment (post owner only).
// DELETE /api/comments/:id/pin
func (ctrl *CommentController) Unpin(c *onihttp.Context) error {
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

	var post models.Post
	if err := database.Table("posts").Where("id = ?", comment.PostID).First(&post); err != nil {
		return c.Abort(404, "post not found")
	}
	if post.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	_ = database.Table("comments").Where("id = ?", commentID).
		Update(database.Map{"is_pinned": false})
	return c.JSON(200, map[string]any{"message": "unpinned"})
}

// enrichComments batch-loads authors and like data for comments.
func enrichComments(comments []models.Comment, viewerID int64) {
	if len(comments) == 0 {
		return
	}

	ids := make([]any, len(comments))
	for i, c := range comments {
		ids[i] = c.ID
	}
	idxByID := make(map[int64]int, len(comments))
	for i, c := range comments {
		idxByID[c.ID] = i
	}

	// Batch load like counts
	type likeCount struct {
		CommentID int64 `db:"comment_id"`
		Count     int64 `db:"count"`
	}
	var lcs []likeCount
	_ = database.Table("comment_likes").
		Select("comment_id").SelectRaw("COUNT(*) AS count").
		WhereIn("comment_id", ids...).
		GroupBy("comment_id").
		All(&lcs)
	for _, lc := range lcs {
		if i, ok := idxByID[lc.CommentID]; ok {
			comments[i].LikeCount = int(lc.Count)
		}
	}

	// Batch check viewer likes
	if viewerID != 0 {
		type viewerLike struct {
			CommentID int64 `db:"comment_id"`
		}
		var viewerLikes []viewerLike
		_ = database.Table("comment_likes").
			Select("comment_id").
			Where("user_id = ?", viewerID).
			WhereIn("comment_id", ids...).
			All(&viewerLikes)
		for _, l := range viewerLikes {
			if i, ok := idxByID[l.CommentID]; ok {
				comments[i].IsLiked = true
			}
		}
	}

	// Batch load authors
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
