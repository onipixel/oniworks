package migrations

import onimig "github.com/onipixel/oniworks/framework/migrations"

type Migration20260506000017 struct{}

func init() {
	onimig.Register("20260506000017_add_parent_comment_id_to_comments", &Migration20260506000017{})
}

func (m *Migration20260506000017) Up(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "comments" ADD COLUMN IF NOT EXISTS "parent_comment_id" BIGINT REFERENCES comments(id) ON DELETE CASCADE`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_comments_parent_id" ON "comments" ("parent_comment_id")`)
}

func (m *Migration20260506000017) Down(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "comments" DROP COLUMN IF EXISTS "parent_comment_id"`)
}
