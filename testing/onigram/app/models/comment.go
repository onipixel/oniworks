package models

import "time"

// Comment is a text reply on a post.
type Comment struct {
	ID        int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	UserID    int64     `db:"user_id"    json:"user_id"`
	PostID    int64     `db:"post_id"    json:"post_id"`
	Body      string    `db:"body"       json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	User *User `db:"-" json:"user,omitempty"`
}

func (Comment) TableName() string { return "comments" }
