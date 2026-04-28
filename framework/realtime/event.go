// Package realtime is the OniWorks realtime platform — the nervous system of the framework.
// It provides a WebSocket connection manager, channel/room router, broadcast via Oni Memory,
// presence tracking, backpressure, auth per connection, and reconnect/resume support.
package realtime

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// Event is the wire-format envelope for all messages flowing over a WebSocket connection.
//
// JSON wire format:
//
//	{"id":"01J...","type":"chat.message","channel":"chat.room1","payload":{...},"ts":1713000000}
type Event struct {
	// ID is a server-assigned monotonic event ID used for reconnect/resume.
	ID string `json:"id,omitempty"`

	// Type classifies the event (e.g. "chat.message", "presence.join").
	Type string `json:"type"`

	// Channel is the pub/sub channel this event belongs to.
	Channel string `json:"channel,omitempty"`

	// Payload is the event body — arbitrary JSON.
	Payload json.RawMessage `json:"payload,omitempty"`

	// Params are wildcard segments extracted by the channel router (not serialized).
	Params map[string]string `json:"-"`

	// ConnID is the sender's connection ID (not sent to clients).
	ConnID string `json:"-"`

	// UserID is the authenticated user's ID (0 = anonymous).
	UserID int64 `json:"-"`

	// Ts is a Unix timestamp in seconds.
	Ts int64 `json:"ts,omitempty"`
}

// eventCounter generates monotonically increasing event IDs.
var eventCounter uint64

// newEventID returns a short lexicographically sortable ID.
func newEventID() string {
	ts := time.Now().UnixMilli()
	n := atomic.AddUint64(&eventCounter, 1)
	return fmt.Sprintf("%013x%08x", ts, n)
}

// NewEvent creates an Event with an auto-generated ID and current timestamp.
func NewEvent(eventType, channel string, payload any) (*Event, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("realtime: marshal payload: %w", err)
	}
	return &Event{
		ID:      newEventID(),
		Type:    eventType,
		Channel: channel,
		Payload: json.RawMessage(b),
		Ts:      time.Now().Unix(),
	}, nil
}

// Decode unmarshals the event payload into dest.
func (e *Event) Decode(dest any) error {
	return json.Unmarshal(e.Payload, dest)
}

// Encode serializes the event to JSON bytes for transmission.
func (e *Event) Encode() ([]byte, error) {
	if e.Ts == 0 {
		e.Ts = time.Now().Unix()
	}
	return json.Marshal(e)
}

// System event types (reserved — prefixed with "oni:")
const (
	EventTypeConnect    = "oni:connect"
	EventTypeDisconnect = "oni:disconnect"
	EventTypeError      = "oni:error"
	EventTypePing       = "oni:ping"
	EventTypePong       = "oni:pong"
	EventTypeResume     = "oni:resume"
	EventTypeAck        = "oni:ack"
)
