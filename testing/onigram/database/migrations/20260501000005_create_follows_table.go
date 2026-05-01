package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000005 struct{}

func init() {
	onimig.Register("20260501000005_create_follows_table", &Migration20260501000005{})
}

func (m *Migration20260501000005) Up(s *onimig.Schema) {
	s.Create("follows", func(t *onimig.Table) {
		t.ID()
		t.ForeignKey("follower_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.ForeignKey("following_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.Timestamp("created_at").NotNullable()
		t.UniqueIndex("follower_id", "following_id")
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_follows_follower" ON "follows" ("follower_id")`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_follows_following" ON "follows" ("following_id")`)
}

func (m *Migration20260501000005) Down(s *onimig.Schema) {
	s.DropIfExists("follows")
}
