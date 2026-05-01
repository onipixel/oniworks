package models

import "time"

// Follow records a follower→following relationship.
type Follow struct {
	ID          int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	FollowerID  int64     `db:"follower_id"  json:"follower_id"`
	FollowingID int64     `db:"following_id" json:"following_id"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
}

func (Follow) TableName() string { return "follows" }
