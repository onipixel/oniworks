package models

// PostImage is one photo in a multi-image carousel post.
type PostImage struct {
	ID        int64  `db:"id,primaryKey,autoIncrement" json:"id"`
	PostID    int64  `db:"post_id"    json:"post_id"`
	ImagePath string `db:"image_path" json:"image_path"`
	Position  int    `db:"position"   json:"position"`
}

func (PostImage) TableName() string { return "post_images" }
