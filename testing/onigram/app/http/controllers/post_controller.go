package controllers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

var hashtagRe = regexp.MustCompile(`#(\w+)`)

// PostController handles photo posts and the home feed.
type PostController struct {
	NotifyFn func(notif *models.Notification)
}

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

	posts := make([]models.Post, 0)
	err := database.Table("posts").
		SelectRaw("DISTINCT posts.*").
		LeftJoin("follows ON follows.following_id = posts.user_id").
		Where("follows.follower_id = ? OR posts.user_id = ?", userID, userID).
		OrderBy("posts.created_at DESC").
		Limit(20).Offset(offset).
		All(&posts)
	if err != nil {
		return err
	}

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

// Store creates a new post with one or more uploaded images.
// POST /api/posts  (multipart/form-data, field name "images[]" or "image")
func (ctrl *PostController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	caption := c.FormValue("caption")

	// Collect uploaded files — try multi-file "images[]" first, fall back to "image"
	uploadCfg := onihttp.UploadConfig{
		MaxSize:      20 << 20,
		AllowedTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
	}

	type savedImg struct {
		path string
	}
	var savedImages []savedImg

	// Try multi-file upload
	for i := 0; i < 10; i++ {
		field := fmt.Sprintf("images[%d]", i)
		uf, err := c.ParseUpload(field, uploadCfg)
		if err != nil {
			break
		}
		filename := fmt.Sprintf("post_%d_%d_%d%s", userID, time.Now().UnixNano(), i, uf.Ext())
		sp, err := uf.Store("storage/posts", filename)
		if err != nil {
			return err
		}
		savedImages = append(savedImages, savedImg{path: "/" + strings.ReplaceAll(sp, "\\", "/")})
	}

	// Fall back to single "image" field
	if len(savedImages) == 0 {
		uf, err := c.ParseUpload("image", uploadCfg)
		if err != nil {
			return c.Abort(422, "image is required: "+err.Error())
		}
		filename := fmt.Sprintf("post_%d_%d%s", userID, time.Now().UnixNano(), uf.Ext())
		sp, err := uf.Store("storage/posts", filename)
		if err != nil {
			return err
		}
		savedImages = append(savedImages, savedImg{path: "/" + strings.ReplaceAll(sp, "\\", "/")})
	}

	post := &models.Post{
		UserID:    userID,
		ImagePath: savedImages[0].path,
		Caption:   caption,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.Table("posts").Insert(post); err != nil {
		return err
	}

	// Store additional images
	if len(savedImages) > 1 {
		for pos, img := range savedImages {
			pi := &models.PostImage{
				PostID:    post.ID,
				ImagePath: img.path,
				Position:  pos,
			}
			_ = database.Table("post_images").Insert(pi)
		}
	}

	// Extract and link hashtags + notify @mentions
	if caption != "" {
		linkHashtags(post.ID, caption)
		notifyMentions(userID, &post.ID, caption, ctrl.NotifyFn)
	}

	return c.JSON(201, post)
}

// Edit updates the caption of a post owned by the authenticated user.
// PUT /api/posts/:id
func (ctrl *PostController) Edit(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid post id")
	}

	var req struct {
		Caption string `json:"caption" validate:"max=2200"`
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	var post models.Post
	if err := database.Table("posts").Where("id = ?", postID).First(&post); err != nil {
		return c.Abort(404, "post not found")
	}
	if post.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	if err := database.Table("posts").Where("id = ?", postID).
		Update(database.Map{"caption": req.Caption, "updated_at": time.Now()}); err != nil {
		return err
	}

	// Re-extract hashtags: drop old links and re-insert
	_ = database.Raw(`DELETE FROM post_hashtags WHERE post_id = $1`, postID).Exec()
	if req.Caption != "" {
		linkHashtags(postID, req.Caption)
		notifyMentions(userID, &postID, req.Caption, ctrl.NotifyFn)
	}

	post.Caption = req.Caption
	return c.JSON(200, post)
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

// enrichPosts loads author info, like/comment counts, bookmarks, and carousel images.
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
		Select("post_id").SelectRaw("COUNT(*) AS count").
		WhereIn("post_id", ids...).
		GroupBy("post_id").
		All(&counts)
	for _, lc := range counts {
		if i, ok := idxByID[lc.PostID]; ok {
			posts[i].LikeCount = int(lc.Count)
		}
	}

	// Batch load comment counts
	type commentCount struct {
		PostID int64 `db:"post_id"`
		Count  int64 `db:"count"`
	}
	var cmtCounts []commentCount
	_ = database.Table("comments").
		Select("post_id").SelectRaw("COUNT(*) AS count").
		WhereIn("post_id", ids...).
		GroupBy("post_id").
		All(&cmtCounts)
	for _, cc := range cmtCounts {
		if i, ok := idxByID[cc.PostID]; ok {
			posts[i].CommentCount = int(cc.Count)
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

	// Batch load carousel images
	var postImages []models.PostImage
	_ = database.Table("post_images").
		WhereIn("post_id", ids...).
		OrderBy("post_id, position ASC").
		All(&postImages)
	imgByPost := make(map[int64][]models.PostImage)
	for _, pi := range postImages {
		imgByPost[pi.PostID] = append(imgByPost[pi.PostID], pi)
	}
	for i := range posts {
		if imgs, ok := imgByPost[posts[i].ID]; ok {
			posts[i].Images = imgs
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

// linkHashtags extracts #tags from a caption and upserts them into the hashtags
// and post_hashtags tables.
func linkHashtags(postID int64, caption string) {
	matches := hashtagRe.FindAllStringSubmatch(strings.ToLower(caption), -1)
	seen := map[string]bool{}
	for _, m := range matches {
		tag := m[1]
		if seen[tag] {
			continue
		}
		seen[tag] = true

		// Upsert hashtag
		_ = database.Raw(
			`INSERT INTO hashtags (tag) VALUES ($1) ON CONFLICT (tag) DO NOTHING`, tag,
		).Exec()

		var h models.Hashtag
		if err := database.Table("hashtags").Where("tag = ?", tag).First(&h); err != nil {
			continue
		}

		// Link to post (ignore if already linked)
		_ = database.Raw(
			`INSERT INTO post_hashtags (post_id, hashtag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			postID, h.ID,
		).Exec()
	}
}
