package models

import "time"

// Like records a user liking a post.
type Like struct {
	ID        int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	UserID    int64     `db:"user_id"    json:"user_id"`
	PostID    int64     `db:"post_id"    json:"post_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (Like) TableName() string { return "likes" }
