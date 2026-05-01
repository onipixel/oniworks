package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000002 struct{}

func init() {
	onimig.Register("20260501000002_create_posts_table", &Migration20260501000002{})
}

func (m *Migration20260501000002) Up(s *onimig.Schema) {
	s.Create("posts", func(t *onimig.Table) {
		t.ID()
		t.ForeignKey("user_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.String("image_path", 500).NotNullable()
		t.Text("caption").Nullable()
		t.Timestamps()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_posts_user_id" ON "posts" ("user_id")`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_posts_created_at" ON "posts" ("created_at" DESC)`)
}

func (m *Migration20260501000002) Down(s *onimig.Schema) {
	s.DropIfExists("posts")
}
