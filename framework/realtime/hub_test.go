package realtime_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/onipixel/oniworks/framework/realtime"
)

// dialWS connects a gorilla WebSocket client to a test server URL.
func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(url, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	return conn
}

// readEvent reads one JSON event from a WebSocket connection.
func readEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) *realtime.Event {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var e realtime.Event
	if err := json.Unmarshal(msg, &e); err != nil {
		t.Fatalf("unmarshal event: %v (raw: %s)", err, msg)
	}
	return &e
}

// sendEvent sends a JSON event over a WebSocket connection.
func sendEvent(t *testing.T, conn *websocket.Conn, e *realtime.Event) {
	t.Helper()
	b, _ := json.Marshal(e)
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

// newTestHub creates a Hub and returns its HTTP handler.
func newTestHub(t *testing.T, opts ...func(*realtime.Options)) (http.HandlerFunc, *realtime.Hub) {
	t.Helper()
	o := realtime.Options{}
	for _, fn := range opts {
		fn(&o)
	}
	hub := realtime.New(o)
	t.Cleanup(hub.Shutdown)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeHTTP(w, r)
	})
	return handler, hub
}

// ─── Tests ─────────────────────────────────────────────────────────

func TestUpgradeAndConnectAck(t *testing.T) {
	handler, _ := newTestHub(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	defer conn.Close()

	// First message should be the connect ack
	e := readEvent(t, conn, 3*time.Second)
	if e.Type != realtime.EventTypeConnect {
		t.Errorf("first event type: got %q, want %q", e.Type, realtime.EventTypeConnect)
	}
}

func TestPingPong(t *testing.T) {
	handler, _ := newTestHub(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	defer conn.Close()

	// Consume connect ack
	readEvent(t, conn, 2*time.Second)

	// Send ping
	sendEvent(t, conn, &realtime.Event{Type: realtime.EventTypePing})

	// Read pong
	e := readEvent(t, conn, 2*time.Second)
	if e.Type != realtime.EventTypePong {
		t.Errorf("expected pong, got %q", e.Type)
	}
}

func TestChannelHandler(t *testing.T) {
	handler, hub := newTestHub(t)

	fired := make(chan string, 1)
	hub.Channel("test.{room}", func(c *realtime.Conn, e *realtime.Event) error {
		fired <- e.Params["room"]
		return nil
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	defer conn.Close()
	readEvent(t, conn, 2*time.Second) // consume ack

	sendEvent(t, conn, &realtime.Event{
		Type:    "message",
		Channel: "test.myroom",
	})

	select {
	case room := <-fired:
		if room != "myroom" {
			t.Errorf("room param: got %q, want %q", room, "myroom")
		}
	case <-time.After(2 * time.Second):
		t.Error("channel handler was not called")
	}
}

func TestBroadcast(t *testing.T) {
	handler, hub := newTestHub(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Connect a subscriber
	conn := dialWS(t, srv.URL)
	defer conn.Close()
	readEvent(t, conn, 2*time.Second) // consume ack

	// Subscribe to a channel via sending an event to it (auto-subscribe)
	sendEvent(t, conn, &realtime.Event{
		Type:    "join",
		Channel: "news.feed",
	})
	time.Sleep(50 * time.Millisecond)

	// Broadcast from server side
	if err := hub.Broadcast("news.feed", map[string]any{"headline": "OniWorks released!"}); err != nil {
		t.Fatalf("Broadcast: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage after broadcast: %v", err)
	}
	if !strings.Contains(string(msg), "OniWorks released") {
		t.Errorf("expected broadcast payload in message: %s", msg)
	}
}

func TestConnCount(t *testing.T) {
	handler, hub := newTestHub(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	if hub.ConnCount() != 0 {
		t.Error("initially 0 connections")
	}

	conn1 := dialWS(t, srv.URL)
	conn2 := dialWS(t, srv.URL)
	time.Sleep(50 * time.Millisecond) // allow goroutines to register

	if hub.ConnCount() != 2 {
		t.Errorf("expected 2 connections, got %d", hub.ConnCount())
	}

	conn1.Close()
	conn2.Close()
	time.Sleep(100 * time.Millisecond)

	if hub.ConnCount() != 0 {
		t.Errorf("expected 0 after close, got %d", hub.ConnCount())
	}
}

func TestOnConnectHook(t *testing.T) {
	handler, hub := newTestHub(t)
	connected := make(chan string, 1)

	hub.OnConnect(func(c *realtime.Conn) error {
		connected <- c.ID()
		return nil
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	defer conn.Close()

	select {
	case id := <-connected:
		if id == "" {
			t.Error("expected non-empty connection ID")
		}
	case <-time.After(2 * time.Second):
		t.Error("OnConnect hook was not called")
	}
}

func TestOnDisconnectHook(t *testing.T) {
	handler, hub := newTestHub(t)
	disconnected := make(chan struct{}, 1)

	hub.OnDisconnect(func(c *realtime.Conn) {
		disconnected <- struct{}{}
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	readEvent(t, conn, 2*time.Second) // ack
	conn.Close()

	select {
	case <-disconnected:
	case <-time.After(2 * time.Second):
		t.Error("OnDisconnect hook was not called")
	}
}
