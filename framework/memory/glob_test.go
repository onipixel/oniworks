package memory

import "testing"

// TestGlobMatch covers the wildcard matcher Keys relies on, including the
// mid-pattern "*" that presence LeaveAll needs.
func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "anything", true},
		{"oni:presence:room_1:conn5", "oni:presence:room_1:conn5", true},
		{"oni:presence:*", "oni:presence:room_1:conn5", true},
		{"oni:presence:*:conn5", "oni:presence:room_1:conn5", true},   // mid-pattern *
		{"oni:presence:*:conn5", "oni:presence:room_9:conn5", true},   // any channel
		{"oni:presence:*:conn5", "oni:presence:room_1:conn6", false},  // wrong conn
		{"oni:presence:*:conn5", "other:room_1:conn5", false},         // wrong prefix
		{"a:*:b:*:c", "a:1:b:2:c", true},                              // multiple *
		{"a:*:b:*:c", "a:1:b:2:d", false},
		{"prefix*", "prefix-anything", true},
		{"prefix*", "nope", false},
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.s); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

// TestKeysMidPattern verifies Store.Keys honors a mid-pattern wildcard, the
// case LeaveAll exercises.
func TestKeysMidPattern(t *testing.T) {
	s := New(Options{})
	s.Set("oni:presence:room_1:conn5", 1, 0)
	s.Set("oni:presence:room_2:conn5", 1, 0)
	s.Set("oni:presence:room_1:conn9", 1, 0)

	keys := s.Keys("oni:presence:*:conn5")
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys for conn5 across channels, got %d: %v", len(keys), keys)
	}
}
