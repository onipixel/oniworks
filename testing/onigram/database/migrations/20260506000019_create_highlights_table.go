package migrations

import onimig "github.com/onipixel/oniworks/framework/migrations"

type Migration20260506000019 struct{}

func init() {
	onimig.Register("20260506000019_create_highlights_table", &Migration20260506000019{})
}

func (m *Migration20260506000019) Up(s *onimig.Schema) {
	s.Create("highlights", func(t *onimig.Table) {
		t.ID()
		t.ForeignKey("user_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.String("title", 100).NotNullable()
		t.String("cover_image_path", 500).Nullable()
		t.Timestamps()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_highlights_user_id" ON "highlights" ("user_id")`)
}

func (m *Migration20260506000019) Down(s *onimig.Schema) {
	s.DropIfExists("highlights")
}
