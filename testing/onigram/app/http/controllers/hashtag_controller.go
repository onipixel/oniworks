package controllers

import (
	"strconv"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// HashtagController handles hashtag discovery and feeds.
type HashtagController struct{}

// Trending returns the top hashtags by post count.
// GET /api/hashtags/trending
func (ctrl *HashtagController) Trending(c *onihttp.Context) error {
	type row struct {
		Tag       string `db:"tag"        json:"tag"`
		PostCount int64  `db:"post_count" json:"post_count"`
	}
	var rows []row
	_ = database.Raw(`
		SELECT h.tag, COUNT(ph.post_id) AS post_count
		FROM hashtags h
		JOIN post_hashtags ph ON ph.hashtag_id = h.id
		GROUP BY h.id, h.tag
		ORDER BY post_count DESC
		LIMIT 10`).All(&rows)
	if rows == nil {
		rows = []row{}
	}
	return c.JSON(200, map[string]any{"hashtags": rows})
}

// Feed returns posts tagged with :tag.
// GET /api/hashtags/:tag
func (ctrl *HashtagController) Feed(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	viewerID, _ := uid.(int64)

	tag := c.Param("tag")
	page, _ := strconv.Atoi(c.QueryD("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 30

	var hashtag models.Hashtag
	if err := database.Table("hashtags").Where("tag = ?", tag).First(&hashtag); err != nil {
		return c.JSON(200, map[string]any{"posts": []any{}, "tag": tag, "page": page})
	}

	posts := make([]models.Post, 0)
	err := database.Table("posts").
		Select("posts.*").
		Join("post_hashtags ON post_hashtags.post_id = posts.id").
		Where("post_hashtags.hashtag_id = ?", hashtag.ID).
		OrderBy("posts.created_at DESC").
		Limit(30).Offset(offset).
		All(&posts)
	if err != nil {
		return err
	}

	enrichPosts(posts, viewerID)
	return c.JSON(200, map[string]any{"posts": posts, "tag": tag, "page": page})
}
