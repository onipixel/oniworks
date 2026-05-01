package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000006 struct{}

func init() {
	onimig.Register("20260501000006_create_notifications_table", &Migration20260501000006{})
}

func (m *Migration20260501000006) Up(s *onimig.Schema) {
	s.Create("notifications", func(t *onimig.Table) {
		t.ID()
		t.ForeignKey("user_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.ForeignKey("actor_id", "users", "id").OnDelete("CASCADE").NotNullable()
		t.String("type", 50).NotNullable()
		t.BigInteger("post_id").Nullable()
		t.Boolean("read").Default(false).NotNullable()
		t.Timestamp("created_at").NotNullable()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_notifications_user_id" ON "notifications" ("user_id", "read")`)
}

func (m *Migration20260501000006) Down(s *onimig.Schema) {
	s.DropIfExists("notifications")
}
