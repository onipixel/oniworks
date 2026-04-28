package realtime

import (
	"sync"
	"time"
)

// EventBuffer stores recent events per channel for reconnect/resume.
// When a client reconnects with a last_event_id, the hub replays all events
// published after that ID — no messages lost during brief disconnects.
type EventBuffer struct {
	mu       sync.RWMutex
	channels map[string]*ringBuffer
	maxSize  int
	maxAge   time.Duration
}

// NewEventBuffer creates a buffer retaining the last maxSize events per channel,
// up to maxAge old.
func NewEventBuffer(maxSize int, maxAge time.Duration) *EventBuffer {
	if maxSize <= 0 {
		maxSize = 512
	}
	if maxAge <= 0 {
		maxAge = 2 * time.Minute
	}
	return &EventBuffer{
		channels: make(map[string]*ringBuffer),
		maxSize:  maxSize,
		maxAge:   maxAge,
	}
}

// Push appends an event to the channel's ring buffer.
func (eb *EventBuffer) Push(channel string, e *Event) {
	eb.mu.Lock()
	rb, ok := eb.channels[channel]
	if !ok {
		rb = newRingBuffer(eb.maxSize)
		eb.channels[channel] = rb
	}
	eb.mu.Unlock()
	rb.push(e)
}

// Since returns all buffered events for channel published after lastEventID.
// Pass "" to get all buffered events.
func (eb *EventBuffer) Since(channel, lastEventID string) []*Event {
	eb.mu.RLock()
	rb, ok := eb.channels[channel]
	eb.mu.RUnlock()
	if !ok {
		return nil
	}
	return rb.since(lastEventID, time.Now().Add(-eb.maxAge))
}

// ─────────────────────────── ring buffer ──────────────────────────

type ringBuffer struct {
	mu     sync.Mutex
	events []*Event
	start  int
	count  int
	cap    int
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		events: make([]*Event, capacity),
		cap:    capacity,
	}
}

func (rb *ringBuffer) push(e *Event) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	pos := (rb.start + rb.count) % rb.cap
	rb.events[pos] = e
	if rb.count == rb.cap {
		rb.start = (rb.start + 1) % rb.cap
	} else {
		rb.count++
	}
}

func (rb *ringBuffer) since(lastEventID string, cutoff time.Time) []*Event {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var result []*Event
	found := lastEventID == ""

	for i := 0; i < rb.count; i++ {
		idx := (rb.start + i) % rb.cap
		e := rb.events[idx]
		if e == nil {
			continue
		}
		if e.Ts > 0 && time.Unix(e.Ts, 0).Before(cutoff) {
			continue
		}
		if !found {
			if e.ID == lastEventID {
				found = true
			}
			continue
		}
		result = append(result, e)
	}
	return result
}
