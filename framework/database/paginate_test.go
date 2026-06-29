package database

import (
	"strings"
	"testing"
)

// TestBuildCountIncludesSoftDelete is the Paginate-total regression: the COUNT
// query must carry the same deleted_at filter as the data query, or Total
// over-counts trashed rows.
func TestBuildCountIncludesSoftDelete(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "posts")
	b.softDelete = true
	q, _ := b.buildCount()
	if !strings.Contains(q, `"deleted_at" IS NULL`) {
		t.Fatalf("count query missing soft-delete filter: %q", q)
	}
	if !strings.Contains(q, "COUNT(*)") {
		t.Fatalf("expected COUNT(*), got %q", q)
	}
}

// TestBuildCountWithTrashedSkipsFilter verifies WithTrashed counts include
// trashed rows.
func TestBuildCountWithTrashedSkipsFilter(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "posts")
	b.softDelete = true
	b.withTrashed = true
	q, _ := b.buildCount()
	if strings.Contains(q, "deleted_at") {
		t.Fatalf("WithTrashed count should not filter deleted_at: %q", q)
	}
}

// TestBuildCountGroupedUsesSubquery verifies a grouped query counts the number
// of groups via a subquery rather than returning a per-group count.
func TestBuildCountGroupedUsesSubquery(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "comments")
	b.GroupBy("post_id")
	q, _ := b.buildCount()
	if !strings.Contains(q, "SELECT COUNT(*) FROM (") || !strings.Contains(q, "_oni_count") {
		t.Fatalf("grouped count should wrap in a subquery, got %q", q)
	}
	if !strings.Contains(q, `GROUP BY "post_id"`) {
		t.Fatalf("inner query should retain GROUP BY, got %q", q)
	}
}

// TestSliceToAny verifies Page.Items population from a typed destination slice.
func TestSliceToAny(t *testing.T) {
	type Post struct{ ID int }
	posts := []Post{{1}, {2}, {3}}
	items := sliceToAny(&posts)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if p, ok := items[1].(Post); !ok || p.ID != 2 {
		t.Fatalf("item[1] = %v, want Post{2}", items[1])
	}

	// Non-slice input returns nil rather than panicking.
	if got := sliceToAny("not a slice"); got != nil {
		t.Fatalf("expected nil for non-slice, got %v", got)
	}
}
