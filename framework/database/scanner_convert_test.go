package database

import (
	"testing"
	"time"
)

// TestConvertAssignByteSources is the silent-data-loss regression: Postgres
// often returns NUMERIC/bool/timestamp as []byte or string. These must convert,
// not silently leave the zero value.
func TestConvertAssignByteSources(t *testing.T) {
	t.Run("[]byte to int64", func(t *testing.T) {
		var n int64
		if err := convertAssign(&n, []byte("42")); err != nil {
			t.Fatal(err)
		}
		if n != 42 {
			t.Fatalf("got %d, want 42", n)
		}
	})

	t.Run("[]byte numeric to float64", func(t *testing.T) {
		var f float64
		if err := convertAssign(&f, []byte("3.14")); err != nil {
			t.Fatal(err)
		}
		if f != 3.14 {
			t.Fatalf("got %v, want 3.14", f)
		}
	})

	t.Run("string bool t", func(t *testing.T) {
		var b bool
		if err := convertAssign(&b, "t"); err != nil {
			t.Fatal(err)
		}
		if !b {
			t.Fatal("expected true")
		}
	})

	t.Run("[]byte timestamp to time.Time", func(t *testing.T) {
		var tm time.Time
		if err := convertAssign(&tm, []byte("2026-06-29 12:30:00")); err != nil {
			t.Fatal(err)
		}
		if tm.Year() != 2026 || tm.Hour() != 12 {
			t.Fatalf("got %v, want 2026-06-29 12:30", tm)
		}
	})

	t.Run("RFC3339 timestamp", func(t *testing.T) {
		var tm time.Time
		if err := convertAssign(&tm, "2026-06-29T12:30:00Z"); err != nil {
			t.Fatal(err)
		}
		if tm.Year() != 2026 {
			t.Fatalf("got %v", tm)
		}
	})

	t.Run("int from float string 42.0", func(t *testing.T) {
		var n int
		if err := convertAssign(&n, "42.0"); err != nil {
			t.Fatal(err)
		}
		if n != 42 {
			t.Fatalf("got %d, want 42", n)
		}
	})
}

// TestConvertAssignErrorsOnGarbage verifies unconvertible input returns an error
// rather than silently producing a zero value.
func TestConvertAssignErrorsOnGarbage(t *testing.T) {
	var n int64
	if err := convertAssign(&n, "not-a-number"); err == nil {
		t.Fatal("expected error scanning garbage into int64, got nil (silent data loss)")
	}
	var f float64
	if err := convertAssign(&f, []byte("xyz")); err == nil {
		t.Fatal("expected error scanning garbage into float64")
	}
	var tm time.Time
	if err := convertAssign(&tm, "tomorrow"); err == nil {
		t.Fatal("expected error scanning garbage into time.Time")
	}
}
