package migrations

import onimig "github.com/onipixel/oniworks/framework/migrations"

type Migration20260506000020 struct{}

func init() {
	onimig.Register("20260506000020_create_highlight_stories_table", &Migration20260506000020{})
}

func (m *Migration20260506000020) Up(s *onimig.Schema) {
	s.Create("highlight_stories", func(t *onimig.Table) {
		t.ForeignKey("highlight_id", "highlights", "id").OnDelete("CASCADE").NotNullable()
		t.ForeignKey("story_id", "stories", "id").OnDelete("CASCADE").NotNullable()
		t.Timestamp("added_at").NotNullable()
	})
	s.Raw(`ALTER TABLE "highlight_stories" ADD PRIMARY KEY ("highlight_id", "story_id")`)
}

func (m *Migration20260506000020) Down(s *onimig.Schema) {
	s.DropIfExists("highlight_stories")
}
