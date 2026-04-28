package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB
	sendBufSize    = 256        // buffered send channel capacity
)

// connCounter generates unique numeric connection IDs.
var connCounter uint64

// Conn represents a single WebSocket connection to a client.
// It runs two goroutines: a read loop and a write loop.
// The send channel provides backpressure — slow clients are dropped after write timeout.
type Conn struct {
	id     string
	ws     *websocket.Conn
	hub    *Hub
	send   chan []byte
	ctx    context.Context
	cancel context.CancelFunc

	// Authenticated user (0 = anonymous)
	UserID int64

	// lastEventID is the ID of the last event the client acknowledged.
	// Populated from the Sec-WebSocket-Protocol or query param on connect.
	lastEventID string

	// subscriptions is the set of channel patterns this connection is subscribed to.
	subsMu sync.RWMutex
	subs   map[string]bool

	// metadata is arbitrary per-connection data set by auth/middleware hooks.
	metaMu sync.RWMutex
	meta   map[string]any
}

func newConn(ws *websocket.Conn, hub *Hub, userID int64, lastEventID string) *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	n := atomic.AddUint64(&connCounter, 1)
	return &Conn{
		id:          fmt.Sprintf("conn-%x", n),
		ws:          ws,
		hub:         hub,
		send:        make(chan []byte, sendBufSize),
		ctx:         ctx,
		cancel:      cancel,
		UserID:      userID,
		lastEventID: lastEventID,
		subs:        make(map[string]bool),
		meta:        make(map[string]any),
	}
}

// ID returns the unique connection identifier.
func (c *Conn) ID() string { return c.id }

// Set stores a value in the per-connection metadata bag.
func (c *Conn) Set(key string, value any) {
	c.metaMu.Lock()
	c.meta[key] = value
	c.metaMu.Unlock()
}

// Get retrieves a value from the per-connection metadata bag.
func (c *Conn) Get(key string) (any, bool) {
	c.metaMu.RLock()
	v, ok := c.meta[key]
	c.metaMu.RUnlock()
	return v, ok
}

// Subscribe marks this connection as interested in a channel pattern.
func (c *Conn) Subscribe(channel string) {
	c.subsMu.Lock()
	c.subs[channel] = true
	c.subsMu.Unlock()
}

// Unsubscribe removes a channel subscription.
func (c *Conn) Unsubscribe(channel string) {
	c.subsMu.Lock()
	delete(c.subs, channel)
	c.subsMu.Unlock()
}

// IsSubscribed reports whether this connection is subscribed to a channel.
func (c *Conn) IsSubscribed(channel string) bool {
	c.subsMu.RLock()
	ok := c.subs[channel]
	c.subsMu.RUnlock()
	return ok
}

// Send queues an event for delivery. Non-blocking — drops if buffer is full.
func (c *Conn) Send(e *Event) bool {
	b, err := e.Encode()
	if err != nil {
		return false
	}
	select {
	case c.send <- b:
		return true
	default:
		slog.Warn("realtime: send buffer full, dropping event", "conn", c.id, "type", e.Type)
		return false
	}
}

// Close terminates the connection gracefully.
func (c *Conn) Close() {
	c.cancel()
}

// readLoop reads messages from the WebSocket and dispatches them to the hub.
func (c *Conn) readLoop() {
	defer func() {
		c.hub.unregister(c)
		c.cancel()
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("realtime: unexpected close", "conn", c.id, "error", err)
			}
			return
		}
		c.hub.dispatch(c, msg)
	}
}

// writeLoop flushes the send queue to the WebSocket.
func (c *Conn) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			_ = c.ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return

		case msg, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendError sends an error event to the client.
func (c *Conn) sendError(msg string) {
	e := &Event{
		Type:    EventTypeError,
		Payload: json.RawMessage(`{"error":` + fmt.Sprintf("%q", msg) + `}`),
		Ts:      time.Now().Unix(),
	}
	b, _ := e.Encode()
	select {
	case c.send <- b:
	default:
	}
}

// ─────────────────────────── upgrader ─────────────────────────────

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Override in Hub.Options.CheckOrigin for production
		return true
	},
}
