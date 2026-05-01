package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000004 struct{}

func init() {
	onimig.Register("20260501000004_create_comments_table", &Migration20260501000004{})
}

func (m *Migration20260501000004) Up(s *onimig.Schema) {
	s.Create("comments", func(t *onimig.Table) {
		t.ID()
		t.ForeignKey("user_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.ForeignKey("post_id", "posts", "id").OnDelete("CASCADE").NotNullable()
		t.Text("body").NotNullable()
		t.Timestamps()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_comments_post_id" ON "comments" ("post_id")`)
}

func (m *Migration20260501000004) Down(s *onimig.Schema) {
	s.DropIfExists("comments")
}
