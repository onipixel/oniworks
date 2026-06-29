package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAdminFailsClosedWithoutAuth verifies the panel denies every route when no
// authorizer is configured — the panel must never serve data unauthenticated.
func TestAdminFailsClosedWithoutAuth(t *testing.T) {
	p := New(nil) // no WithAuth
	h := p.Handler()

	for _, path := range []string{"/admin/", "/admin/users", "/admin/users/new", "/admin/users/1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("path %s: got %d, want 403 (fail-closed without authorizer)", path, rr.Code)
		}
	}
}

// TestAdminRejectsUnauthorized verifies a configured authorizer that returns
// false blocks access with 403.
func TestAdminRejectsUnauthorized(t *testing.T) {
	p := New(nil, WithAuth(func(r *http.Request) bool { return false }))
	rr := httptest.NewRecorder()
	p.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403 for rejecting authorizer", rr.Code)
	}
}

// TestAdminAllowsAuthorized verifies an authorizer returning true lets the
// request reach the handler (dashboard renders 200).
func TestAdminAllowsAuthorized(t *testing.T) {
	called := false
	p := New(nil, WithAuth(func(r *http.Request) bool { called = true; return true }))
	rr := httptest.NewRecorder()
	p.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if !called {
		t.Fatal("authorizer was never invoked")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 for authorized dashboard request", rr.Code)
	}
}

// TestAPIHandlerGuarded verifies the JSON API is guarded too.
func TestAPIHandlerGuarded(t *testing.T) {
	p := New(nil)
	p.Register(&Resource{Name: "User", Model: struct {
		ID int64 `db:"id"`
	}{}})
	rr := httptest.NewRecorder()
	p.APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/users", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403 (API must fail closed without authorizer)", rr.Code)
	}
}

// TestResourceHasColumn covers the search-column allow-list used to block the
// admin search injection.
func TestResourceHasColumn(t *testing.T) {
	r := &Resource{Columns: []string{"id", "email", "name"}}
	if !r.hasColumn("email") {
		t.Fatal("expected email to be an allowed column")
	}
	if r.hasColumn("email ILIKE '%' OR 1=1 --") {
		t.Fatal("injection payload must not be treated as a known column")
	}
}
