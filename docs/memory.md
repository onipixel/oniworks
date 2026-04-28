# Oni Memory — Distributed In-Memory Database

Oni Memory is a full distributed in-memory KV store with TTL, pub/sub, atomic operations, and cross-node synchronization. It is built directly into OniWorks — no Redis required by default.

## What It's For

| Use case | Example key |
|----------|-------------|
| Sessions | `session:<id>` |
| Cache | `user:<id>:profile` |
| Realtime state | `user:<id>:status` |
| Presence | `presence:chat.room1:<connID>` |
| Rate limiting | `rl:<ip>` |
| Feature flags | `flag:dark-mode` |
| Pub/Sub event bus | `notify:<userID>` |

> **Note:** Oni Memory is an enhancer, not a replacement for PostgreSQL. Use it for speed-critical, ephemeral, or realtime state. Persist truth to the database.

## Setup

```go
mem := memory.New(memory.Options{
    NodeID:      "node-1",              // unique per process
    BindAddr:    ":7946",               // TCP gossip address
    Peers:       []string{":7947"},     // other nodes
    Persist:     true,                  // snapshot to disk
    SnapshotPath: "./storage/mem.snap",
    GracefulSave: true,                 // save on SIGTERM
})
```

## KV Operations

```go
// Set with TTL (0 = no expiry)
mem.Set("session:abc123", sessionData, 30*time.Minute)

// Get
val, ok := mem.Get("session:abc123")

// Atomic increment (for rate limiting, counters)
n := mem.Incr("counter:page_views")

// Delete
mem.Delete("session:abc123")
```

## Pub/Sub

```go
// Subscribe
cancel := mem.Subscribe("orders.*", func(topic string, payload any) {
    fmt.Println("new order:", topic, payload)
})
defer cancel()  // unsubscribe

// Publish (fan-out to all subscribers, including cross-node)
mem.Publish("orders.created", order)
```

Wildcards: `*` matches one segment, `**` matches any depth.

## Cross-Node Sync

By default, Oni Memory uses a built-in TCP gossip protocol for cross-node sync. Set/Delete/Publish operations are broadcast to all peers.

**Conflict resolution:** Last-write-wins with vector clocks.

### Optional: Use Redis for sync

If you prefer Redis pub/sub for cross-node sync (e.g., you already run Redis):

```go
mem := memory.New(memory.Options{
    RedisURL: "redis://localhost:6379",  // enables Redis sync adapter
    // BindAddr and Peers are ignored when RedisURL is set
})
```

## Snapshot & Restore

On graceful shutdown, Oni Memory writes a snapshot to `SnapshotPath`. On next startup it restores automatically:

```go
mem := memory.New(memory.Options{
    Persist:     true,
    SnapshotPath: "./storage/mem.snap",
    GracefulSave: true,
})
```

## Eviction

Expired keys are lazily removed on access and periodically cleaned up:

```go
mem := memory.New(memory.Options{
    MaxKeys:       100_000,          // cap total keys
    EvictInterval: 5 * time.Minute, // how often to run eviction
})
```
