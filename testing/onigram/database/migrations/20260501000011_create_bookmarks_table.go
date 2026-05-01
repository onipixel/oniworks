package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000011 struct{}

func init() {
	onimig.Register("20260501000011_create_bookmarks_table", &Migration20260501000011{})
}

func (m *Migration20260501000011) Up(s *onimig.Schema) {
	s.Create("bookmarks", func(t *onimig.Table) {
		t.ID()
		t.BigInteger("user_id").NotNullable()
		t.BigInteger("post_id").NotNullable()
		t.Timestamp("created_at").NotNullable()
	})
	s.Raw(`CREATE UNIQUE INDEX IF NOT EXISTS "idx_bookmarks_unique" ON "bookmarks" ("user_id", "post_id")`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_bookmarks_user_id" ON "bookmarks" ("user_id")`)
}

func (m *Migration20260501000011) Down(s *onimig.Schema) {
	s.DropIfExists("bookmarks")
}
