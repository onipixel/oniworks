package database

import (
	"strings"
	"testing"
)

// newTestBuilder returns a Builder bound to a connection-less DB with the given
// dialect grammar, so buildSelect can be exercised without a real database.
func newTestBuilder(driver Driver, table string) *Builder {
	var g Grammar
	if driver == DriverMySQL {
		g = &mysqlGrammar{}
	} else {
		g = &postgresGrammar{}
	}
	db := &DB{driver: driver, grammar: g}
	b := &Builder{db: db, table: table}
	return b
}

// TestSelectQuotesIdentifiers verifies legitimate column references survive and
// are quoted, preserving the documented API surface.
func TestSelectQuotesIdentifiers(t *testing.T) {
	cases := []struct {
		name string
		cols []string
		want string // substring expected in the SELECT list
	}{
		{"single", []string{"id"}, `"id"`},
		{"multi", []string{"id", "email"}, `"id", "email"`},
		{"qualified", []string{"users.id"}, `"users"."id"`},
		{"alias", []string{"id AS user_id"}, `"id" AS "user_id"`},
		{"implicit_alias", []string{"id user_id"}, `"id" AS "user_id"`},
		{"star", []string{"*"}, `*`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newTestBuilder(DriverPostgres, "users")
			b.Select(tc.cols...)
			if b.err != nil {
				t.Fatalf("unexpected builder error: %v", b.err)
			}
			q, _ := b.buildSelect()
			if !strings.Contains(q, tc.want) {
				t.Fatalf("got %q, want it to contain %q", q, tc.want)
			}
		})
	}
}

// TestOrderByPreservesRealUsage verifies every ORDER BY pattern used by the
// bundled apps still works and is quoted.
func TestOrderByPreservesRealUsage(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"created_at DESC", `ORDER BY "created_at" DESC`},
		{"key ASC", `ORDER BY "key" ASC`},
		{"bookmarks.created_at DESC", `ORDER BY "bookmarks"."created_at" DESC`},
		{"is_pinned DESC, created_at ASC", `ORDER BY "is_pinned" DESC, "created_at" ASC`},
		{"last_message_at DESC NULLS LAST", `ORDER BY "last_message_at" DESC NULLS LAST`},
		{"post_id, position ASC", `ORDER BY "post_id", "position" ASC`},
		{"id ASC", `ORDER BY "id" ASC`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			b := newTestBuilder(DriverPostgres, "t")
			b.OrderBy(tc.in)
			if b.err != nil {
				t.Fatalf("unexpected builder error for %q: %v", tc.in, b.err)
			}
			q, _ := b.buildSelect()
			if !strings.Contains(q, tc.want) {
				t.Fatalf("got %q, want it to contain %q", q, tc.want)
			}
		})
	}
}

// TestGroupByQuotes verifies GROUP BY columns are quoted.
func TestGroupByQuotes(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "comments")
	b.GroupBy("comment_id")
	if b.err != nil {
		t.Fatalf("unexpected error: %v", b.err)
	}
	q, _ := b.buildSelect()
	if !strings.Contains(q, `GROUP BY "comment_id"`) {
		t.Fatalf("got %q, want GROUP BY \"comment_id\"", q)
	}
}

// TestInjectionAttemptsRejected is the core security regression: malicious
// identifier input must set the builder error and must NOT appear unescaped in
// the generated SQL.
func TestInjectionAttemptsRejected(t *testing.T) {
	payloads := []struct {
		method string
		apply  func(*Builder)
	}{
		{"OrderBy;DROP", func(b *Builder) { b.OrderBy("id; DROP TABLE users") }},
		{"OrderBy_subquery", func(b *Builder) { b.OrderBy("(SELECT password FROM users)") }},
		{"OrderBy_caseStmt", func(b *Builder) { b.OrderBy("CASE WHEN 1=1 THEN id ELSE name END") }},
		{"OrderBy_badDir", func(b *Builder) { b.OrderBy("id; --") }},
		{"OrderBy_unionish", func(b *Builder) { b.OrderBy("1 UNION SELECT 1") }},
		{"Select_func", func(b *Builder) { b.Select("(SELECT secret FROM vault)") }},
		{"Select_comment", func(b *Builder) { b.Select("id; DROP TABLE users --") }},
		{"GroupBy_inject", func(b *Builder) { b.GroupBy("id); DELETE FROM users; --") }},
	}
	for _, p := range payloads {
		t.Run(p.method, func(t *testing.T) {
			b := newTestBuilder(DriverPostgres, "users")
			p.apply(b)
			if b.err == nil {
				t.Fatalf("expected builder error for injection payload, got none")
			}
			// The terminal method must surface the error rather than run a query.
			var dest []map[string]any
			if err := b.All(&dest); err == nil {
				t.Fatalf("expected All() to return the builder error, got nil")
			}
		})
	}
}

// TestInjectionNeverReachesSQL ensures that even if a payload were somehow
// appended, the dangerous metacharacters never appear raw in buildSelect output
// for the rejected cases (defense-in-depth on the generated string).
func TestInjectionNeverReachesSQL(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "users")
	b.OrderBy("id; DROP TABLE users")
	q, _ := b.buildSelect()
	if strings.Contains(q, "DROP TABLE") {
		t.Fatalf("injection payload leaked into SQL: %q", q)
	}
}

// TestRawEscapeHatches confirms the explicit raw variants still allow trusted
// expressions through unchanged.
func TestRawEscapeHatches(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "posts")
	b.SelectRaw("COUNT(*) AS n").OrderByRaw("RANDOM()").GroupByRaw("date_trunc('day', created_at)")
	if b.err != nil {
		t.Fatalf("raw variants should not error: %v", b.err)
	}
	q, _ := b.buildSelect()
	for _, want := range []string{"COUNT(*) AS n", "RANDOM()", "date_trunc('day', created_at)"} {
		if !strings.Contains(q, want) {
			t.Fatalf("raw expression %q missing from %q", want, q)
		}
	}
}

// TestMySQLBacktickQuoting verifies the MySQL grammar quotes with backticks.
func TestMySQLBacktickQuoting(t *testing.T) {
	b := newTestBuilder(DriverMySQL, "users")
	b.Select("id").OrderBy("created_at DESC")
	if b.err != nil {
		t.Fatalf("unexpected error: %v", b.err)
	}
	q, _ := b.buildSelect()
	if !strings.Contains(q, "`id`") || !strings.Contains(q, "`created_at` DESC") {
		t.Fatalf("expected backtick quoting, got %q", q)
	}
}
