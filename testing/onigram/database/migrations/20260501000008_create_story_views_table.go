package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000008 struct{}

func init() {
	onimig.Register("20260501000008_create_story_views_table", &Migration20260501000008{})
}

func (m *Migration20260501000008) Up(s *onimig.Schema) {
	s.Create("story_views", func(t *onimig.Table) {
		t.ID()
		t.BigInteger("story_id").NotNullable()
		t.BigInteger("viewer_id").NotNullable()
		t.Timestamp("viewed_at").NotNullable()
	})
	s.Raw(`CREATE UNIQUE INDEX IF NOT EXISTS "idx_story_views_unique" ON "story_views" ("story_id", "viewer_id")`)
}

func (m *Migration20260501000008) Down(s *onimig.Schema) {
	s.DropIfExists("story_views")
}
