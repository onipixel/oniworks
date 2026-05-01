package migrations

import (
	onimig "github.com/onipixel/oniworks/framework/migrations"
)

type Migration20260501000001 struct{}

func init() {
	onimig.Register("20260501000001_create_users_table", &Migration20260501000001{})
}

func (m *Migration20260501000001) Up(s *onimig.Schema) {
	s.Create("users", func(t *onimig.Table) {
		t.ID()
		t.String("username", 50).Unique().NotNullable()
		t.String("email", 255).Unique().NotNullable()
		t.String("password_hash", 255).NotNullable()
		t.Text("bio").Nullable()
		t.String("avatar_path", 500).Nullable()
		t.Timestamps()
	})
	s.Raw(`CREATE INDEX IF NOT EXISTS "idx_users_username" ON "users" ("username")`)
}

func (m *Migration20260501000001) Down(s *onimig.Schema) {
	s.DropIfExists("users")
}
