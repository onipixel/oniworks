package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onipixel/oniworks/framework/session"
	"golang.org/x/crypto/bcrypt"
)

// memStore is a minimal in-memory session.Store for testing Regenerate.
type memStore struct{ data map[string]map[string]any }

func newMemStore() *memStore { return &memStore{data: map[string]map[string]any{}} }

func (s *memStore) Get(_ context.Context, id string) (map[string]any, error) {
	return s.data[id], nil
}
func (s *memStore) Set(_ context.Context, id string, d map[string]any, _ time.Duration) error {
	s.data[id] = d
	return nil
}
func (s *memStore) Delete(_ context.Context, id string) error { delete(s.data, id); return nil }
func (s *memStore) Regenerate(_ context.Context, oldID string) (string, error) {
	newID := oldID + "-rotated"
	if d, ok := s.data[oldID]; ok {
		s.data[newID] = d
		delete(s.data, oldID)
	}
	return newID, nil
}

type provider struct{ u User }

func (p provider) FindByID(context.Context, int64) (User, error) { return p.u, nil }
func (p provider) FindByEmail(_ context.Context, email string) (User, error) {
	if p.u != nil && p.u.GetEmail() == email {
		return p.u, nil
	}
	return nil, nil // not found
}

type acct struct {
	id    int64
	email string
	hash  string
}

func (a acct) GetID() int64        { return a.id }
func (a acct) GetEmail() string    { return a.email }
func (a acct) GetPassword() string { return a.hash }

// TestAttemptRegeneratesSessionOnLogin is the session-fixation regression: the
// session ID must change after a successful login.
func TestAttemptRegeneratesSessionOnLogin(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.DefaultCost)
	g := NewGuard(provider{u: acct{id: 7, email: "a@b.com", hash: string(hash)}}, nil, goodSecret)

	store := newMemStore()
	mgr := session.NewManager(store)
	sess, err := mgr.Start(context.Background(), httptest.NewRequest("POST", "/login", nil), httptest.NewRecorder())
	if err != nil {
		t.Fatalf("session start: %v", err)
	}
	// Persist the new session so Regenerate has data to carry over.
	if err := mgr.Save(context.Background(), httptest.NewRecorder(), sess); err != nil {
		t.Fatalf("session save: %v", err)
	}
	originalID := sess.ID

	if _, err := g.Attempt(context.Background(), "a@b.com", "correct-horse", sess); err != nil {
		t.Fatalf("Attempt: %v", err)
	}
	if sess.ID == originalID {
		t.Fatal("session ID was not rotated on login (session-fixation risk)")
	}
	if v, ok := sess.Get("_auth_user_id"); !ok || v != int64(7) {
		t.Fatalf("auth user id not set on rotated session: %v ok=%v", v, ok)
	}
}

// TestAttemptUnknownUserStillFails verifies an unknown email returns invalid
// credentials (and exercises the constant-time dummy-compare path).
func TestAttemptUnknownUserStillFails(t *testing.T) {
	g := NewGuard(provider{u: acct{id: 1, email: "real@b.com", hash: "x"}}, nil, goodSecret)
	_, err := g.Attempt(context.Background(), "ghost@b.com", "whatever", nil)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v, want ErrInvalidCredentials", err)
	}
}
