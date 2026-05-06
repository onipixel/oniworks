package models

// Hashtag is a unique tag extracted from post captions.
type Hashtag struct {
	ID  int64  `db:"id,primaryKey,autoIncrement" json:"id"`
	Tag string `db:"tag" json:"tag"`
}

func (Hashtag) TableName() string { return "hashtags" }
