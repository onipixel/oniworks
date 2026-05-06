package models

import "time"

// Comment is a text reply on a post, optionally threaded under a parent comment.
type Comment struct {
	ID              int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	UserID          int64     `db:"user_id"           json:"user_id"`
	PostID          int64     `db:"post_id"           json:"post_id"`
	ParentCommentID *int64    `db:"parent_comment_id" json:"parent_comment_id,omitempty"`
	Body            string    `db:"body"              json:"body"`
	IsPinned        bool      `db:"is_pinned"         json:"is_pinned,omitempty"`
	CreatedAt       time.Time `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"        json:"updated_at"`

	User      *User     `db:"-" json:"user,omitempty"`
	LikeCount int       `db:"-" json:"like_count,omitempty"`
	IsLiked   bool      `db:"-" json:"is_liked,omitempty"`
	Replies   []Comment `db:"-" json:"replies,omitempty"`
}

func (Comment) TableName() string { return "comments" }
