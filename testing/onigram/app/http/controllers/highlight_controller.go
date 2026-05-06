package controllers

import (
	"strconv"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// HighlightController manages story highlight reels on profiles.
type HighlightController struct{}

// Index returns all highlights for a user with their stories.
// GET /api/highlights/:username
func (ctrl *HighlightController) Index(c *onihttp.Context) error {
	username := c.Param("username")

	var user models.User
	if err := database.Table("users").Where("username = ?", username).First(&user); err != nil {
		return c.Abort(404, "user not found")
	}

	highlights := make([]models.Highlight, 0)
	if err := database.Table("highlights").
		Where("user_id = ?", user.ID).
		OrderBy("created_at ASC").
		All(&highlights); err != nil {
		return err
	}

	if len(highlights) > 0 {
		// Batch load stories for each highlight
		hlIDs := make([]any, len(highlights))
		for i, h := range highlights {
			hlIDs[i] = h.ID
		}

		type hlStory struct {
			HighlightID int64  `db:"highlight_id"`
			StoryID     int64  `db:"story_id"`
			ImagePath   string `db:"image_path"`
		}
		var rows []hlStory
		_ = database.Raw(`
			SELECT hs.highlight_id, hs.story_id, s.image_path
			FROM highlight_stories hs
			JOIN stories s ON s.id = hs.story_id
			WHERE hs.highlight_id = ANY($1::bigint[])
			ORDER BY hs.added_at ASC`,
			pgArray(hlIDs),
		).All(&rows)

		storiesByHL := map[int64][]models.Story{}
		for _, r := range rows {
			storiesByHL[r.HighlightID] = append(storiesByHL[r.HighlightID], models.Story{
				ID:        r.StoryID,
				ImagePath: r.ImagePath,
			})
		}

		for i := range highlights {
			highlights[i].Stories = storiesByHL[highlights[i].ID]
			// Use first story image as cover if none set
			if highlights[i].CoverImagePath == "" && len(highlights[i].Stories) > 0 {
				highlights[i].CoverImagePath = highlights[i].Stories[0].ImagePath
			}
		}
	}

	return c.JSON(200, map[string]any{"highlights": highlights})
}

// Store creates a new highlight reel.
// POST /api/highlights
func (ctrl *HighlightController) Store(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	var req struct {
		Title   string `json:"title"    validate:"required,min=1,max=100"`
		StoryID int64  `json:"story_id"`
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	hl := &models.Highlight{
		UserID:    userID,
		Title:     req.Title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.Table("highlights").Insert(hl); err != nil {
		return err
	}

	// Optionally add a first story
	if req.StoryID > 0 {
		_ = database.Raw(
			`INSERT INTO highlight_stories (highlight_id, story_id, added_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			hl.ID, req.StoryID, time.Now(),
		).Exec()

		// Set cover from story
		var s models.Story
		if err := database.Table("stories").Where("id = ?", req.StoryID).First(&s); err == nil {
			hl.CoverImagePath = s.ImagePath
			_ = database.Table("highlights").Where("id = ?", hl.ID).
				Update(database.Map{"cover_image_path": s.ImagePath})
		}
	}

	return c.JSON(201, hl)
}

// Destroy deletes a highlight (owner only).
// DELETE /api/highlights/:id
func (ctrl *HighlightController) Destroy(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	hlID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid highlight id")
	}

	var hl models.Highlight
	if err := database.Table("highlights").Where("id = ?", hlID).First(&hl); err != nil {
		return c.Abort(404, "highlight not found")
	}
	if hl.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	_ = database.Table("highlights").Where("id = ?", hlID).Delete()
	return c.JSON(200, map[string]any{"message": "deleted"})
}

// AddStory adds a story to an existing highlight.
// POST /api/highlights/:id/stories
func (ctrl *HighlightController) AddStory(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	hlID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid highlight id")
	}

	var req struct {
		StoryID int64 `json:"story_id" validate:"required"`
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	var hl models.Highlight
	if err := database.Table("highlights").Where("id = ?", hlID).First(&hl); err != nil {
		return c.Abort(404, "highlight not found")
	}
	if hl.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	_ = database.Raw(
		`INSERT INTO highlight_stories (highlight_id, story_id, added_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		hlID, req.StoryID, time.Now(),
	).Exec()

	// Update cover if none set
	if hl.CoverImagePath == "" {
		var s models.Story
		if err := database.Table("stories").Where("id = ?", req.StoryID).First(&s); err == nil {
			_ = database.Table("highlights").Where("id = ?", hlID).
				Update(database.Map{"cover_image_path": s.ImagePath, "updated_at": time.Now()})
		}
	}

	return c.JSON(200, map[string]any{"message": "added"})
}

// RemoveStory removes a story from a highlight.
// DELETE /api/highlights/:id/stories/:storyId
func (ctrl *HighlightController) RemoveStory(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	hlID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid highlight id")
	}
	storyID, err := strconv.ParseInt(c.Param("storyId"), 10, 64)
	if err != nil {
		return c.Abort(400, "invalid story id")
	}

	var hl models.Highlight
	if err := database.Table("highlights").Where("id = ?", hlID).First(&hl); err != nil {
		return c.Abort(404, "highlight not found")
	}
	if hl.UserID != userID {
		return c.Abort(403, "forbidden")
	}

	_ = database.Raw(
		`DELETE FROM highlight_stories WHERE highlight_id = $1 AND story_id = $2`, hlID, storyID,
	).Exec()

	return c.JSON(200, map[string]any{"message": "removed"})
}

// pgArray converts a []any of int64-like values to a PostgreSQL array string "{1,2,3}".
func pgArray(ids []any) string {
	s := "{"
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += strconv.FormatInt(toInt64(id), 10)
	}
	return s + "}"
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	default:
		return 0
	}
}
