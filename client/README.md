# @oniworks/socket — Oni Socket TypeScript client

The official, dependency-free TypeScript client for OniWorks realtime.

```bash
npm install @oniworks/socket
```

## Usage

```typescript
import { OniSocket } from "@oniworks/socket";

const socket = new OniSocket("/ws", { token: jwt });

// Subscribe and listen
socket.channel("chat.general")
  .on("chat.message", (e) => appendMessage(e.payload));

// Send
socket.channel("chat.general").send("chat.message", { text: "hey!" });

// Presence
socket.channel("room.42")
  .joining((e) => console.log("joined:", e.payload))
  .leaving((e) => console.log("left:", e.payload));

// Leave
socket.channel("chat.general").leave();
```

## Features

- **Channels** — `socket.channel(name).on(type, cb)` / `.send(type, payload)` / `.leave()`
- **Typed events** — `on<MyPayload>("type", e => e.payload /* MyPayload */)`
- **Auto-reconnect** — exponential backoff (configurable)
- **Resume** — tracks `last_event_id` and replays missed events on reconnect (no message loss on brief disconnects)
- **Heartbeat** — periodic ping/pong keepalive
- **Presence** — `joining()` / `leaving()` helpers
- **Node/test friendly** — inject a `WebSocket` implementation via `options.WebSocket`

## Options

| Option | Default | Description |
|---|---|---|
| `token` | – | Sent as `?token=…` on the connection URL (JWT/session) |
| `reconnect` | `true` | Reconnect on unexpected close |
| `reconnectBaseMs` | `500` | Initial backoff delay |
| `reconnectMaxMs` | `10000` | Max backoff delay |
| `heartbeatMs` | `25000` | Ping interval (`0` to disable) |
| `WebSocket` | global | WebSocket implementation (for Node/testing) |
| `onOpen` / `onClose` / `onError` / `onStateChange` | – | Lifecycle callbacks |

## Wire protocol

Mirrors `framework/realtime`. Events are JSON envelopes
`{ id?, type, channel?, payload?, ts? }`. The client sends `oni:subscribe` /
`oni:unsubscribe` to manage channel membership, `oni:ping` for keepalive, and
`oni:resume` (with the last event id) after reconnecting.

## Develop

```bash
npm run typecheck   # tsc --noEmit
npm run build       # emit dist/
npm test            # node --test (mock WebSocket)
```
