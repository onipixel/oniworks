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

type StoryController struct{}

// Feed returns active stories (not expired) from followed users + self,
// grouped by user.
// GET /api/stories/feed
func (ctrl *StoryController) Feed(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	stories := make([]models.Story, 0)
	err := database.Table("stories").
		SelectRaw("DISTINCT stories.*").
		LeftJoin("follows ON follows.following_id = stories.user_id").
		Where("(follows.follower_id = ? OR stories.user_id = ?) AND stories.expires_at > NOW()", userID, userID).
		OrderBy("stories.created_at DESC").
		Limit(100).
		All(&stories)
	if err != nil {
		return err
	}

	if len(stories) == 0 {
		return c.JSON(200, map[string]any{"stories": stories})
	}

	// Batch load users
	userIDs := make([]any, 0, len(stories))
	seen := map[int64]bool{}
	for _, s := range stories {
		if !seen[s.UserID] {
			userIDs = append(userIDs, s.UserID)
			seen[s.UserID] = true
		}
	}
	var storyUsers []models.User
	_ = database.Table("users").Select("id", "username", "avatar_path").
		WhereIn("id", userIDs...).All(&storyUsers)
	userMap := make(map[int64]*models.User, len(storyUsers))
	for i := range storyUsers {
		userMap[storyUsers[i].ID] = &storyUsers[i]
	}

	// Batch check which stories viewer has seen
	storyIDs := make([]any, len(stories))
	for i, s := range stories {
		storyIDs[i] = s.ID
	}
	type viewRow struct {
		StoryID int64 `db:"story_id"`
	}
	var viewRows []viewRow
	_ = database.Table("story_views").
		Select("story_id").
		Where("viewer_id = ?", userID).
		WhereIn("story_id", storyIDs...).
		All(&viewRows)
	viewedSet := map[int64]bool{}
	for _, v := range viewRows {
		viewedSet[v.StoryID] = true
	}

	for i := range stories {
		if u, ok := userMap[stories[i].UserID]; ok {
			stories[i].User = u
		}
		stories[i].Viewed = viewedSet[stories[i].ID]
	}

	return c.JSON(200, map[string]any{"stories": stories})
}

// Store uploads a new story (24-hour expiry).
// POST /api/stories
func (ctrl *StoryController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	uf, err := c.ParseUpload("image", onihttp.UploadConfig{
		MaxSize:      10 << 20,
		AllowedTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
	})
	if err != nil {
		return c.Abort(422, "invalid image: "+err.Error())
	}

	filename := fmt.Sprintf("story_%d_%d%s", userID, time.Now().UnixNano(), uf.Ext())
	savedPath, err := uf.Store("storage/stories", filename)
	if err != nil {
		return err
	}
	urlPath := "/" + strings.ReplaceAll(savedPath, "\\", "/")

	story := &models.Story{
		UserID:    userID,
		ImagePath: urlPath,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := database.Table("stories").Insert(story); err != nil {
		return err
	}
	return c.JSON(201, story)
}

// MarkViewed marks a story as viewed by the authenticated user.
// POST /api/stories/:id/view
func (ctrl *StoryController) MarkViewed(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	storyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid story id")
	}

	// Upsert — ignore duplicate
	_ = database.Raw(
		`INSERT INTO story_views (story_id, viewer_id, viewed_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		storyID, userID, time.Now(),
	).Exec()

	return c.JSON(200, map[string]any{"ok": true})
}

// Destroy deletes own story.
// DELETE /api/stories/:id
func (ctrl *StoryController) Destroy(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	storyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid story id")
	}

	var story models.Story
	if err := database.Table("stories").Where("id = ?", storyID).First(&story); err != nil {
		return c.Abort(404, "story not found")
	}
	if story.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	_ = database.Table("stories").Where("id = ?", storyID).Delete()
	return c.JSON(200, map[string]any{"message": "deleted"})
}
