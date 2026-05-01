package models

import "time"

// Notification types
const (
	NotifLike    = "like"
	NotifComment = "comment"
	NotifFollow  = "follow"
)

// Notification is a realtime event delivered to a user.
type Notification struct {
	ID        int64      `db:"id,primaryKey,autoIncrement" json:"id"`
	UserID    int64      `db:"user_id"    json:"user_id"`
	ActorID   int64      `db:"actor_id"   json:"actor_id"`
	Type      string     `db:"type"       json:"type"`
	PostID    *int64     `db:"post_id"    json:"post_id,omitempty"`
	Read      bool       `db:"read"       json:"read"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at,omitempty"`

	Actor *User `db:"-" json:"actor,omitempty"`
}

func (Notification) TableName() string { return "notifications" }
