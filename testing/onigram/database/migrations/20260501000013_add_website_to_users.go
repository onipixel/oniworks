package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000013 struct{}

func init() {
	onimig.Register("20260501000013_add_website_to_users", &Migration20260501000013{})
}

func (m *Migration20260501000013) Up(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "users" ADD COLUMN IF NOT EXISTS "website" VARCHAR(500)`)
}

func (m *Migration20260501000013) Down(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "users" DROP COLUMN IF EXISTS "website"`)
}
