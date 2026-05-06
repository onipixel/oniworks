package migrations

import onimig "github.com/onipixel/oniworks/framework/migrations"

type Migration20260506000018 struct{}

func init() {
	onimig.Register("20260506000018_create_post_images_table", &Migration20260506000018{})
}

func (m *Migration20260506000018) Up(s *onimig.Schema) {
	s.Create("post_images", func(t *onimig.Table) {
		t.ID()
		t.ForeignKey("post_id", "posts", "id").OnDelete("CASCADE").NotNullable()
		t.String("image_path", 500).NotNullable()
		t.Integer("position").NotNullable()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_post_images_post_id" ON "post_images" ("post_id", "position")`)
}

func (m *Migration20260506000018) Down(s *onimig.Schema) {
	s.DropIfExists("post_images")
}
