package controllers

import (
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/auth"
	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

// AuthController handles registration, login, and logout.
type AuthController struct {
	Guard *auth.Guard
}

// Register creates a new user account.
// POST /api/auth/register
func (ctrl *AuthController) Register(c *onihttp.Context) error {
	var req struct {
		Username string `json:"username" validate:"required,min=3,max=30,alphanum"`
		Email    string `json:"email"    validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	// Check duplicate username / email
	exists, err := database.Table("users").
		Where("username = ? OR email = ?", req.Username, req.Email).
		Exists()
	if err != nil {
		return err
	}
	if exists {
		return c.Abort(409, "username or email already taken")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := database.Table("users").Insert(user); err != nil {
		return err
	}

	token, err := ctrl.Guard.IssueToken(user, 7*24*time.Hour)
	if err != nil {
		return err
	}

	return c.JSON(201, map[string]any{
		"token": token,
		"user":  user,
	})
}

// Login authenticates a user and returns a JWT.
// POST /api/auth/login
func (ctrl *AuthController) Login(c *onihttp.Context) error {
	var req struct {
		Email    string `json:"email"    validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	var user models.User
	if err := database.Table("users").Where("email = ?", req.Email).First(&user); err != nil {
		if err == database.ErrNotFound {
			return c.Abort(401, "invalid email or password")
		}
		return err
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		return c.Abort(401, "invalid email or password")
	}

	token, err := ctrl.Guard.IssueToken(&user, 7*24*time.Hour)
	if err != nil {
		return err
	}

	return c.JSON(200, map[string]any{
		"token": token,
		"user":  user,
	})
}

// Me returns the currently authenticated user.
// GET /api/auth/me  (requires Auth middleware)
func (ctrl *AuthController) Me(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	var user models.User
	if err := database.Table("users").Where("id = ?", userID).First(&user); err != nil {
		return c.Abort(404, "user not found")
	}
	return c.JSON(200, user)
}

// Logout is a no-op for stateless JWT (client just discards the token).
// POST /api/auth/logout
func (ctrl *AuthController) Logout(c *onihttp.Context) error {
	return c.JSON(200, map[string]any{"message": "logged out"})
}
