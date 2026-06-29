// Package session provides server-side session management for OniWorks.
// The default store is Oni Memory (in-process, fast). A database store is also available.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

// Store is the interface all session stores must implement.
type Store interface {
	Get(ctx context.Context, id string) (map[string]any, error)
	Set(ctx context.Context, id string, data map[string]any, ttl time.Duration) error
	Delete(ctx context.Context, id string) error
	Regenerate(ctx context.Context, oldID string) (string, error)
}

// Config holds session configuration.
type Config struct {
	CookieName string
	Domain     string
	Path       string
	Secure     bool
	HTTPOnly   bool
	SameSite   http.SameSite
	TTL        time.Duration
}

// DefaultConfig returns production-safe session defaults.
func DefaultConfig() Config {
	return Config{
		CookieName: "oni_session",
		Path:       "/",
		HTTPOnly:   true,
		Secure:     true,
		SameSite:   http.SameSiteLaxMode,
		TTL:        2 * time.Hour,
	}
}

// Session represents a single user session.
type Session struct {
	ID      string
	data    map[string]any
	store   Store
	cfg     Config
	dirty   bool // true if data was modified (needs save)
	isNew   bool
}

// Manager manages session lifecycle.
type Manager struct {
	store Store
	cfg   Config
}

// NewManager creates a session Manager.
func NewManager(store Store, cfg ...Config) *Manager {
	c := DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &Manager{store: store, cfg: c}
}

// Start reads or creates the session for the current request.
func (m *Manager) Start(ctx context.Context, r *http.Request, w http.ResponseWriter) (*Session, error) {
	cookie, err := r.Cookie(m.cfg.CookieName)
	if err != nil || cookie.Value == "" {
		return m.create(ctx, w)
	}

	data, err := m.store.Get(ctx, cookie.Value)
	if err != nil || data == nil {
		return m.create(ctx, w)
	}

	return &Session{
		ID:    cookie.Value,
		data:  data,
		store: m.store,
		cfg:   m.cfg,
	}, nil
}

func (m *Manager) create(ctx context.Context, w http.ResponseWriter) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	sess := &Session{
		ID:    id,
		data:  make(map[string]any),
		store: m.store,
		cfg:   m.cfg,
		dirty: true,
		isNew: true,
	}
	m.setCookie(w, id)
	return sess, nil
}

func (m *Manager) setCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cfg.CookieName,
		Value:    id,
		Path:     m.cfg.Path,
		Domain:   m.cfg.Domain,
		MaxAge:   int(m.cfg.TTL.Seconds()),
		Secure:   m.cfg.Secure,
		HttpOnly: m.cfg.HTTPOnly,
		SameSite: m.cfg.SameSite,
	})
}

// Save persists the session data to the store. Call this at the end of each request.
func (m *Manager) Save(ctx context.Context, w http.ResponseWriter, sess *Session) error {
	if !sess.dirty {
		return nil
	}
	if err := m.store.Set(ctx, sess.ID, sess.data, m.cfg.TTL); err != nil {
		return fmt.Errorf("session: save: %w", err)
	}
	m.setCookie(w, sess.ID)
	sess.dirty = false
	return nil
}

// Destroy deletes the session and clears the cookie.
func (m *Manager) Destroy(ctx context.Context, w http.ResponseWriter, sess *Session) error {
	if err := m.store.Delete(ctx, sess.ID); err != nil {
		return fmt.Errorf("session: destroy: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     m.cfg.CookieName,
		Value:    "",
		Path:     m.cfg.Path,
		MaxAge:   -1,
		HttpOnly: true,
	})
	sess.data = nil
	return nil
}

// ─────────────────────────── Session data access ───────────────────

func (s *Session) Get(key string) (any, bool) {
	v, ok := s.data[key]
	return v, ok
}

func (s *Session) Set(key string, value any) {
	if s.data == nil {
		s.data = make(map[string]any)
	}
	s.data[key] = value
	s.dirty = true
}

func (s *Session) Delete(key string) {
	delete(s.data, key)
	s.dirty = true
}

func (s *Session) Has(key string) bool {
	_, ok := s.data[key]
	return ok
}

func (s *Session) Flash(key string, value any) {
	s.Set("_flash_"+key, value)
}

func (s *Session) GetFlash(key string) (any, bool) {
	v, ok := s.Get("_flash_" + key)
	if ok {
		s.Delete("_flash_" + key)
	}
	return v, ok
}

func (s *Session) IsNew() bool { return s.isNew }

// Regenerate rotates the session ID while preserving the session data. Call it
// on any privilege change — especially after login — to prevent session
// fixation, where an attacker fixes a victim's pre-auth session ID and reuses
// it once authenticated. The new ID is written to the cookie on the next Save.
func (s *Session) Regenerate(ctx context.Context) error {
	if s.store == nil {
		return fmt.Errorf("session: regenerate: no store bound to session")
	}
	newID, err := s.store.Regenerate(ctx, s.ID)
	if err != nil {
		return fmt.Errorf("session: regenerate: %w", err)
	}
	s.ID = newID
	s.dirty = true
	return nil
}

// ─────────────────────────── helpers ──────────────────────────────

func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
