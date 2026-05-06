package models

import "time"

// User represents a registered OniGram account.
// Implements auth.User so it can be passed to the JWT guard.
type User struct {
	ID           int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	Username     string    `db:"username"      json:"username"`
	Email        string    `db:"email"         json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Bio          string    `db:"bio"           json:"bio"`
	Website      string    `db:"website"       json:"website"`
	AvatarPath   string    `db:"avatar_path"   json:"avatar_path"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`

	// Computed/loaded fields (not in DB columns)
	FollowerCount  int  `db:"-" json:"follower_count,omitempty"`
	FollowingCount int  `db:"-" json:"following_count,omitempty"`
	PostCount      int  `db:"-" json:"post_count,omitempty"`
	IsFollowing    bool `db:"-" json:"is_following,omitempty"`
}

func (User) TableName() string       { return "users" }
func (u *User) GetID() int64         { return u.ID }
func (u *User) GetEmail() string     { return u.Email }
func (u *User) GetPassword() string  { return u.PasswordHash }
