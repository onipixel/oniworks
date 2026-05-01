package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000014 struct{}

func init() {
	onimig.Register("20260501000014_add_updated_at_to_notifications", &Migration20260501000014{})
}

func (m *Migration20260501000014) Up(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "notifications" ADD COLUMN IF NOT EXISTS "updated_at" TIMESTAMPTZ`)
}

func (m *Migration20260501000014) Down(s *onimig.Schema) {
	s.Raw(`ALTER TABLE "notifications" DROP COLUMN IF EXISTS "updated_at"`)
}
