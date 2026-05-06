package migrations

import onimig "github.com/onipixel/oniworks/framework/migrations"

type Migration20260506000021 struct{}

func init() {
	onimig.Register("20260506000021_add_is_pinned_to_comments", &Migration20260506000021{})
}

func (m *Migration20260506000021) Up(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "comments" ADD COLUMN IF NOT EXISTS "is_pinned" BOOLEAN NOT NULL DEFAULT false`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_comments_is_pinned" ON "comments" ("post_id", "is_pinned") WHERE is_pinned = true`)
}

func (m *Migration20260506000021) Down(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "comments" DROP COLUMN IF EXISTS "is_pinned"`)
}
