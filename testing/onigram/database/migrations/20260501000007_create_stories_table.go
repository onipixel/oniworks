package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000007 struct{}

func init() {
	onimig.Register("20260501000007_create_stories_table", &Migration20260501000007{})
}

func (m *Migration20260501000007) Up(s *onimig.Schema) {
	s.Create("stories", func(t *onimig.Table) {
		t.ID()
		t.BigInteger("user_id").NotNullable()
		t.String("image_path", 500).NotNullable()
		t.Timestamp("expires_at").NotNullable()
		t.Timestamp("created_at").NotNullable()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_stories_user_id" ON "stories" ("user_id")`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_stories_expires_at" ON "stories" ("expires_at")`)
}

func (m *Migration20260501000007) Down(s *onimig.Schema) {
	s.DropIfExists("stories")
}
