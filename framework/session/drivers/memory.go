// Package drivers provides session store implementations.
package drivers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// entry is a stored session with its expiry time.
type entry struct {
	data    map[string]any
	expiresAt time.Time
}

// MemoryStore is a fast in-process session store backed by a sync.Map.
// It is the default session store in development and is suitable for single-node production.
// Use a database or Redis store for multi-node deployments.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]entry
}

// NewMemoryStore creates a MemoryStore and starts the background eviction goroutine.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{entries: make(map[string]entry)}
	go s.evict()
	return s
}

func (s *MemoryStore) Get(_ context.Context, id string) (map[string]any, error) {
	s.mu.RLock()
	e, ok := s.entries[id]
	s.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, nil
	}
	// Return a copy to prevent external mutation
	copy := make(map[string]any, len(e.data))
	for k, v := range e.data {
		copy[k] = v
	}
	return copy, nil
}

func (s *MemoryStore) Set(_ context.Context, id string, data map[string]any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	// Store a defensive copy
	cp := make(map[string]any, len(data))
	for k, v := range data {
		cp[k] = v
	}
	s.mu.Lock()
	s.entries[id] = entry{data: cp, expiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	delete(s.entries, id)
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Regenerate(ctx context.Context, oldID string) (string, error) {
	s.mu.Lock()
	e, ok := s.entries[oldID]
	if ok {
		delete(s.entries, oldID)
	}
	s.mu.Unlock()

	newID, err := generateSessionID()
	if err != nil {
		return "", err
	}
	if ok {
		_ = s.Set(ctx, newID, e.data, time.Until(e.expiresAt))
	}
	return newID, nil
}

func (s *MemoryStore) evict() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for id, e := range s.entries {
			if now.After(e.expiresAt) {
				delete(s.entries, id)
			}
		}
		s.mu.Unlock()
	}
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
