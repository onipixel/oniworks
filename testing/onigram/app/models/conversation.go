package models

import "time"

type Conversation struct {
	ID            int64      `db:"id,primaryKey,autoIncrement" json:"id"`
	User1ID       int64      `db:"user1_id"        json:"user1_id"`
	User2ID       int64      `db:"user2_id"        json:"user2_id"`
	LastMessageAt *time.Time `db:"last_message_at" json:"last_message_at"`
	CreatedAt     time.Time  `db:"created_at"      json:"created_at"`

	// Loaded
	OtherUser   *User    `db:"-" json:"other_user,omitempty"`
	LastMessage *Message `db:"-" json:"last_message,omitempty"`
	UnreadCount int      `db:"-" json:"unread_count"`
}

func (Conversation) TableName() string { return "conversations" }
