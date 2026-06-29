package middleware

import "testing"

// TestSecureCompareRejectsEmpty verifies a fail-closed empty token never
// validates and that mismatches/lengths are handled safely.
func TestSecureCompareRejectsEmpty(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "", false},
		{"abc", "", false},
		{"", "abc", false},
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "abcd", false}, // differing length must not panic, must be false
	}
	for _, c := range cases {
		if got := secureCompare(c.a, c.b); got != c.want {
			t.Fatalf("secureCompare(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// TestGenerateCSRFTokenUnique verifies tokens are non-empty and unique.
func TestGenerateCSRFTokenUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := generateCSRFToken()
		if err != nil {
			t.Fatalf("generateCSRFToken: %v", err)
		}
		if len(tok) != 64 { // 32 bytes hex-encoded
			t.Fatalf("token length = %d, want 64", len(tok))
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %s", tok)
		}
		seen[tok] = true
	}
}
