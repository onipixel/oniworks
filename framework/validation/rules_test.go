package validation

import "testing"

// TestRequiredAcceptsZeroValueNumbers verifies a legitimate 0 / false on a
// non-pointer field is NOT rejected by required (the previous behavior wrongly
// treated any zero value as missing).
func TestRequiredAcceptsZeroValueNumbers(t *testing.T) {
	type Form struct {
		Age    int  `validate:"required"`
		Active bool `validate:"required"`
	}
	if err := Validate(&Form{Age: 0, Active: false}); err != nil {
		t.Fatalf("zero number / false bool should satisfy required, got: %v", err)
	}
}

// TestRequiredRejectsEmptyString verifies required still catches empty strings
// and nil slices.
func TestRequiredRejectsEmptyString(t *testing.T) {
	type Form struct {
		Name string   `validate:"required"`
		Tags []string `validate:"required"`
	}
	if err := Validate(&Form{}); err == nil {
		t.Fatal("expected required to reject empty string and nil slice")
	}
}

// TestRequiredRejectsNilPointer verifies a nil pointer is treated as missing.
func TestRequiredRejectsNilPointer(t *testing.T) {
	type Form struct {
		Ptr *int `validate:"required"`
	}
	if err := Validate(&Form{}); err == nil {
		t.Fatal("expected required to reject nil pointer")
	}
}

// TestMinRejectsUnsupportedType is the silent-bypass regression: min on a type
// that is neither string nor numeric must error, not silently pass.
func TestMinRejectsUnsupportedType(t *testing.T) {
	v := New()
	if msg := v.applyRule("min", struct{ X int }{}, "5"); msg == "" {
		t.Fatal("min on an unsupported type must produce an error, not silently pass")
	}
}

// TestMinMaxNumeric verifies numeric min/max still work.
func TestMinMaxNumeric(t *testing.T) {
	type Form struct {
		N int `validate:"min=10,max=20"`
	}
	if err := Validate(&Form{N: 5}); err == nil {
		t.Fatal("expected min violation for N=5")
	}
	if err := Validate(&Form{N: 15}); err != nil {
		t.Fatalf("N=15 should be within [10,20], got: %v", err)
	}
	if err := Validate(&Form{N: 25}); err == nil {
		t.Fatal("expected max violation for N=25")
	}
}
