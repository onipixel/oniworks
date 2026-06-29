package realtime_test

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onipixel/oniworks/framework/realtime"
)

// TestSubscribeRespectsChannelAuth is the channel-authorization regression: an
// oni:subscribe to a channel with a handler MUST run that handler's auth logic,
// so a client cannot subscribe to a channel it isn't allowed to (e.g. another
// user's DMs) by bypassing the handler.
func TestSubscribeRespectsChannelAuth(t *testing.T) {
	handler, hub := newTestHub(t)
	// Authorizing handler: only "room.allowed" may be subscribed.
	hub.Channel("room.{name}", func(c *realtime.Conn, e *realtime.Event) error {
		if e.Params["name"] != "allowed" {
			return fmt.Errorf("unauthorized channel")
		}
		return nil
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	defer conn.Close()
	readEvent(t, conn, 2*time.Second) // consume connect ack

	// Subscribe to a forbidden channel → the handler rejects → error frame, no sub.
	sendEvent(t, conn, &realtime.Event{Type: realtime.EventTypeSubscribe, Channel: "room.secret"})
	if e := readEvent(t, conn, 2*time.Second); e.Type != realtime.EventTypeError {
		t.Fatalf("unauthorized subscribe should return an error frame, got %q", e.Type)
	}

	// Subscribe to the allowed channel → ack, then receive a broadcast.
	sendEvent(t, conn, &realtime.Event{Type: realtime.EventTypeSubscribe, Channel: "room.allowed"})
	if e := readEvent(t, conn, 2*time.Second); e.Type != realtime.EventTypeAck {
		t.Fatalf("allowed subscribe should ack, got %q", e.Type)
	}
	if err := hub.Broadcast("room.allowed", map[string]any{"hello": 1}); err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if e := readEvent(t, conn, 2*time.Second); e.Channel != "room.allowed" {
		t.Fatalf("expected a broadcast on room.allowed, got %+v", e)
	}
}

// TestSubscribeOpenChannel verifies a channel with NO handler is open: an
// oni:subscribe succeeds and the connection receives broadcasts.
func TestSubscribeOpenChannel(t *testing.T) {
	handler, hub := newTestHub(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	defer conn.Close()
	readEvent(t, conn, 2*time.Second) // ack

	sendEvent(t, conn, &realtime.Event{Type: realtime.EventTypeSubscribe, Channel: "public.feed"})
	if e := readEvent(t, conn, 2*time.Second); e.Type != realtime.EventTypeAck {
		t.Fatalf("open-channel subscribe should ack, got %q", e.Type)
	}
	if err := hub.Broadcast("public.feed", map[string]any{"n": 1}); err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if e := readEvent(t, conn, 2*time.Second); e.Channel != "public.feed" {
		t.Fatalf("expected broadcast on public.feed, got %+v", e)
	}
}
