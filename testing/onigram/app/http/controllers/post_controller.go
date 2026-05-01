package controllers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// PostController handles photo posts and the home feed.
type PostController struct{}

// Feed returns the paginated home feed for the authenticated user.
// GET /api/feed
func (ctrl *PostController) Feed(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	page, _ := strconv.Atoi(c.QueryD("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20

	// Posts from users the current user follows, plus their own posts
	posts := make([]models.Post, 0)
	err := database.Table("posts").
		Select("DISTINCT posts.*").
		LeftJoin("follows ON follows.following_id = posts.user_id").
		Where("follows.follower_id = ? OR posts.user_id = ?", userID, userID).
		OrderBy("posts.created_at DESC").
		Limit(20).Offset(offset).
		All(&posts)
	if err != nil {
		return err
	}

	// Enrich posts with author info and like counts
	enrichPosts(posts, userID)

	return c.JSON(200, map[string]any{"posts": posts, "page": page})
}

// Show returns a single post.
// GET /api/posts/:id
func (ctrl *PostController) Show(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	viewerID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	var post models.Post
	if err := database.Table("posts").Where("id = ?", postID).First(&post); err != nil {
		if err == database.ErrNotFound {
			return c.Abort(404, "post not found")
		}
		return err
	}
	ps := []models.Post{post}
	enrichPosts(ps, viewerID)

	return c.JSON(200, ps[0])
}

// Store creates a new post with an uploaded image.
// POST /api/posts  (multipart/form-data)
func (ctrl *PostController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	uf, err := c.ParseUpload("image", onihttp.UploadConfig{
		MaxSize:      20 << 20, // 20 MB
		AllowedTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
	})
	if err != nil {
		return c.Abort(422, "invalid image: "+err.Error())
	}

	filename := fmt.Sprintf("post_%d_%d%s", userID, time.Now().UnixNano(), uf.Ext())
	savedPath, err := uf.Store("storage/posts", filename)
	if err != nil {
		return err
	}
	urlPath := "/" + strings.ReplaceAll(savedPath, "\\", "/")
	caption := c.FormValue("caption")

	post := &models.Post{
		UserID:    userID,
		ImagePath: urlPath,
		Caption:   caption,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.Table("posts").Insert(post); err != nil {
		return err
	}

	return c.JSON(201, post)
}

// Destroy deletes a post owned by the authenticated user.
// DELETE /api/posts/:id
func (ctrl *PostController) Destroy(c *onihttp.Context) error {
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
	if post.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	if err := database.Table("posts").Where("id = ?", postID).Delete(); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"message": "deleted"})
}

// UserPosts returns all posts by a username.
// GET /api/users/:username/posts
func (ctrl *PostController) UserPosts(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	viewerID, _ := uid.(int64)
	username := c.Param("username")

	var owner models.User
	if err := database.Table("users").Where("username = ?", username).First(&owner); err != nil {
		return c.Abort(404, "user not found")
	}

	posts := make([]models.Post, 0)
	if err := database.Table("posts").Where("user_id = ?", owner.ID).
		OrderBy("created_at DESC").Limit(30).All(&posts); err != nil {
		return err
	}
	enrichPosts(posts, viewerID)
	return c.JSON(200, map[string]any{"posts": posts})
}

// Explore returns recent public posts for discovery (no follow required).
// GET /api/explore
func (ctrl *PostController) Explore(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	viewerID, _ := uid.(int64)

	page, _ := strconv.Atoi(c.QueryD("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 30

	posts := make([]models.Post, 0)
	if err := database.Table("posts").
		OrderBy("created_at DESC").
		Limit(30).Offset(offset).
		All(&posts); err != nil {
		return err
	}

	enrichPosts(posts, viewerID)
	return c.JSON(200, map[string]any{"posts": posts, "page": page})
}

// enrichPosts loads author info and like counts for a slice of posts.
// Uses batch queries — no N+1.
func enrichPosts(posts []models.Post, viewerID int64) {
	if len(posts) == 0 {
		return
	}
	ids := make([]any, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	idxByID := make(map[int64]int, len(posts))
	for i, p := range posts {
		idxByID[p.ID] = i
	}

	// Batch load like counts
	type likeCount struct {
		PostID int64 `db:"post_id"`
		Count  int64 `db:"count"`
	}
	var counts []likeCount
	_ = database.Table("likes").
		Select("post_id", "COUNT(*) AS count").
		WhereIn("post_id", ids...).
		GroupBy("post_id").
		All(&counts)
	for _, lc := range counts {
		if i, ok := idxByID[lc.PostID]; ok {
			posts[i].LikeCount = int(lc.Count)
		}
	}

	// Batch check viewer likes
	if viewerID != 0 {
		type liked struct {
			PostID int64 `db:"post_id"`
		}
		var likedRows []liked
		_ = database.Table("likes").
			Select("post_id").
			Where("user_id = ?", viewerID).
			WhereIn("post_id", ids...).
			All(&likedRows)
		for _, lr := range likedRows {
			if i, ok := idxByID[lr.PostID]; ok {
				posts[i].IsLiked = true
			}
		}
	}

	// Batch check viewer bookmarks
	if viewerID != 0 {
		type bookmarked struct {
			PostID int64 `db:"post_id"`
		}
		var bookmarkRows []bookmarked
		_ = database.Table("bookmarks").
			Select("post_id").
			Where("user_id = ?", viewerID).
			WhereIn("post_id", ids...).
			All(&bookmarkRows)
		for _, br := range bookmarkRows {
			if i, ok := idxByID[br.PostID]; ok {
				posts[i].IsBookmarked = true
			}
		}
	}

	// Batch load authors
	userIDs := make([]any, 0, len(posts))
	seen := map[int64]bool{}
	for _, p := range posts {
		if !seen[p.UserID] {
			userIDs = append(userIDs, p.UserID)
			seen[p.UserID] = true
		}
	}
	var users []models.User
	_ = database.Table("users").
		Select("id", "username", "avatar_path").
		WhereIn("id", userIDs...).
		All(&users)
	userMap := make(map[int64]*models.User, len(users))
	for i := range users {
		userMap[users[i].ID] = &users[i]
	}
	for i := range posts {
		if u, ok := userMap[posts[i].UserID]; ok {
			posts[i].User = u
		}
	}
}
