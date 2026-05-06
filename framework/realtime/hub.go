package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/onipixel/oniworks/framework/memory"
)

// ConnectHandler is called after a WebSocket connection is established and authenticated.
type ConnectHandler func(c *Conn) error

// DisconnectHandler is called when a WebSocket connection closes.
type DisconnectHandler func(c *Conn)

// Options configures the Hub.
type Options struct {
	// Memory is the Oni Memory store used for cross-node broadcast and presence.
	Memory *memory.Store

	// CheckOrigin validates the WebSocket origin header. Return true to allow.
	// Defaults to allowing all origins (override in production).
	CheckOrigin func(r *http.Request) bool

	// AuthFunc authenticates a connection during the HTTP upgrade handshake.
	// Return the user ID (0 = anonymous) and an error to reject the connection.
	AuthFunc func(r *http.Request) (userID int64, err error)

	// EventBufferSize is the max number of events buffered per channel for resume.
	EventBufferSize int

	// EventBufferAge is how long events stay in the buffer (default: 2 minutes).
	EventBufferAge time.Duration

	// PresenceChannels lists channel patterns that auto-track presence.
	PresenceChannels []string

	// Logger (defaults to slog.Default).
	Logger *slog.Logger
}

// Hub is the central OniWorks realtime hub.
// It manages all WebSocket connections, routes events to channel handlers,
// broadcasts via Oni Memory (cross-node safe), and tracks presence.
type Hub struct {
	opts     Options
	mem      *memory.Store
	router   *eventRouter
	buffer   *EventBuffer
	presence *PresenceManager

	mu    sync.RWMutex
	conns map[string]*Conn // connID → *Conn

	connectHandlers    []ConnectHandler
	disconnectHandlers []DisconnectHandler
	presencePatterns   []string

	// memorySub is the cancel function for our Oni Memory subscription.
	// Hub subscribes to "oni:broadcast:*" to receive cross-node events.
	memorySub func()

	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
}

// New creates a Hub with the given options.
func New(opts Options) *Hub {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.EventBufferSize <= 0 {
		opts.EventBufferSize = 512
	}
	if opts.EventBufferAge <= 0 {
		opts.EventBufferAge = 2 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		opts:     opts,
		mem:      opts.Memory,
		router:   newEventRouter(),
		buffer:   NewEventBuffer(opts.EventBufferSize, opts.EventBufferAge),
		conns:    make(map[string]*Conn),
		ctx:      ctx,
		cancel:   cancel,
		logger:   opts.Logger,
	}

	if opts.Memory != nil {
		h.presence = newPresenceManager(opts.Memory)
		// Subscribe to cross-node broadcasts from peer nodes
		h.memorySub = opts.Memory.Subscribe("oni:broadcast:**", h.handleRemoteBroadcast)
	}

	if opts.CheckOrigin != nil {
		upgrader.CheckOrigin = opts.CheckOrigin
	}

	return h
}

// ─────────────────────────── Route registration ───────────────────

// Channel registers a handler for events arriving on a channel pattern.
//
//	hub.Channel("chat.{room}", func(c *Conn, e *Event) error {
//	    return hub.Broadcast("chat."+e.Params["room"], e.Payload)
//	})
func (h *Hub) Channel(pattern string, fn HandlerFunc) {
	h.router.Register(pattern, fn)
}

// Presence marks a channel pattern as a presence channel.
// When a client subscribes/unsubscribes, their entry is written to Oni Memory.
//
//	hub.Presence("room.{id}")
func (h *Hub) Presence(pattern string) {
	h.presencePatterns = append(h.presencePatterns, pattern)
}

// OnConnect registers a hook called after every successful connection.
func (h *Hub) OnConnect(fn ConnectHandler) {
	h.connectHandlers = append(h.connectHandlers, fn)
}

// OnDisconnect registers a hook called when a connection closes.
func (h *Hub) OnDisconnect(fn DisconnectHandler) {
	h.disconnectHandlers = append(h.disconnectHandlers, fn)
}

// ─────────────────────────── HTTP handler ─────────────────────────

// ServeHTTP upgrades an HTTP request to a WebSocket connection.
// Mount this on your router:
//
//	router.Get("/ws", hub.ServeHTTP)
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Auth per connection
	var userID int64
	if h.opts.AuthFunc != nil {
		uid, err := h.opts.AuthFunc(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID = uid
	}

	// Upgrade
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("realtime: upgrade failed", "error", err)
		return
	}

	// Resume support: client sends last event ID as query param
	lastEventID := r.URL.Query().Get("last_event_id")

	conn := newConn(ws, h, userID, lastEventID)
	h.register(conn)

	// Run connect hooks
	for _, fn := range h.connectHandlers {
		if err := fn(conn); err != nil {
			h.logger.Warn("realtime: connect hook error", "conn", conn.id, "error", err)
			conn.sendError(err.Error())
			h.unregister(conn)
			return
		}
	}

	// Replay missed events if client provided a lastEventID
	if lastEventID != "" {
		h.replayMissed(conn, lastEventID)
	}

	// Send connection ack
	ack := &Event{
		Type:    EventTypeConnect,
		Payload: json.RawMessage(fmt.Sprintf(`{"conn_id":%q,"user_id":%d}`, conn.id, userID)),
		Ts:      time.Now().Unix(),
	}
	conn.Send(ack)

	// Start I/O loops
	go conn.writeLoop()
	conn.readLoop() // blocks until connection closes
}

// ─────────────────────────── Broadcast ───────────────────────────

// Broadcast sends an event to all connections subscribed to channel,
// including connections on other nodes (via Oni Memory pub/sub).
//
//	hub.Broadcast("chat.room1", myPayload)
func (h *Hub) Broadcast(channel string, payload any) error {
	e, err := NewEvent("broadcast", channel, payload)
	if err != nil {
		return err
	}
	e.ID = newEventID()

	// Buffer for reconnect resume
	h.buffer.Push(channel, e)

	// Push to local connections
	h.pushToChannel(channel, e)

	// Publish to Oni Memory for cross-node delivery
	if h.mem != nil {
		b, _ := e.Encode()
		h.mem.Publish("oni:broadcast:"+channel, b)
	}
	return nil
}

// BroadcastEvent broadcasts a pre-built Event to a channel.
func (h *Hub) BroadcastEvent(channel string, e *Event) error {
	if e.ID == "" {
		e.ID = newEventID()
	}
	h.buffer.Push(channel, e)
	h.pushToChannel(channel, e)
	if h.mem != nil {
		b, _ := e.Encode()
		h.mem.Publish("oni:broadcast:"+channel, b)
	}
	return nil
}

// Push sends an event directly to a specific connection by ID.
func (h *Hub) Push(connID string, e *Event) bool {
	h.mu.RLock()
	c, ok := h.conns[connID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	return c.Send(e)
}

// pushToChannel delivers e to all local connections subscribed to channel.
func (h *Hub) pushToChannel(channel string, e *Event) {
	h.mu.RLock()
	conns := make([]*Conn, 0, len(h.conns))
	for _, c := range h.conns {
		if c.IsSubscribed(channel) {
			conns = append(conns, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range conns {
		c.Send(e)
	}
}

// handleRemoteBroadcast is the Oni Memory subscription handler for cross-node broadcasts.
func (h *Hub) handleRemoteBroadcast(topic string, payload any) {
	// topic = "oni:broadcast:<channel>"
	channel := ""
	if len(topic) > len("oni:broadcast:") {
		channel = topic[len("oni:broadcast:"):]
	}

	var b []byte
	switch v := payload.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			return
		}
	}

	var e Event
	if err := json.Unmarshal(b, &e); err != nil {
		return
	}
	e.Channel = channel

	// Push to local connections only (avoid re-broadcasting to Memory)
	h.pushToChannel(channel, &e)
}

// ─────────────────────────── Presence ────────────────────────────

// RoomSize returns the number of connections subscribed to a channel.
// Uses Oni Memory for cross-node accuracy.
func (h *Hub) RoomSize(channel string) int {
	if h.presence != nil {
		return h.presence.Count(channel)
	}
	// Fallback: count local connections
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, c := range h.conns {
		if c.IsSubscribed(channel) {
			count++
		}
	}
	return count
}

// Members returns all presence info for a channel (cross-node via Oni Memory).
func (h *Hub) Members(channel string) []PresenceInfo {
	if h.presence != nil {
		return h.presence.Members(channel)
	}
	return nil
}

// ─────────────────────────── Event dispatch ───────────────────────

// dispatch is called by conn.readLoop when a message arrives from a client.
func (h *Hub) dispatch(c *Conn, msg []byte) {
	var e Event
	if err := json.Unmarshal(msg, &e); err != nil {
		c.sendError("invalid event format")
		return
	}
	e.ConnID = c.id
	e.UserID = c.UserID

	// Handle system events
	switch e.Type {
	case EventTypePing:
		pong := &Event{Type: EventTypePong, Ts: time.Now().Unix()}
		c.Send(pong)
		return
	case EventTypeResume:
		if e.Channel != "" {
			h.replayMissed(c, e.ID)
		}
		return
	}

	// Route to channel handler
	if e.Channel != "" {
		handler, params := h.router.Match(e.Channel)
		if handler != nil {
			e.Params = params
			if err := handler(c, &e); err != nil {
				c.sendError(err.Error())
			}
			return
		}
		// Auto-subscribe if client sends to a channel with no explicit handler
		c.Subscribe(e.Channel)
		// Check presence channels
		h.updatePresence(c, e.Channel, true)
	}
}

// replayMissed replays buffered events after lastEventID for a reconnecting client.
func (h *Hub) replayMissed(c *Conn, lastEventID string) {
	// Replay from all channels this connection is subscribed to
	c.subsMu.RLock()
	channels := make([]string, 0, len(c.subs))
	for ch := range c.subs {
		channels = append(channels, ch)
	}
	c.subsMu.RUnlock()

	for _, ch := range channels {
		missed := h.buffer.Since(ch, lastEventID)
		for _, e := range missed {
			c.Send(e)
		}
	}
}

// ─────────────────────────── Connection lifecycle ─────────────────

func (h *Hub) register(c *Conn) {
	h.mu.Lock()
	h.conns[c.id] = c
	h.mu.Unlock()
	h.logger.Debug("realtime: connection registered", "conn", c.id, "user", c.UserID)
}

func (h *Hub) unregister(c *Conn) {
	h.mu.Lock()
	delete(h.conns, c.id)
	h.mu.Unlock()

	// Remove from all presence channels
	if h.presence != nil {
		h.presence.LeaveAll(c.id)
	}

	for _, fn := range h.disconnectHandlers {
		fn(c)
	}
	h.logger.Debug("realtime: connection unregistered", "conn", c.id, "user", c.UserID)
}

// updatePresence writes presence info to Oni Memory for presence-tracked channels.
func (h *Hub) updatePresence(c *Conn, channel string, joining bool) {
	if h.presence == nil {
		return
	}
	for _, pattern := range h.presencePatterns {
		route := &channelRoute{pattern: pattern, segments: parseChannelPattern(pattern)}
		if _, matched := route.match(channel); matched {
			if joining {
				h.presence.Join(channel, PresenceInfo{
					UserID: c.UserID,
					ConnID: c.id,
				})
				c.Subscribe(channel)
			} else {
				h.presence.Leave(channel, c.id)
			}
			return
		}
	}
}

// Shutdown closes all connections and stops the hub.
func (h *Hub) Shutdown() {
	h.cancel()
	if h.memorySub != nil {
		h.memorySub()
	}
	h.mu.Lock()
	for _, c := range h.conns {
		c.Close()
	}
	h.mu.Unlock()
}

// ConnCount returns the number of active connections on this node.
func (h *Hub) ConnCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
