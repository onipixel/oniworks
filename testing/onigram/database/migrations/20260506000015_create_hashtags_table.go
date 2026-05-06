package migrations

import onimig "github.com/onipixel/oniworks/framework/migrations"

type Migration20260506000015 struct{}

func init() {
	onimig.Register("20260506000015_create_hashtags_table", &Migration20260506000015{})
}

func (m *Migration20260506000015) Up(s *onimig.Schema) {
	s.Create("hashtags", func(t *onimig.Table) {
		t.ID()
		t.String("tag", 100).NotNullable()
		t.UniqueIndex("tag")
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_hashtags_tag" ON "hashtags" ("tag")`)
}

func (m *Migration20260506000015) Down(s *onimig.Schema) {
	s.DropIfExists("hashtags")
}
