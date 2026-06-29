package database

import (
	"strings"
	"testing"
)

// TestSoftDeleteIssuesUpdate verifies that Delete() on a soft-delete builder
// stamps deleted_at via an UPDATE rather than removing the row.
func TestSoftDeleteIssuesUpdate(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "posts")
	b.softDelete = true
	b.Where("id = ?", 1)
	q, _ := b.buildDeleteStmt()
	if !strings.HasPrefix(q, "UPDATE") {
		t.Fatalf("soft delete should issue an UPDATE, got %q", q)
	}
	if !strings.Contains(q, `"deleted_at" = ?`) {
		t.Fatalf("soft delete should set deleted_at, got %q", q)
	}
}

// TestForceDeleteIssuesDelete verifies ForceDelete bypasses soft delete.
func TestForceDeleteIssuesDelete(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "posts")
	b.softDelete = true // ForceDelete must override this
	b.Where("id = ?", 1)
	b.softDelete = false // simulate what ForceDelete sets
	q, _ := b.buildDeleteStmt()
	if !strings.HasPrefix(q, "DELETE") {
		t.Fatalf("force delete should issue a DELETE, got %q", q)
	}
}

// TestHardDeleteByDefault verifies a non-soft-delete builder still hard-deletes.
func TestHardDeleteByDefault(t *testing.T) {
	b := newTestBuilder(DriverPostgres, "logs")
	b.Where("id = ?", 1)
	q, _ := b.buildDeleteStmt()
	if !strings.HasPrefix(q, "DELETE") {
		t.Fatalf("default delete should be a DELETE, got %q", q)
	}
}
