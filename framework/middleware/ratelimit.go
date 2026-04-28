package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	onihttp "github.com/oniworks/oniworks/framework/http"
)

// RateLimitConfig configures the rate limiter.
type RateLimitConfig struct {
	// Max is the maximum number of requests allowed in the window.
	Max int
	// Window is the sliding window duration.
	Window time.Duration
	// KeyFunc extracts the rate-limit key from the request (default: client IP).
	KeyFunc func(c *onihttp.Context) string
	// OnExceeded is called when the limit is exceeded (default: 429 JSON response).
	OnExceeded func(c *onihttp.Context) error
}

// RateLimit returns a sliding-window rate limiter middleware.
//
//	middleware.RateLimit(100, time.Minute)  // 100 req/min per IP
func RateLimit(max int, window time.Duration, opts ...RateLimitConfig) onihttp.MiddlewareFunc {
	cfg := RateLimitConfig{
		Max:    max,
		Window: window,
		KeyFunc: func(c *onihttp.Context) string {
			return c.IP()
		},
		OnExceeded: func(c *onihttp.Context) error {
			c.Response.Header().Set("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			return c.JSON(http.StatusTooManyRequests, onihttp.Map{
				"error":       "rate limit exceeded",
				"retry_after": window.Seconds(),
			})
		},
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.KeyFunc != nil {
			cfg.KeyFunc = o.KeyFunc
		}
		if o.OnExceeded != nil {
			cfg.OnExceeded = o.OnExceeded
		}
	}

	store := newSlidingWindowStore(max, window)

	return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			key := cfg.KeyFunc(c)
			if !store.Allow(key) {
				return cfg.OnExceeded(c)
			}
			c.Response.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", max))
			c.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", store.Remaining(key)))
			return next(c)
		}
	}
}

// ─────────────────────────── sliding window counter ────────────────

// windowEntry tracks request timestamps for one key.
type windowEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
}

// slidingWindowStore holds per-key sliding window state.
type slidingWindowStore struct {
	mu      sync.RWMutex
	entries map[string]*windowEntry
	max     int
	window  time.Duration
}

func newSlidingWindowStore(max int, window time.Duration) *slidingWindowStore {
	s := &slidingWindowStore{
		entries: make(map[string]*windowEntry),
		max:     max,
		window:  window,
	}
	go s.cleanup()
	return s
}

// Allow returns true if the request is within the rate limit, recording the timestamp.
func (s *slidingWindowStore) Allow(key string) bool {
	entry := s.getOrCreate(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-s.window)

	// Evict expired timestamps (sliding window)
	valid := entry.timestamps[:0]
	for _, t := range entry.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	entry.timestamps = valid

	if len(entry.timestamps) >= s.max {
		return false
	}
	entry.timestamps = append(entry.timestamps, now)
	return true
}

// Remaining returns the number of remaining requests in the current window.
func (s *slidingWindowStore) Remaining(key string) int {
	entry := s.getOrCreate(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	cutoff := time.Now().Add(-s.window)
	count := 0
	for _, t := range entry.timestamps {
		if t.After(cutoff) {
			count++
		}
	}
	remaining := s.max - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *slidingWindowStore) getOrCreate(key string) *windowEntry {
	s.mu.RLock()
	e, ok := s.entries[key]
	s.mu.RUnlock()
	if ok {
		return e
	}
	s.mu.Lock()
	if e, ok = s.entries[key]; !ok {
		e = &windowEntry{}
		s.entries[key] = e
	}
	s.mu.Unlock()
	return e
}

func (s *slidingWindowStore) cleanup() {
	ticker := time.NewTicker(s.window)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-s.window)
		s.mu.Lock()
		for key, entry := range s.entries {
			entry.mu.Lock()
			valid := entry.timestamps[:0]
			for _, t := range entry.timestamps {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			entry.timestamps = valid
			if len(entry.timestamps) == 0 {
				delete(s.entries, key)
			}
			entry.mu.Unlock()
		}
		s.mu.Unlock()
	}
}
