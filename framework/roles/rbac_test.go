package roles

import (
	"context"
	"testing"
)

// TestWildcardMatchSegmentAware covers the over-grant regression: only
// full-segment "*" wildcards match, so a partial like "user*" can't leak across
// permission boundaries.
func TestWildcardMatchSegmentAware(t *testing.T) {
	cases := []struct {
		pattern, target string
		want            bool
	}{
		{"*", "anything:here", true},
		{"users:*", "users:create", true},
		{"users:*", "users:posts:edit", true},
		{"users:*", "userprofile:read", false}, // must NOT cross boundary
		{"users:*", "users", false},            // needs a sub-segment
		{"org:*:read", "org:5:read", true},
		{"org:*:read", "org:5:write", false},
		{"users:create", "users:create", true},
		{"users:create", "users:delete", false},
		// The dangerous over-grant the fix targets:
		{"user*", "users:delete", false},
		{"user*", "userprofile:read", false},
		{"user*", "user*", true}, // only matches itself literally
	}
	for _, c := range cases {
		if got := isWildcardMatch(c.pattern, c.target); got != c.want {
			t.Errorf("isWildcardMatch(%q, %q) = %v, want %v", c.pattern, c.target, got, c.want)
		}
	}
}

func staticRoles(roles ...string) func(context.Context, int64) ([]string, error) {
	return func(context.Context, int64) ([]string, error) { return roles, nil }
}

// TestCanWithWildcardRole verifies end-to-end grant logic through Can.
func TestCanWithWildcardRole(t *testing.T) {
	m := New(staticRoles("admin"))
	m.Define("admin", "users:*", "posts:create")

	ctx := context.Background()
	check := func(perm Permission, want bool) {
		t.Helper()
		got, err := m.Can(ctx, 1, perm)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Can(%q) = %v, want %v", perm, got, want)
		}
	}
	check("users:create", true)  // via users:*
	check("users:ban", true)     // via users:*
	check("posts:create", true)  // exact
	check("posts:delete", false) // not granted
	check("userprofile:read", false)
}

// TestCannotAndAllPermissions covers the inverse and aggregation helpers.
func TestCannotAndAllPermissions(t *testing.T) {
	m := New(staticRoles("editor"))
	m.Define("editor", "posts:create", "posts:update")
	ctx := context.Background()

	if no, _ := m.Cannot(ctx, 1, "posts:delete"); !no {
		t.Error("editor should NOT be able to posts:delete")
	}
	perms, _ := m.AllPermissions(ctx, 1)
	if len(perms) != 2 {
		t.Fatalf("expected 2 permissions, got %d: %v", len(perms), perms)
	}
}

// TestUnknownRoleGrantsNothing verifies a user with an undefined role gets no
// permissions (fail-closed).
func TestUnknownRoleGrantsNothing(t *testing.T) {
	m := New(staticRoles("ghost"))
	if ok, _ := m.Can(context.Background(), 1, "users:create"); ok {
		t.Error("undefined role must grant nothing")
	}
}
