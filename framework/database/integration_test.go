package database

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These tests run against a real PostgreSQL instance and are skipped unless
// ONIWORKS_TEST_DSN is set, e.g.
//
//	ONIWORKS_TEST_DSN=postgres://postgres:password@127.0.0.1:5432/oniworks_test
//
// They verify that the M0/M1 query-builder fixes actually execute correctly —
// not just that the generated SQL string looks right.

func openIntegrationDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("ONIWORKS_TEST_DSN")
	if dsn == "" {
		t.Skip("ONIWORKS_TEST_DSN not set; skipping Postgres integration tests")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("bad ONIWORKS_TEST_DSN: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())
	pass, _ := u.User.Password()
	cfg := Config{
		Driver:   DriverPostgres,
		Host:     u.Hostname(),
		Port:     port,
		User:     u.User.Username(),
		Password: pass,
		Name:     strings.TrimPrefix(u.Path, "/"),
		SSLMode:  "disable",
		MaxOpen:  5,
		MaxIdle:  2,
	}
	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *DB, sql string) {
	t.Helper()
	if _, err := db.SQLDB().Exec(sql); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

type intRow struct {
	ID      int64     `db:"id,primaryKey,autoIncrement"`
	Name    string    `db:"name"`
	Age     int       `db:"age"`
	Balance float64   `db:"balance"`
	Active  bool      `db:"active"`
	Created time.Time `db:"created"`
}

// TestIntegrationScannerTypes proves the convertAssign fix: NUMERIC, BOOLEAN and
// TIMESTAMPTZ columns round-trip into Go int/float/bool/time without silent loss.
func TestIntegrationScannerTypes(t *testing.T) {
	db := openIntegrationDB(t)
	mustExec(t, db, `DROP TABLE IF EXISTS int_rows`)
	mustExec(t, db, `CREATE TABLE int_rows (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		age INTEGER NOT NULL,
		balance NUMERIC(10,2) NOT NULL,
		active BOOLEAN NOT NULL,
		created TIMESTAMPTZ NOT NULL
	)`)
	mustExec(t, db, `INSERT INTO int_rows (name, age, balance, active, created)
		VALUES ('alice', 30, 1234.56, true, '2026-06-29 12:30:00+00')`)

	var row intRow
	if err := db.Table("int_rows").Where("name = ?", "alice").First(&row); err != nil {
		t.Fatalf("First: %v", err)
	}
	if row.Age != 30 {
		t.Errorf("age = %d, want 30", row.Age)
	}
	if row.Balance != 1234.56 {
		t.Errorf("balance = %v, want 1234.56 (NUMERIC must not be silently dropped)", row.Balance)
	}
	if !row.Active {
		t.Errorf("active = false, want true")
	}
	if row.Created.Year() != 2026 || row.Created.Month() != time.June {
		t.Errorf("created = %v, want 2026-06", row.Created)
	}
}

// TestIntegrationOrderByAndSelect proves the injection-hardened Select/OrderBy
// still execute and return correctly ordered, projected results.
func TestIntegrationOrderByAndSelect(t *testing.T) {
	db := openIntegrationDB(t)
	mustExec(t, db, `DROP TABLE IF EXISTS ord`)
	mustExec(t, db, `CREATE TABLE ord (id BIGSERIAL PRIMARY KEY, name TEXT, score INT)`)
	mustExec(t, db, `INSERT INTO ord (name, score) VALUES ('a', 10), ('b', 30), ('c', 20)`)

	type r struct {
		Name  string `db:"name"`
		Score int    `db:"score"`
	}
	var rows []r
	if err := db.Table("ord").Select("name", "score").OrderBy("score DESC").All(&rows); err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(rows) != 3 || rows[0].Name != "b" || rows[2].Name != "a" {
		t.Fatalf("ordering wrong: %+v", rows)
	}

	// An injection payload must error out, not execute.
	var bad []r
	err := db.Table("ord").OrderBy("score; DROP TABLE ord").All(&bad)
	if err == nil {
		t.Fatal("expected injection payload to be rejected")
	}
	// Confirm the table still exists (the DROP never ran).
	var cnt int64
	if err := db.Table("ord").Scan(&cnt); err == nil {
		// Scan of COUNT-less query won't work; just re-query existence.
	}
	var probe []r
	if err := db.Table("ord").Select("name").All(&probe); err != nil {
		t.Fatalf("table should still exist after rejected injection: %v", err)
	}
}

type elMsg struct {
	ID         int64  `db:"id,primaryKey,autoIncrement"`
	SenderID   int64  `db:"sender_id"`
	ReceiverID int64  `db:"receiver_id"`
	Body       string `db:"body"`
}

type elUser struct {
	ID   int64 `db:"id,primaryKey,autoIncrement"`
	Name string `db:"name"`
	// Two has_many relations to the SAME table — the field-collision regression.
	Sent     []elMsg `db:"has_many:el_msgs,foreign_key:sender_id"`
	Received []elMsg `db:"has_many:el_msgs,foreign_key:receiver_id"`
}

// TestIntegrationEagerLoadCollision proves end-to-end that two has_many
// relations to the same table load into their own fields via the full With()
// path (not just the unit-level assignHasMany).
func TestIntegrationEagerLoadCollision(t *testing.T) {
	db := openIntegrationDB(t)
	mustExec(t, db, `DROP TABLE IF EXISTS el_msgs`)
	mustExec(t, db, `DROP TABLE IF EXISTS el_users`)
	mustExec(t, db, `CREATE TABLE el_users (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`)
	mustExec(t, db, `CREATE TABLE el_msgs (
		id BIGSERIAL PRIMARY KEY, sender_id BIGINT, receiver_id BIGINT, body TEXT
	)`)
	mustExec(t, db, `INSERT INTO el_users (name) VALUES ('u1'), ('u2')`)
	// u1 sends 2, receives 1.
	mustExec(t, db, `INSERT INTO el_msgs (sender_id, receiver_id, body) VALUES
		(1, 2, 'a'), (1, 2, 'b'), (2, 1, 'c')`)

	var users []elUser
	if err := db.Table("el_users").With("Sent", "Received").OrderBy("id ASC").All(&users); err != nil {
		t.Fatalf("eager load: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
	u1 := users[0]
	if len(u1.Sent) != 2 {
		t.Errorf("u1.Sent = %d, want 2", len(u1.Sent))
	}
	if len(u1.Received) != 1 {
		t.Errorf("u1.Received = %d, want 1", len(u1.Received))
	}
	// Critical: the two relations must not bleed into each other.
	for _, m := range u1.Sent {
		if m.SenderID != 1 {
			t.Errorf("Sent contained a non-sent message: %+v", m)
		}
	}
	for _, m := range u1.Received {
		if m.ReceiverID != 1 {
			t.Errorf("Received contained a non-received message: %+v", m)
		}
	}
}

type sdPost struct {
	ID        int64      `db:"id,primaryKey,autoIncrement"`
	Title     string     `db:"title"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// TestIntegrationSoftDeleteAndPaginate proves the soft-delete write path and the
// Paginate COUNT fix: Total reflects only non-trashed rows.
func TestIntegrationSoftDeleteAndPaginate(t *testing.T) {
	db := openIntegrationDB(t)
	mustExec(t, db, `DROP TABLE IF EXISTS sd_posts`)
	mustExec(t, db, `CREATE TABLE sd_posts (
		id BIGSERIAL PRIMARY KEY, title TEXT NOT NULL, deleted_at TIMESTAMPTZ
	)`)
	for i := 0; i < 5; i++ {
		mustExec(t, db, `INSERT INTO sd_posts (title) VALUES ('p`+strconv.Itoa(i)+`')`)
	}

	// Soft-delete two rows via the builder.
	if err := db.Table("sd_posts").SoftDelete().Where("id <= ?", 2).Delete(); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// Active count should be 3.
	active, err := db.Table("sd_posts").SoftDelete().Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if active != 3 {
		t.Fatalf("active count = %d, want 3", active)
	}

	// WithTrashed count should be 5.
	all, err := db.Table("sd_posts").SoftDelete().WithTrashed().Count()
	if err != nil {
		t.Fatalf("count trashed: %v", err)
	}
	if all != 5 {
		t.Fatalf("trashed count = %d, want 5", all)
	}

	// Paginate Total must match the active (soft-deleted-aware) count.
	var page []sdPost
	p, err := db.Table("sd_posts").SoftDelete().OrderBy("id ASC").Paginate(1, 2, &page)
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if p.Total != 3 {
		t.Fatalf("paginate Total = %d, want 3 (must exclude soft-deleted)", p.Total)
	}
	if len(p.Items) != 2 {
		t.Fatalf("page Items = %d, want 2 (Items must be populated)", len(p.Items))
	}

	// ForceDelete removes a row for real.
	if err := db.Table("sd_posts").SoftDelete().Where("id = ?", 3).ForceDelete(); err != nil {
		t.Fatalf("force delete: %v", err)
	}
	gone, _ := db.Table("sd_posts").SoftDelete().WithTrashed().Count()
	if gone != 4 {
		t.Fatalf("after force delete total = %d, want 4", gone)
	}
}
