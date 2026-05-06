<div align="center">

<img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go" alt="Go 1.25+">
<img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="MIT License">
<img src="https://img.shields.io/badge/status-alpha-orange?style=flat" alt="Alpha">
<img src="https://img.shields.io/badge/PostgreSQL-primary-336791?style=flat&logo=postgresql" alt="PostgreSQL">

# OniWorks

**The Go framework that thinks in realtime.**

OniWorks is a batteries-included Go web framework inspired by Laravel, built for the era of live, connected applications. It provides a full MVC stack, a type-safe query builder, authentication, WebSockets, a distributed in-memory store, queues, a scheduler, and a Vite+TypeScript frontend layer — all with a single import.

```
event → socket → memory → broadcast → UI
                    ↓
             persist → PostgreSQL (when needed)
```

**Repository:** [github.com/onipixel/oniworks](https://github.com/onipixel/oniworks) · Maintained by **[OniPixel](https://github.com/onipixel)**

</div>

---

## Why OniWorks?

Most Go web libraries give you routing and leave the rest to you. OniWorks gives you the whole stack:

| Feature | OniWorks |
|---|---|
| HTTP routing + middleware | ✅ |
| Query builder (PostgreSQL + MySQL) | ✅ |
| Migrations + seeders | ✅ |
| Auth (JWT + sessions + CSRF + RBAC) | ✅ |
| Request validation (`c.Validate()`) | ✅ |
| WebSocket realtime (Oni Socket) | ✅ |
| Distributed in-memory store (Oni Memory) | ✅ |
| Queue + background jobs | ✅ |
| Scheduler / cron | ✅ |
| Mail / SMTP | ✅ |
| File storage (local + S3-compatible) | ✅ |
| Auto TLS via Caddy (Oni Deploy) | ✅ |
| CLI code generator (`oni make:*`) | ✅ |
| Admin panel (auto-generated CRUD) | ✅ |
| Prometheus metrics + health checks | ✅ |
| Vite + TypeScript + Tailwind v4 | ✅ |
| TypeScript type generation from Go structs | ✅ |

---

## Quick Start

```bash
# Install the CLI
go install github.com/onipixel/oniworks/cmd/oni@latest

# Create a new project
oni new my-app
cd my-app

# Configure .env, then:
oni migrate
oni serve
```

## Hello World

```go
package main

import (
    "github.com/onipixel/oniworks/framework/app"
    onihttp "github.com/onipixel/oniworks/framework/http"
    "github.com/onipixel/oniworks/framework/routing"
    "github.com/onipixel/oniworks/framework/middleware"
)

func main() {
    oni := app.New().Load(".env")

    oni.Use(middleware.Recovery(), middleware.Logger())

    oni.Route(func(r *routing.Router) {
        r.Get("/", func(c *onihttp.Context) error {
            return c.JSON(200, onihttp.Map{"message": "Hello from OniWorks!"})
        })
    })

    oni.Serve()
}
```

## Realtime in 15 lines

```go
hub := realtime.New(realtime.Options{Memory: mem})

// Server: route channel events
hub.Channel("chat.{room}", func(conn *realtime.Conn, e *realtime.Event) error {
    return hub.Broadcast("chat."+e.Params["room"], e.Payload)
})

// Mount on the router
r.Get("/ws", hub.Handler())
```

```typescript
// Client (TypeScript)
const socket = new OniSocket("/ws")
socket.channel("chat.general").on("chat.message", (e) => appendMessage(e.payload))
socket.channel("chat.general").send("chat.message", { text: "hey!" })
```

---

## Architecture

```
oniworks/
├── cmd/oni/          CLI (oni new, oni serve, oni make:*, oni migrate, ...)
├── framework/
│   ├── app/          Application bootstrap & IoC container
│   ├── http/         Router, context, middleware, validation binding
│   ├── routing/      Route groups, named routes, trie-based dispatch
│   ├── database/     Query builder, NULL-safe scanner, eager loading
│   ├── migrations/   Schema migration engine (Up/Down, batch rollback)
│   ├── seeder/       Database seeders
│   ├── auth/         JWT, sessions, bcrypt, CSRF, per-request auth
│   ├── roles/        RBAC — roles & permissions with wildcard support
│   ├── realtime/     Oni Socket — WebSocket hub, channels, presence
│   ├── memory/       Oni Memory — distributed KV + pub/sub + TTL
│   ├── queue/        Job queue (memory + Redis drivers, exponential backoff)
│   ├── scheduler/    Cron-style task scheduler (robfig/cron)
│   ├── mail/         SMTP mailer with template support
│   ├── storage/      File storage (local + S3-compatible)
│   ├── validation/   Struct-tag validation with custom rules
│   ├── config/       .env + YAML config
│   ├── secrets/      AES-256-GCM encrypted secrets
│   ├── logging/      Structured logger (slog)
│   ├── errors/       Rich debug pages (dev) / clean JSON (prod)
│   ├── health/       Health check endpoints
│   ├── metrics/      Prometheus metrics
│   ├── admin/        Auto-generated CRUD admin panel
│   ├── deploy/       Caddy integration + auto TLS
│   ├── backup/       Database backup & restore
│   └── frontend/     Vite manifest loader + TypeScript type generator
├── testing/
│   ├── stress/       Load test suite (HTTP, Memory, PostgreSQL, WebSocket)
│   └── onigram/      Integration test application
├── stubs/            Code generation templates
├── examples/         Example applications
└── sample/
    ├── oniworks-site/ Documentation website
    └── onimail/       OniMail — self-hosted email platform built with OniWorks
```

---

## Core Concepts

### Query Builder

Lazy, fluent, NULL-safe. Nothing executes until a terminal method is called:

```go
// Single row — returns ErrNotFound if no match
var user User
err := db.Table("users").Where("id = ?", id).First(&user)

// Multiple rows with eager loading (no N+1)
var posts []Post
err := db.Table("posts").
    With("Author", "Tags").
    Where("published = ?", true).
    OrderBy("created_at DESC").
    All(&posts)

// Pagination
var posts []Post
page, err := db.Table("posts").
    Where("user_id = ?", userID).
    Paginate(1, 20, &posts)
// page.Total, page.LastPage, page.From, page.To

// Soft deletes
err := db.Table("posts").SoftDelete().Where("user_id = ?", id).All(&posts)

// Transactions with savepoints (auto-rollback on panic)
err := db.Transaction(func(tx *database.DB) error {
    tx.Table("accounts").Where("id = ?", from).Update(database.Map{"balance": newBal})
    return tx.Table("accounts").Where("id = ?", to).Update(database.Map{"balance": newBal2})
})
```

### Request Validation

Bind and validate in one call:

```go
func createUser(c *onihttp.Context) error {
    var in struct {
        Name  string `json:"name"  validate:"required,min=2"`
        Email string `json:"email" validate:"required,email"`
    }
    if err := c.Validate(&in); err != nil {
        return err // 422 {"message":"validation failed","errors":{...}}
    }
    // in.Name and in.Email are valid
}
```

### Oni Socket — Realtime Platform

Oni Socket is not just a WebSocket wrapper — it is a full realtime platform:

- **Channel routing** with `{param}` captures
- **Presence** — know who is online per channel
- **Backpressure** — bounded send buffer per connection, slow clients dropped gracefully
- **Per-connection auth** (JWT or session, during HTTP upgrade)
- **Reconnect/resume** with server-side event buffer and `last_event_id` replay
- **Cross-node broadcast** via Oni Memory pub/sub (no Redis required by default)

```go
hub := realtime.New(realtime.Options{
    Memory: mem,
    AuthFunc: func(r *http.Request) (int64, error) {
        // return userID from session/JWT
    },
})

hub.Channel("orders.{userID}", func(conn *realtime.Conn, e *realtime.Event) error {
    return hub.Push(conn.ID(), &realtime.Event{Type: "order.update", Payload: e.Payload})
})

hub.Presence("room.{id}")  // tracks who's online per room

r.Get("/ws", hub.Handler())
```

### Oni Memory — Distributed In-Memory Store

```go
mem := memory.New(memory.Options{
    Persist:  true,         // snapshot on shutdown
    BindAddr: ":7946",      // enable TCP gossip for multi-node
})

mem.Set("session:abc", data, 2*time.Hour)
mem.Incr("page:views")                          // atomic counter
mem.CompareAndSwap("lock", nil, "held", 30*time.Second)

cancel := mem.Subscribe("orders.*", func(topic string, payload any) {
    // fan-out pub/sub — 7M+ effective messages/sec
})
```

---

## Performance (stress test, 50 workers, Go 1.25, Windows)

| Component | Test | Throughput |
|---|---|---|
| HTTP Router | GET (in-process) | **1.8M req/s** |
| HTTP Router | POST + JSON bind | **1.1M req/s** |
| HTTP Router | TCP round-trip | **8K req/s** |
| Oni Memory | Get (10K keys) | **8.1M ops/s** |
| Oni Memory | Incr (50-way contention) | **2.3M ops/s** |
| Oni Memory | Pub/Sub (20 subscribers) | **7.2M msg/s effective** |
| PostgreSQL | SELECT by PK | **194K qps** |
| PostgreSQL | INSERT | **59K qps** |
| PostgreSQL | Transaction (R+W) | **17K tx/s** |
| WebSocket | Round-trip echo | **590K msg/s** |
| WebSocket | Broadcast delivery | **100% to 50 conns** |

Run the stress suite: `go run ./testing/stress --workers 50 --duration 8s`

---

## CLI

```bash
oni new <name>                  Create a new project
oni new <name> --frontend       Create with Vite + TypeScript + Tailwind

oni serve                       Start dev server (uses Air if installed)
oni build                       Compile production binary
oni deploy --domain myapp.com   Deploy with Let's Encrypt TLS

# Generators
oni make:controller <Name>      HTTP controller
oni make:model <Name>           Database model
oni make:migration <name>       Migration file
oni make:middleware <Name>      Middleware
oni make:job <Name>             Background job
oni make:mail <Name>            Mailer
oni make:seeder <Name>          Database seeder
oni make:channel <Name>         WebSocket channel handler
oni make:resource <Name>        Controller + model + migration

# Database
oni migrate                     Run pending migrations
oni migrate:rollback            Rollback last batch
oni migrate:fresh               Drop all + re-run
oni migrate:status              Show status table
oni db:seed                     Run seeders

# Operations
oni queue:work                  Start queue worker
oni schedule:run                Run scheduled tasks
oni route:list                  List all routes
oni key:generate                Generate APP_KEY
oni health                      Run health checks
```

---

## Documentation

Full documentation is in [`sample/oniworks-site/content/docs/`](sample/oniworks-site/content/docs/):

| Guide | |
|---|---|
| [Getting Started](sample/oniworks-site/content/docs/getting-started.md) | Install, scaffold, run |
| [Routing](sample/oniworks-site/content/docs/routing.md) | Routes, params, middleware, named routes |
| [Database](sample/oniworks-site/content/docs/database.md) | Query builder, eager loading, migrations |
| [Query Builder](sample/oniworks-site/content/docs/query-builder.md) | Full builder API reference |
| [Authentication](sample/oniworks-site/content/docs/auth.md) | Session auth, JWT, CSRF, RBAC |
| [Validation](sample/oniworks-site/content/docs/validation.md) | `c.Validate()`, struct tags, custom rules |
| [Oni Socket](sample/oniworks-site/content/docs/realtime.md) | WebSockets, channels, presence |
| [Oni Memory](sample/oniworks-site/content/docs/memory.md) | KV store, pub/sub, distributed mode |
| [Queue & Jobs](sample/oniworks-site/content/docs/queue.md) | Background jobs, retries, drivers |
| [Middleware](sample/oniworks-site/content/docs/middleware.md) | Built-in and custom middleware |
| [CLI Reference](sample/oniworks-site/content/docs/cli.md) | All `oni` commands |
| [Testing](sample/oniworks-site/content/docs/testing.md) | HTTP test helpers, assertions |
| [Deployment](sample/oniworks-site/content/docs/deployment.md) | Production + auto TLS |

---

## OniMail — Built with OniWorks

[OniMail](sample/onimail/) is a production self-hosted email platform built entirely with the OniWorks framework. It demonstrates real-world usage across every framework layer:

- Full SMTP server (inbound + outbound with DKIM/SPF/DMARC)
- Full IMAP4rev2 server with IDLE push
- Gmail-quality webmail UI (TypeScript + Tailwind)
- Real-time inbox via Oni Socket WebSocket
- Marketing campaigns with open/click tracking
- Multi-domain support with admin panel

```bash
cd sample/onimail
go build -o onimail ./cmd/onimail
./onimail install --domain mail.example.com --admin admin@example.com
./onimail migrate
./onimail serve
```

---

## Status

OniWorks v1.1 — all core features stable.

- [x] Core HTTP layer + router
- [x] Query builder + NULL-safe scanner + eager loading
- [x] Pagination + soft deletes
- [x] Request validation binding (`c.Validate()`)
- [x] Auth (JWT + sessions + CSRF + RBAC)
- [x] Oni Socket (WebSocket realtime)
- [x] Oni Memory (distributed KV + pub/sub)
- [x] Queue + scheduler
- [x] Mail + storage (local + S3)
- [x] CLI generator (`oni make:*`)
- [x] Admin panel
- [x] Vite + TypeScript + Tailwind v4
- [x] Stress test suite
- [ ] OAuth / social login (v1.3)
- [ ] Image processing (v1.4)

---

## Contributing

```bash
git clone https://github.com/onipixel/oniworks
cd oniworks
go mod download
go test ./...

# Run stress tests
go run ./testing/stress --workers 50 --duration 8s
```

---

## License

MIT — see [LICENSE](LICENSE).
