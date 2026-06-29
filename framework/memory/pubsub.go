package memory

import (
	"strings"
	"sync"
	"sync/atomic"
)

// PubSub is a thread-safe, fan-out publish/subscribe engine.
// Topics support wildcard matching: "user.*" matches "user.login", "user.42.status".
type PubSub struct {
	mu          sync.RWMutex
	subscribers map[uint64]*subscriber
	nextID      uint64
}

type subscriber struct {
	id      uint64
	topic   string
	handler func(topic string, payload any)
	buf     chan pubMsg

	// mu guards buf against concurrent deliver/close. closed makes cancellation
	// idempotent and prevents a send-on-closed-channel panic when a publish
	// races with an unsubscribe.
	mu     sync.Mutex
	closed bool
}

// deliver performs a non-blocking send to the subscriber's buffer. It is a
// no-op once the subscriber has been closed, so it can never panic by sending
// on a closed channel.
func (s *subscriber) deliver(msg pubMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.buf <- msg:
	default:
		// Drop message for slow subscribers rather than blocking the publisher.
		// This is by design for high-throughput systems.
	}
}

// close idempotently closes the subscriber's buffer, stopping its delivery
// goroutine. Safe to call multiple times.
func (s *subscriber) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.buf)
}

type pubMsg struct {
	topic   string
	payload any
}

func newPubSub() *PubSub {
	return &PubSub{subscribers: make(map[uint64]*subscriber)}
}

// publish delivers payload to all matching subscribers.
func (ps *PubSub) publish(topic string, payload any) {
	ps.mu.RLock()
	subs := make([]*subscriber, 0)
	for _, s := range ps.subscribers {
		if topicMatches(s.topic, topic) {
			subs = append(subs, s)
		}
	}
	ps.mu.RUnlock()

	msg := pubMsg{topic: topic, payload: payload}
	for _, s := range subs {
		s.deliver(msg)
	}
}

// subscribe registers handler for topic. Returns a cancel function.
// Wildcards: "chat.*" matches "chat.room1", "chat.room2".
// "user.*.status" matches "user.42.status", "user.99.status".
func (ps *PubSub) subscribe(topic string, handler func(topic string, payload any)) func() {
	id := atomic.AddUint64(&ps.nextID, 1)
	sub := &subscriber{
		id:      id,
		topic:   topic,
		handler: handler,
		buf:     make(chan pubMsg, 256),
	}

	ps.mu.Lock()
	ps.subscribers[id] = sub
	ps.mu.Unlock()

	// Start delivery goroutine
	go func() {
		for msg := range sub.buf {
			sub.handler(msg.topic, msg.payload)
		}
	}()

	return func() {
		ps.mu.Lock()
		delete(ps.subscribers, id)
		ps.mu.Unlock()
		sub.close()
	}
}

// topicMatches reports whether pattern matches topic.
// Patterns: "*" = match anything, "user.*" = match one wildcard segment.
// Segments are delimited by ".".
func topicMatches(pattern, topic string) bool {
	if pattern == "*" || pattern == topic {
		return true
	}
	// Split both into segments and match segment-by-segment with wildcard support
	pSegs := strings.Split(pattern, ".")
	tSegs := strings.Split(topic, ".")

	if len(pSegs) != len(tSegs) {
		// If pattern ends with "**", match any remaining segments
		if len(pSegs) > 0 && pSegs[len(pSegs)-1] == "**" {
			return len(tSegs) >= len(pSegs)-1 && segmentsMatch(pSegs[:len(pSegs)-1], tSegs[:len(pSegs)-1])
		}
		return false
	}

	return segmentsMatch(pSegs, tSegs)
}

func segmentsMatch(pattern, topic []string) bool {
	for i, p := range pattern {
		if p != "*" && p != topic[i] {
			return false
		}
	}
	return true
}
