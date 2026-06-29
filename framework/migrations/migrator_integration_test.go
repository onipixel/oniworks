package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Gated behind ONIWORKS_TEST_DSN, e.g.
//
//	ONIWORKS_TEST_DSN=postgres://postgres:password@127.0.0.1:5432/oniworks_test

func openMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("ONIWORKS_TEST_DSN")
	if dsn == "" {
		t.Skip("ONIWORKS_TEST_DSN not set; skipping migration integration tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type goodMig struct{ table string }

func (g goodMig) Up(s *Schema) {
	s.Create(g.table, func(tb *Table) { tb.ID(); tb.String("name", 100) })
}
func (g goodMig) Down(s *Schema) { s.DropIfExists(g.table) }

type badMig struct{}

func (badMig) Up(s *Schema)   { s.Raw("CREATE TABLE this is not valid sql") }
func (badMig) Down(s *Schema) {}

// TestMigrateBatchRollsBackOnFailure is the batch-transactionality regression:
// if a later migration in a batch fails, earlier migrations in the SAME batch
// must be rolled back, leaving no tables and no migration records.
func TestMigrateBatchRollsBackOnFailure(t *testing.T) {
	db := openMigrationDB(t)
	ctx := context.Background()

	// Clean slate.
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS mig_a, mig_b, oni_migrations`)

	m := New(db, "postgres")
	m.Register("0001_create_mig_a", goodMig{table: "mig_a"})
	m.Register("0002_bad", badMig{})

	if err := m.Migrate(ctx); err == nil {
		t.Fatal("expected the batch to fail on the bad migration")
	}

	// mig_a must NOT exist — the whole batch rolled back.
	var exists bool
	_ = db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='mig_a')`).Scan(&exists)
	if exists {
		t.Fatal("mig_a should have been rolled back with the failed batch")
	}

	// No migration should have been recorded.
	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM oni_migrations`).Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 recorded migrations after rollback, got %d", count)
	}
}

// TestMigrateBatchCommitsOnSuccess verifies a clean batch commits all of it.
func TestMigrateBatchCommitsOnSuccess(t *testing.T) {
	db := openMigrationDB(t)
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS mig_a, mig_b, oni_migrations`)

	m := New(db, "postgres")
	m.Register("0001_create_mig_a", goodMig{table: "mig_a"})
	m.Register("0002_create_mig_b", goodMig{table: "mig_b"})

	if err := m.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, tbl := range []string{"mig_a", "mig_b"} {
		var exists bool
		_ = db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=$1)`, tbl).Scan(&exists)
		if !exists {
			t.Fatalf("table %s should exist after successful migrate", tbl)
		}
	}
	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM oni_migrations`).Scan(&count)
	if count != 2 {
		t.Fatalf("expected 2 recorded migrations, got %d", count)
	}
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS mig_a, mig_b, oni_migrations`)
}
