package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000009 struct{}

func init() {
	onimig.Register("20260501000009_create_conversations_table", &Migration20260501000009{})
}

func (m *Migration20260501000009) Up(s *onimig.Schema) {
	s.Create("conversations", func(t *onimig.Table) {
		t.ID()
		t.BigInteger("user1_id").NotNullable()
		t.BigInteger("user2_id").NotNullable()
		t.Timestamp("last_message_at").Nullable()
		t.Timestamp("created_at").NotNullable()
	})
	s.Raw(`CREATE UNIQUE INDEX IF NOT EXISTS "idx_conversations_pair" ON "conversations" ("user1_id", "user2_id")`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_conversations_user1" ON "conversations" ("user1_id")`)
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_conversations_user2" ON "conversations" ("user2_id")`)
}

func (m *Migration20260501000009) Down(s *onimig.Schema) {
	s.DropIfExists("conversations")
}
