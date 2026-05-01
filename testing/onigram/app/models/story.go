package models

import "time"

type Story struct {
	ID        int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	UserID    int64     `db:"user_id"    json:"user_id"`
	ImagePath string    `db:"image_path" json:"image_path"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`

	// Loaded
	User   *User `db:"-" json:"user,omitempty"`
	Viewed bool  `db:"-" json:"viewed"`
}

func (Story) TableName() string { return "stories" }
