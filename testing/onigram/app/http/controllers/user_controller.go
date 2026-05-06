package controllers

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// UserController handles user profile, follow, and search operations.
type UserController struct{}

// Show returns a public user profile by username.
// GET /api/users/:username
func (ctrl *UserController) Show(c *onihttp.Context) error {
	username := c.Param("username")
	viewerID, _ := c.Get("user_id")
	vid, _ := viewerID.(int64)

	var user models.User
	if err := database.Table("users").Where("username = ?", username).First(&user); err != nil {
		if err == database.ErrNotFound {
			return c.Abort(404, "user not found")
		}
		return err
	}

	// Follower / following / post counts
	followerCount, _ := database.Table("follows").Where("following_id = ?", user.ID).Count()
	followingCount, _ := database.Table("follows").Where("follower_id = ?", user.ID).Count()
	postCount, _ := database.Table("posts").Where("user_id = ?", user.ID).Count()
	user.FollowerCount = int(followerCount)
	user.FollowingCount = int(followingCount)
	user.PostCount = int(postCount)

	// Is the viewer following this user?
	if vid != 0 && vid != user.ID {
		isFollowing, _ := database.Table("follows").
			Where("follower_id = ? AND following_id = ?", vid, user.ID).
			Exists()
		user.IsFollowing = isFollowing
	}

	return c.JSON(200, user)
}

// Update modifies the authenticated user's profile.
// PUT /api/users/me
func (ctrl *UserController) Update(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	var req struct {
		Username string `json:"username"`
		Bio      string `json:"bio"`
		Website  string `json:"website"`
	}
	if err := c.Bind(&req); err != nil {
		return c.Abort(400, "invalid request body")
	}

	updates := database.Map{"updated_at": time.Now()}
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.Bio != "" {
		updates["bio"] = req.Bio
	}
	updates["website"] = req.Website

	if err := database.Table("users").Where("id = ?", userID).Update(updates); err != nil {
		return err
	}

	var user models.User
	_ = database.Table("users").Where("id = ?", userID).First(&user)
	return c.JSON(200, user)
}

// UpdateAvatar uploads and sets the user's avatar image.
// POST /api/users/me/avatar
func (ctrl *UserController) UpdateAvatar(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	uf, err := c.ParseUpload("avatar", onihttp.UploadConfig{
		MaxSize:      5 << 20, // 5 MB
		AllowedTypes: []string{"image/jpeg", "image/png", "image/webp"},
	})
	if err != nil {
		return c.Abort(422, "invalid image: "+err.Error())
	}

	filename := fmt.Sprintf("avatar_%d%s", userID, uf.Ext())
	dest := "storage/avatars"
	savedPath, err := uf.Store(dest, filename)
	if err != nil {
		return err
	}

	// Normalise path to URL-friendly /storage/avatars/...
	urlPath := "/" + strings.ReplaceAll(filepath.ToSlash(savedPath), "\\", "/")

	if err := database.Table("users").Where("id = ?", userID).
		Update(database.Map{"avatar_path": urlPath, "updated_at": time.Now()}); err != nil {
		return err
	}

	return c.JSON(200, map[string]any{"avatar_path": urlPath})
}

// Follow makes the authenticated user follow :username.
// POST /api/users/:username/follow
func (ctrl *UserController) Follow(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	followerID, _ := uid.(int64)
	username := c.Param("username")

	var target models.User
	if err := database.Table("users").Where("username = ?", username).First(&target); err != nil {
		return c.Abort(404, "user not found")
	}
	if target.ID == followerID {
		return c.Abort(422, "cannot follow yourself")
	}

	already, _ := database.Table("follows").
		Where("follower_id = ? AND following_id = ?", followerID, target.ID).Exists()
	if already {
		return c.JSON(200, map[string]any{"message": "already following"})
	}

	follow := &models.Follow{
		FollowerID:  followerID,
		FollowingID: target.ID,
		CreatedAt:   time.Now(),
	}
	if err := database.Table("follows").Insert(follow); err != nil {
		return err
	}

	// Create notification for the followed user
	notif := &models.Notification{
		UserID:    target.ID,
		ActorID:   followerID,
		Type:      models.NotifFollow,
		Read:      false,
		CreatedAt: time.Now(),
	}
	_ = database.Table("notifications").Insert(notif)

	return c.JSON(201, map[string]any{"message": "following"})
}

// Unfollow removes the follow relationship.
// DELETE /api/users/:username/follow
func (ctrl *UserController) Unfollow(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	followerID, _ := uid.(int64)
	username := c.Param("username")

	var target models.User
	if err := database.Table("users").Where("username = ?", username).First(&target); err != nil {
		return c.Abort(404, "user not found")
	}

	if err := database.Table("follows").
		Where("follower_id = ? AND following_id = ?", followerID, target.ID).Delete(); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"message": "unfollowed"})
}

// Search finds users matching a query string.
// Suggestions returns users the current user is not yet following.
// GET /api/users/suggestions
func (ctrl *UserController) Suggestions(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	var users []models.User
	err := database.Raw(`
		SELECT id, username, COALESCE(bio,'') AS bio, COALESCE(avatar_path,'') AS avatar_path FROM users
		WHERE id != $1
		AND id NOT IN (SELECT following_id FROM follows WHERE follower_id = $1)
		ORDER BY RANDOM() LIMIT 6`, userID).All(&users)
	if err != nil {
		return err
	}
	if users == nil {
		users = []models.User{}
	}
	return c.JSON(200, map[string]any{"users": users})
}

// GET /api/users/search?q=...
func (ctrl *UserController) Search(c *onihttp.Context) error {
	q := c.Query("q")
	if len(q) < 2 {
		return c.JSON(200, map[string]any{"users": []any{}})
	}
	pattern := "%" + q + "%"
	var users []models.User
	if err := database.Table("users").
		Select("id", "username", "bio", "avatar_path").
		Where("username ILIKE ? OR bio ILIKE ?", pattern, pattern).
		Limit(20).All(&users); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"users": users})
}

// Followers returns the list of users following :username.
// GET /api/users/:username/followers
func (ctrl *UserController) Followers(c *onihttp.Context) error {
	username := c.Param("username")
	var target models.User
	if err := database.Table("users").Where("username = ?", username).First(&target); err != nil {
		return c.Abort(404, "user not found")
	}
	var users []models.User
	if err := database.Table("users").
		Select("users.id", "users.username", "users.bio", "users.avatar_path").
		Join("follows ON follows.follower_id = users.id").
		Where("follows.following_id = ?", target.ID).
		Limit(50).All(&users); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"users": users})
}

// Following returns the list of users that :username follows.
// GET /api/users/:username/following
func (ctrl *UserController) Following(c *onihttp.Context) error {
	username := c.Param("username")
	var target models.User
	if err := database.Table("users").Where("username = ?", username).First(&target); err != nil {
		return c.Abort(404, "user not found")
	}
	var users []models.User
	if err := database.Table("users").
		Select("users.id", "users.username", "users.bio", "users.avatar_path").
		Join("follows ON follows.following_id = users.id").
		Where("follows.follower_id = ?", target.ID).
		Limit(50).All(&users); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"users": users})
}
