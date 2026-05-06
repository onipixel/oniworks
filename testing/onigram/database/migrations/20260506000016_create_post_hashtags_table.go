package migrations

import onimig "github.com/onipixel/oniworks/framework/migrations"

type Migration20260506000016 struct{}

func init() {
	onimig.Register("20260506000016_create_post_hashtags_table", &Migration20260506000016{})
}

func (m *Migration20260506000016) Up(s *onimig.Schema) {
	s.Create("post_hashtags", func(t *onimig.Table) {
		t.ForeignKey("post_id", "posts", "id").OnDelete("CASCADE").NotNullable()
		t.ForeignKey("hashtag_id", "hashtags", "id").OnDelete("CASCADE").NotNullable()
	})
	s.Raw(`ALTER TABLE "post_hashtags" ADD PRIMARY KEY ("post_id", "hashtag_id")`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_post_hashtags_hashtag_id" ON "post_hashtags" ("hashtag_id")`)
}

func (m *Migration20260506000016) Down(s *onimig.Schema) {
	s.DropIfExists("post_hashtags")
}
