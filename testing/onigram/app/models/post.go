package models

import "time"

// Post is a single photo post on OniGram.
type Post struct {
	ID        int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	UserID    int64     `db:"user_id"    json:"user_id"`
	ImagePath string    `db:"image_path" json:"image_path"`
	Caption   string    `db:"caption"    json:"caption"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// Loaded / computed
	User         *User       `db:"-" json:"user,omitempty"`
	LikeCount    int         `db:"-" json:"like_count,omitempty"`
	CommentCount int         `db:"-" json:"comment_count,omitempty"`
	IsLiked      bool        `db:"-" json:"is_liked,omitempty"`
	IsBookmarked bool        `db:"-" json:"is_bookmarked,omitempty"`
	Images       []PostImage `db:"-" json:"images,omitempty"`
}

func (Post) TableName() string { return "posts" }
