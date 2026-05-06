package models

import "time"

// Highlight is a named collection of pinned stories on a user's profile.
type Highlight struct {
	ID             int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	UserID         int64     `db:"user_id"          json:"user_id"`
	Title          string    `db:"title"            json:"title"`
	CoverImagePath string    `db:"cover_image_path" json:"cover_image_path"`
	CreatedAt      time.Time `db:"created_at"       json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"       json:"updated_at"`

	Stories []Story `db:"-" json:"stories,omitempty"`
}

func (Highlight) TableName() string { return "highlights" }
