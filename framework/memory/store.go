package memory

import (
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Options configures a Store instance.
type Options struct {
	// NodeID uniquely identifies this node. Auto-generated if empty.
	NodeID string

	// BindAddr is the TCP address this node listens on for gossip (e.g. "0.0.0.0:7946").
	// Leave empty to run in single-node mode (no gossip, no cross-node sync).
	BindAddr string

	// Peers is the list of known peer addresses for gossip bootstrapping.
	// Example: []string{"10.0.0.2:7946", "10.0.0.3:7946"}
	Peers []string

	// GossipSecret is a pre-shared secret used to authenticate peer connections.
	// Every node in a cluster must share the same value. If empty, gossip runs
	// UNAUTHENTICATED (any host that can reach BindAddr can read and inject data)
	// and a loud warning is logged — set this for any non-trusted network.
	GossipSecret string

	// Persist enables snapshot-to-disk on shutdown.
	Persist bool
	// SnapshotPath is the file path for the snapshot (default: "storage/memory.snap").
	SnapshotPath string

	// GracefulSave saves snapshot on SIGTERM/SIGINT.
	GracefulSave bool

	// RedisURL enables the Redis sync adapter instead of the built-in gossip.
	// When set, gossip is disabled and Redis pub/sub is used for cross-node sync.
	// Example: "redis://localhost:6379"
	RedisURL string

	// MaxKeys caps the number of stored keys (0 = unlimited).
	MaxKeys int

	// EvictInterval is how often the TTL eviction loop runs (default: 30s).
	EvictInterval time.Duration
}

// entry is a single KV entry with TTL metadata.
type entry struct {
	Value     any
	ExpiresAt time.Time // zero means no expiry
	Clock     ClockValue
}

func (e *entry) expired() bool {
	return !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt)
}

// Store is the OniWorks distributed in-memory database.
// It provides KV storage, pub/sub, TTL eviction, snapshot persistence,
// and cross-node sync via TCP gossip (or Redis as optional adapter).
type Store struct {
	opts    Options
	nodeID  string
	clock   *VectorClock

	mu      sync.RWMutex
	data    map[string]*entry

	pubsub  *PubSub
	gossip  *gossipTransport  // nil in single-node mode
	redis   *redisSyncAdapter // nil when not configured

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates and starts a Store.
func New(opts Options) *Store {
	nodeID := opts.NodeID
	if nodeID == "" {
		nodeID = generateNodeID()
	}
	if opts.SnapshotPath == "" {
		opts.SnapshotPath = "storage/memory.snap"
	}
	if opts.EvictInterval <= 0 {
		opts.EvictInterval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Store{
		opts:   opts,
		nodeID: nodeID,
		clock:  NewVectorClock(nodeID),
		data:   make(map[string]*entry),
		ctx:    ctx,
		cancel: cancel,
	}
	s.pubsub = newPubSub()

	// Load snapshot if it exists
	if opts.Persist {
		_ = s.loadSnapshot()
	}

	// Start TTL eviction loop
	go s.evictLoop()

	// Start cross-node transport
	if opts.RedisURL != "" {
		s.redis = newRedisSyncAdapter(s, opts.RedisURL)
	} else if opts.BindAddr != "" {
		s.gossip = newGossipTransport(s, opts.BindAddr, opts.Peers, opts.GossipSecret)
	}

	// Graceful shutdown
	if opts.GracefulSave {
		go s.watchShutdown()
	}

	return s
}

// ─────────────────────────── KV operations ────────────────────────

// Set stores key with value and an optional TTL (0 = no expiry).
func (s *Store) Set(key string, value any, ttl time.Duration) {
	cv := s.clock.Tick()
	e := &entry{Value: value, Clock: cv}
	if ttl > 0 {
		e.ExpiresAt = time.Now().Add(ttl)
	}
	s.mu.Lock()
	s.data[key] = e
	s.mu.Unlock()

	// Notify pub/sub subscribers for this key
	s.pubsub.publish("key:"+key, value)

	// Sync to peers
	s.syncSet(key, e)
}

// Get retrieves a value. Returns (nil, false) if absent or expired.
func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if e.expired() {
		// Best-effort lazy eviction; evictLoop will clean it up.
		s.mu.Lock()
		if cur, still := s.data[key]; still && cur.expired() {
			delete(s.data, key)
		}
		s.mu.Unlock()
		return nil, false
	}
	return e.Value, true
}

// GetString retrieves a string value. Returns ("", false) if absent or wrong type.
func (s *Store) GetString(key string) (string, bool) {
	v, ok := s.Get(key)
	if !ok {
		return "", false
	}
	str, ok := v.(string)
	return str, ok
}

// GetInt64 retrieves an int64 value.
func (s *Store) GetInt64(key string) (int64, bool) {
	v, ok := s.Get(key)
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// Has reports whether a key exists and is not expired.
func (s *Store) Has(key string) bool {
	_, ok := s.Get(key)
	return ok
}

// Delete removes a key.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
	s.syncDelete(key)
}

// Expire updates the TTL of an existing key.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	e, ok := s.data[key]
	if ok && !e.expired() {
		if ttl > 0 {
			e.ExpiresAt = time.Now().Add(ttl)
		} else {
			e.ExpiresAt = time.Time{}
		}
	}
	s.mu.Unlock()
	return ok
}

// ─────────────────────────── Atomic operations ─────────────────────

// Incr atomically increments a counter key by 1. Creates with value 1 if absent.
// Returns the new value.
func (s *Store) Incr(key string) int64 {
	return s.IncrBy(key, 1)
}

// IncrBy atomically increments a counter key by n. Returns the new value.
func (s *Store) IncrBy(key string, n int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	var current int64
	if e, ok := s.data[key]; ok && !e.expired() {
		switch v := e.Value.(type) {
		case int64:
			current = v
		case int:
			current = int64(v)
		case float64:
			current = int64(v)
		}
	}
	next := current + n
	cv := s.clock.Tick()
	s.data[key] = &entry{Value: next, Clock: cv}
	return next
}

// Decr atomically decrements a counter key by 1.
func (s *Store) Decr(key string) int64 { return s.IncrBy(key, -1) }

// CompareAndSwap atomically updates key to newValue only if it currently equals expected.
// Returns true if the swap occurred.
func (s *Store) CompareAndSwap(key string, expected, newValue any, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, exists := s.data[key]
	currentVal := any(nil)
	if exists && !e.expired() {
		currentVal = e.Value
	}

	if !reflect.DeepEqual(currentVal, expected) {
		return false
	}

	cv := s.clock.Tick()
	ne := &entry{Value: newValue, Clock: cv}
	if ttl > 0 {
		ne.ExpiresAt = time.Now().Add(ttl)
	}
	s.data[key] = ne
	return true
}

// ─────────────────────────── Key scanning ──────────────────────────

// Keys returns all non-expired keys matching the given glob prefix pattern.
// Use "*" to match all keys. Use "session:*" to match all session keys.
func (s *Store) Keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for k, e := range s.data {
		if e.expired() {
			continue
		}
		if globMatch(pattern, k) {
			keys = append(keys, k)
		}
	}
	return keys
}

// globMatch reports whether s matches a glob pattern where "*" matches any run
// of characters (including none). Unlike a trailing-only prefix match, "*" may
// appear anywhere — e.g. "oni:presence:*:conn5" matches a connection's presence
// keys across every channel.
func globMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	parts := strings.Split(pattern, "*")
	// The first segment must be a prefix.
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	// Middle segments must appear in order.
	for _, mid := range parts[1 : len(parts)-1] {
		idx := strings.Index(s, mid)
		if idx < 0 {
			return false
		}
		s = s[idx+len(mid):]
	}
	// The last segment must be a suffix of what remains.
	return strings.HasSuffix(s, parts[len(parts)-1])
}

// Count returns the number of non-expired keys matching a pattern.
func (s *Store) Count(pattern string) int { return len(s.Keys(pattern)) }

// Flush removes all keys. Use only in development/tests.
func (s *Store) Flush() {
	s.mu.Lock()
	s.data = make(map[string]*entry)
	s.mu.Unlock()
}

// ─────────────────────────── Pub/Sub ──────────────────────────────

// Publish broadcasts payload to all subscribers of topic.
// Cross-node: the gossip/Redis transport propagates this to peer nodes.
func (s *Store) Publish(topic string, payload any) {
	s.pubsub.publish(topic, payload)
	s.syncPublish(topic, payload)
}

// Subscribe registers handler to receive messages on topic.
// Topic can include wildcards: "user.*" matches "user.login", "user.logout".
// Returns a cancel function that unsubscribes the handler.
func (s *Store) Subscribe(topic string, handler func(topic string, payload any)) func() {
	return s.pubsub.subscribe(topic, handler)
}

// ─────────────────────────── Lifecycle ────────────────────────────

// Shutdown stops the store, saves snapshot if configured, and closes gossip connections.
func (s *Store) Shutdown() error {
	s.cancel()
	if s.gossip != nil {
		s.gossip.stop()
	}
	if s.redis != nil {
		s.redis.stop()
	}
	if s.opts.Persist {
		return s.saveSnapshot()
	}
	return nil
}

// ─────────────────────────── internal ──────────────────────────────

func (s *Store) evictLoop() {
	ticker := time.NewTicker(s.opts.EvictInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.evict()
		}
	}
}

func (s *Store) evict() {
	now := time.Now()
	s.mu.Lock()
	for k, e := range s.data {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			delete(s.data, k)
		}
	}
	s.mu.Unlock()
}

func (s *Store) watchShutdown() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-ch:
		_ = s.Shutdown()
	case <-s.ctx.Done():
	}
}

// applyRemoteSet applies a write received from a peer node (gossip or Redis).
func (s *Store) applyRemoteSet(key string, value any, expiresAt time.Time, clock ClockValue) {
	s.clock.Merge(clock)
	s.mu.Lock()
	existing, ok := s.data[key]
	// Last-write-wins: only apply if received clock is newer
	if !ok || !existing.Clock.After(clock) {
		s.data[key] = &entry{Value: value, ExpiresAt: expiresAt, Clock: clock}
	}
	s.mu.Unlock()
	// Notify local subscribers
	s.pubsub.publish("key:"+key, value)
}

// applyRemoteDelete applies a delete received from a peer.
func (s *Store) applyRemoteDelete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}

// applyRemotePublish delivers a pub/sub message from a peer to local subscribers.
func (s *Store) applyRemotePublish(topic string, payload any) {
	s.pubsub.publish(topic, payload)
}

// syncSet sends a set delta to all peers.
func (s *Store) syncSet(key string, e *entry) {
	if s.gossip != nil {
		s.gossip.broadcastSet(key, e)
	} else if s.redis != nil {
		s.redis.broadcastSet(key, e)
	}
}

// syncDelete sends a delete delta to all peers.
func (s *Store) syncDelete(key string) {
	if s.gossip != nil {
		s.gossip.broadcastDelete(key)
	} else if s.redis != nil {
		s.redis.broadcastDelete(key)
	}
}

// syncPublish sends a pub/sub event to all peers.
func (s *Store) syncPublish(topic string, payload any) {
	if s.gossip != nil {
		s.gossip.broadcastPublish(topic, payload)
	} else if s.redis != nil {
		s.redis.broadcastPublish(topic, payload)
	}
}

// ─────────────────────────── snapshot ─────────────────────────────

// snapshotData is the serialized form of the in-memory store.
type snapshotData struct {
	Entries map[string]snapshotEntry
}

type snapshotEntry struct {
	Value     any
	ExpiresAt time.Time
}

func init() {
	// Register common types for gob encoding
	gob.Register(map[string]any{})
	gob.Register([]any{})
	gob.Register(int64(0))
	gob.Register(float64(0))
	gob.Register(bool(false))
	gob.Register("")
}

func (s *Store) saveSnapshot() error {
	dir := filepath.Dir(s.opts.SnapshotPath)
	_ = os.MkdirAll(dir, 0755)

	f, err := os.Create(s.opts.SnapshotPath)
	if err != nil {
		return fmt.Errorf("memory: snapshot save: %w", err)
	}
	defer f.Close()

	s.mu.RLock()
	snap := snapshotData{Entries: make(map[string]snapshotEntry, len(s.data))}
	now := time.Now()
	for k, e := range s.data {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			continue
		}
		snap.Entries[k] = snapshotEntry{Value: e.Value, ExpiresAt: e.ExpiresAt}
	}
	s.mu.RUnlock()

	return gob.NewEncoder(f).Encode(snap)
}

func (s *Store) loadSnapshot() error {
	f, err := os.Open(s.opts.SnapshotPath)
	if err != nil {
		return nil // snapshot doesn't exist yet — that's fine
	}
	defer f.Close()

	var snap snapshotData
	if err := gob.NewDecoder(f).Decode(&snap); err != nil {
		return fmt.Errorf("memory: snapshot load: %w", err)
	}

	now := time.Now()
	s.mu.Lock()
	for k, se := range snap.Entries {
		if !se.ExpiresAt.IsZero() && now.After(se.ExpiresAt) {
			continue // skip expired entries
		}
		s.data[k] = &entry{Value: se.Value, ExpiresAt: se.ExpiresAt}
	}
	s.mu.Unlock()
	return nil
}

// generateNodeID creates a random node identifier.
func generateNodeID() string {
	b := make([]byte, 8)
	_, _ = randReader.Read(b)
	return fmt.Sprintf("node-%x", b)
}
