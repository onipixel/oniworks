package models

import "time"

type Message struct {
	ID             int64     `db:"id,primaryKey,autoIncrement" json:"id"`
	ConversationID int64     `db:"conversation_id" json:"conversation_id"`
	SenderID       int64     `db:"sender_id"       json:"sender_id"`
	Body           string    `db:"body"            json:"body"`
	Read           bool      `db:"read"            json:"read"`
	CreatedAt      time.Time `db:"created_at"      json:"created_at"`

	// Loaded
	Sender *User `db:"-" json:"sender,omitempty"`
}

func (Message) TableName() string { return "messages" }
