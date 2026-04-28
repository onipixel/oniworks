# Oni Socket — Realtime Platform

Oni Socket is the realtime nervous system of your OniWorks application. It goes far beyond "chat" — it's a complete event transport, state sync, and presence engine.

## Architecture

```
Client (Browser)
    ↕ WebSocket
Hub (Oni Socket)
    ├── Event Router     — routes events to channel handlers
    ├── Presence Manager — tracks who's in which channel (via Oni Memory)
    ├── Event Buffer     — stores recent events for reconnect/resume
    └── Broadcaster      — fans out to local + remote nodes (via Oni Memory pub/sub)
```

## Setup

```go
hub := realtime.New(realtime.Options{
    Memory: mem,  // Oni Memory for cross-node sync
    AuthFunc: func(r *http.Request) (int64, error) {
        // Return user ID from session/JWT
        return getUserIDFromToken(r), nil
    },
    CheckOrigin: func(r *http.Request) bool {
        return r.Header.Get("Origin") == "https://example.com"
    },
})

// Mount the WebSocket endpoint
router.Get("/ws", func(c *onihttp.Context) error {
    hub.ServeHTTP(c.Response, c.Request.Request)
    return nil
})
```

## Channels

Register handlers for channel patterns. Patterns support `{param}` placeholders:

```go
// chat.room1, chat.anything
hub.Channel("chat.{room}", func(c *realtime.Conn, e *realtime.Event) error {
    room := e.Params["room"]
    return hub.Broadcast("chat."+room, map[string]any{
        "from":    c.UserID,
        "payload": e.Payload,
    })
})
```

## Broadcast

```go
// Broadcast to all subscribers of a channel (cross-node, via Oni Memory)
hub.Broadcast("chat.room1", map[string]any{
    "message": "Hello everyone!",
})

// Push to a specific connection
hub.Push(connID, &realtime.Event{
    Type:    "notification",
    Payload: json.RawMessage(`{"text":"You have a new message"}`),
})
```

## Presence

```go
// Mark a channel as presence-tracked
hub.Presence("room.{id}")

// Get members (cross-node via Oni Memory)
members := hub.Members("room.123")
count   := hub.RoomSize("room.123")
```

## Reconnect & Resume

Clients can pass `?last_event_id=<id>` when reconnecting. The hub replays missed events:

```javascript
const ws = new WebSocket(`wss://example.com/ws?last_event_id=${lastID}`);
```

Events are buffered per-channel (default: 512 events, 2 minutes).

## Connection Hooks

```go
hub.OnConnect(func(c *realtime.Conn) error {
    slog.Info("connected", "user", c.UserID, "conn", c.ID())
    return nil
})

hub.OnDisconnect(func(c *realtime.Conn) {
    slog.Info("disconnected", "user", c.UserID)
})
```

## Per-Connection Metadata

```go
// In OnConnect or a channel handler:
c.Set("locale", "en")
locale, _ := c.Get("locale")
```

## Cross-Node Sync

When Oni Memory is wired in, broadcasts automatically propagate to all nodes:

```
Node A: hub.Broadcast("chat.room1", msg)
    → writes to Oni Memory pub/sub: "oni:broadcast:chat.room1"
    → Node B receives via Memory subscription
    → Node B pushes to its local connections subscribed to "chat.room1"
```

No extra configuration needed.

## Client Example (TypeScript)

```typescript
const ws = new WebSocket('wss://example.com/ws?token=YOUR_TOKEN');

ws.onopen = () => {
    // Subscribe to a room
    ws.send(JSON.stringify({
        type: 'message',
        channel: 'chat.room1',
        payload: { text: 'Hello!' },
    }));
};

ws.onmessage = (e) => {
    const event = JSON.parse(e.data);
    console.log(event.type, event.channel, event.payload);
};
```
