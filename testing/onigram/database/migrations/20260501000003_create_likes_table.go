package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000003 struct{}

func init() {
	onimig.Register("20260501000003_create_likes_table", &Migration20260501000003{})
}

func (m *Migration20260501000003) Up(s *onimig.Schema) {
	s.Create("likes", func(t *onimig.Table) {
		t.ID()
		t.ForeignKey("user_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.ForeignKey("post_id", "posts", "id").OnDelete("CASCADE").NotNullable()
		t.Timestamp("created_at").NotNullable()
		t.UniqueIndex("user_id", "post_id")
	})
}

func (m *Migration20260501000003) Down(s *onimig.Schema) {
	s.DropIfExists("likes")
}
