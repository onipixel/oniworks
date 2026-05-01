package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000012 struct{}

func init() {
	onimig.Register("20260501000012_create_comment_likes_table", &Migration20260501000012{})
}

func (m *Migration20260501000012) Up(s *onimig.Schema) {
	s.Create("comment_likes", func(t *onimig.Table) {
		t.ID()
		t.BigInteger("user_id").NotNullable()
		t.BigInteger("comment_id").NotNullable()
		t.Timestamp("created_at").NotNullable()
	})
	s.Raw(`CREATE UNIQUE INDEX IF NOT EXISTS "idx_comment_likes_unique" ON "comment_likes" ("user_id", "comment_id")`)
}

func (m *Migration20260501000012) Down(s *onimig.Schema) {
	s.DropIfExists("comment_likes")
}
