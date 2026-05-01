package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000010 struct{}

func init() {
	onimig.Register("20260501000010_create_messages_table", &Migration20260501000010{})
}

func (m *Migration20260501000010) Up(s *onimig.Schema) {
	s.Create("messages", func(t *onimig.Table) {
		t.ID()
		t.BigInteger("conversation_id").NotNullable()
		t.BigInteger("sender_id").NotNullable()
		t.Text("body").NotNullable()
		t.Boolean("read").Default("false")
		t.Timestamp("created_at").NotNullable()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_messages_conversation_id" ON "messages" ("conversation_id", "created_at" DESC)`)
}

func (m *Migration20260501000010) Down(s *onimig.Schema) {
	s.DropIfExists("messages")
}
