package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onipixel/oniworks/framework/session"
	"github.com/onipixel/oniworks/framework/session/drivers"
)

func newManager() *session.Manager {
	return session.NewManager(drivers.NewMemoryStore())
}

func TestStartAndSave(t *testing.T) {
	m := newManager()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	sess, err := m.Start(context.Background(), req, w)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if sess == nil {
		t.Fatal("session is nil")
	}
	if !sess.IsNew() {
		t.Error("expected new session")
	}

	sess.Set("user_id", int64(99))
	if err := m.Save(context.Background(), w, sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Cookie should be set
	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Error("expected session cookie to be set")
	}
}

func TestGetSet(t *testing.T) {
	m := newManager()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	sess, _ := m.Start(context.Background(), req, w)
	sess.Set("name", "Alice")
	sess.Set("count", 42)

	name, ok := sess.Get("name")
	if !ok || name != "Alice" {
		t.Errorf("Get name: got %v, ok=%v", name, ok)
	}
	count, ok := sess.Get("count")
	if !ok || count != 42 {
		t.Errorf("Get count: got %v, ok=%v", count, ok)
	}
}

func TestDelete(t *testing.T) {
	m := newManager()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	sess, _ := m.Start(context.Background(), req, w)
	sess.Set("temp", "value")
	sess.Delete("temp")

	if sess.Has("temp") {
		t.Error("deleted key should not exist")
	}
}

func TestFlash(t *testing.T) {
	m := newManager()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	sess, _ := m.Start(context.Background(), req, w)
	sess.Flash("notice", "Login successful")

	msg, ok := sess.GetFlash("notice")
	if !ok {
		t.Fatal("flash message not found")
	}
	if msg != "Login successful" {
		t.Errorf("flash: got %v", msg)
	}

	// Flash consumed, should be gone
	_, ok2 := sess.GetFlash("notice")
	if ok2 {
		t.Error("flash should be consumed after first read")
	}
}

func TestDestroy(t *testing.T) {
	m := newManager()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	sess, _ := m.Start(context.Background(), req, w)
	sess.Set("data", "value")
	_ = m.Save(context.Background(), w, sess)

	if err := m.Destroy(context.Background(), w, sess); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

func TestReloadSession(t *testing.T) {
	m := newManager()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Create and save session
	sess, _ := m.Start(context.Background(), req, w)
	sess.Set("key", "value")
	_ = m.Save(context.Background(), w, sess)

	// Get the session cookie
	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Skip("no session cookie set, skip reload test")
	}

	// Reload with the cookie
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])
	w2 := httptest.NewRecorder()

	sess2, err := m.Start(context.Background(), req2, w2)
	if err != nil {
		t.Fatalf("reload Start: %v", err)
	}
	if sess2.IsNew() {
		t.Error("reloaded session should not be new")
	}
	val, ok := sess2.Get("key")
	if !ok || val != "value" {
		t.Errorf("reloaded session key: got %v, ok=%v", val, ok)
	}
}
