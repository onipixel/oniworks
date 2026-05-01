<div align="center">

<img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go" alt="Go 1.25+">
<img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="MIT License">
<img src="https://img.shields.io/badge/status-alpha-orange?style=flat" alt="Alpha">
<img src="https://img.shields.io/badge/PostgreSQL-primary-336791?style=flat&logo=postgresql" alt="PostgreSQL">

# OniWorks

**The Go framework that thinks in realtime.**

OniWorks is a batteries-included Go web framework inspired by Laravel, built for the era of live, connected applications. It gives you a familiar MVC-style structure, a full ORM, queues, auth, and more — with a first-class realtime layer baked in from day one.

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

| Feature | Status |
|---|---|
| HTTP routing + middleware | ✅ Stable |
| ORM — query builder + struct scanner | ✅ Stable |
| Migrations (schema builder, rollback) | ✅ Stable |
| Auth (JWT + bcrypt + RBAC) | ✅ Stable |
| WebSocket realtime (Oni Socket) | ✅ Stable |
| Distributed in-memory store (Oni Memory) | ✅ Stable |
| Queue / background jobs | ✅ Stable |
| Scheduler / cron | ✅ Stable |
| Mail / SMTP | ✅ Stable |
| File storage (local + S3) | ✅ Stable |
| Vite + Tailwind frontend integration | ✅ Stable |
| Metrics + health checks | ✅ Stable |
| Auto TLS via Caddy (Oni Deploy) | ✅ Stable |
| CLI code generation (`oni make:*`) | 🗓 Roadmap v1.1 |
| NULL-safe database scanner | 🗓 Roadmap v1.1 |
| Request validation binding (`c.Validate`) | 🗓 Roadmap v1.1 |
| Pagination helper | 🗓 Roadmap v1.2 |
| OAuth / social login | 🗓 Roadmap v1.3 |
| Image processing (resize, WebP) | 🗓 Roadmap v1.4 |

---

## Quick Start

```bash
# Install the CLI
go install github.com/onipixel/oniworks/cmd/oni@latest

# Create a new project
oni new my-app
cd my-app
```

## Hello World

```go
package main

import (
    "github.com/onipixel/oniworks/framework/app"
    "github.com/onipixel/oniworks/framework/http"
)

func main() {
    oni := app.New()

    oni.Router.Get("/", func(c *http.Context) error {
        return c.JSON(200, map[string]string{"message": "Hello from OniWorks!"})
    })

    oni.Start()
}
```

## Realtime in 10 lines

```go
// Server — broadcast to a channel
socket.On("chat:message", func(e *realtime.Event) {
    socket.Broadcast("chat:"+e.Channel, e.Data)
})

// Client (TypeScript)
const oni = new OniSocket("/ws")
oni.join("chat:general")
oni.on("chat:general", (msg) => appendMessage(msg))
oni.emit("chat:message", { channel: "general", text: "hey!" })
```

---

## Architecture

```
oniworks/
├── cmd/oni/          CLI (oni new, oni serve, oni make:*)
├── framework/
│   ├── app/          Application bootstrap & IoC container
│   ├── http/         Router, context, middleware, response
│   ├── routing/      Route groups, resource routes, named routes
│   ├── database/     ORM, query builder, connection pool
│   ├── migrations/   Schema migration engine
│   ├── seeder/       Database seeder
│   ├── auth/         JWT, sessions, CSRF, password hashing
│   ├── roles/        RBAC — roles & permissions
│   ├── realtime/     Oni Socket — WebSocket hub & channels
│   ├── memory/       Oni Memory — distributed KV + pub/sub
│   ├── queue/        Job queue with PostgreSQL backend
│   ├── scheduler/    Cron-style task scheduler
│   ├── mail/         SMTP mailer with template support
│   ├── storage/      File storage (local + S3-compatible)
│   ├── config/       .env + YAML config with validation
│   ├── secrets/      Encrypted secrets management
│   ├── logging/      Structured logger (slog-based)
│   ├── errors/       HTTP error pages + stack traces
│   ├── health/       Health check endpoints
│   ├── metrics/      Prometheus metrics
│   ├── admin/        Auto-generated admin panel
│   ├── deploy/       Caddy integration + auto TLS
│   ├── backup/       Database backup & restore
│   └── frontend/     Vite + Tailwind integration
├── stubs/            Code generation templates
├── examples/         Example applications
└── docs/             Full documentation
```

---

## Oni Socket — Realtime Platform

Oni Socket is the nervous system of OniWorks. It is not just a WebSocket wrapper — it is a full realtime platform with:

- **Channel/room system** with access control
- **Presence** — know who is online in each channel
- **Event routing** with typed handlers
- **Backpressure & message batching** for high-throughput workloads
- **Per-connection authentication** (JWT or session)
- **Reconnect/resume** with message replay
- **Cross-node sync** for multi-server deployments

## Oni Memory — Distributed In-Memory Store

Oni Memory is a distributed in-memory database built into the framework. It acts as:

- **Cache** — TTL-based key/value store
- **Sessions** — fast session storage
- **Realtime state** — shared state across socket connections
- **Presence** — track who is in each room
- **Event bus** — cross-node pub/sub

By default it runs in-process. Enable TCP gossip mesh for multi-node sync with a single config option.

---

## CLI

```bash
# Available today
oni new <name>              Create a new project
oni db:create               Create the configured database
oni db:drop                 Drop the configured database
oni migrate                 Run pending migrations
oni migrate:rollback        Rollback last migration batch
oni migrate:fresh           Drop all tables and re-run all migrations
oni migrate:status          Show migration status

# Coming in v1.1
oni make:controller <Name>  Generate a controller
oni make:model <Name>       Generate a model + migration
oni make:migration <Name>   Generate a migration file
oni make:middleware <Name>  Generate middleware
oni make:channel <Name>     Generate a WebSocket channel
oni db:seed                 Run database seeders
```

---

## Documentation

Full documentation lives in [`docs/`](docs/):

| Guide | |
|---|---|
| [Getting Started](docs/getting-started.md) | Install, create project, run |
| [Routing](docs/routing.md) | Routes, middleware, groups |
| [Database & ORM](docs/database.md) | Models, queries, relationships |
| [Authentication](docs/auth/) | JWT, sessions, CSRF |
| [Realtime — Oni Socket](docs/realtime.md) | WebSockets, channels, presence |
| [Oni Memory](docs/memory.md) | Distributed cache & event bus |
| [CLI Reference](docs/cli.md) | All `oni` commands |
| [Deploy — Caddy + TLS](docs/deploy.md) | Production deployment |
| [Testing](docs/testing.md) | Unit & integration testing |

---

## Real-world Example — OniGram

OniGram is a full Instagram clone built entirely on OniWorks. It was developed as a stress test to validate the framework against a production-complexity app and surface any gaps.

**What it ships with:**
- JWT authentication, avatar uploads, user search
- Posts, likes, comments, bookmarks, hashtags, @mentions
- Stories with 24-hour expiry and view tracking
- Real-time direct messages over Oni Socket
- Explore page with a 3-column grid and lightbox viewer
- Notifications (like, comment, follow, DM)
- Responsive SPA with mobile swipe gestures and a desktop sidebar
- Seeded with 14 users, 70 posts, real images from free CDNs

Source: [`testing/onigram/`](testing/onigram/)

---

## Examples

- [`testing/onigram`](testing/onigram/) — Full Instagram clone (OniGram)

---

## Status

OniWorks is currently in **alpha**. The core API is stable but may have breaking changes before v1.0.

- [x] Core HTTP layer
- [x] ORM + query builder + migrations
- [x] Auth (JWT + bcrypt + RBAC)
- [x] Oni Socket (realtime WebSockets)
- [x] Oni Memory (distributed cache + pub/sub)
- [x] Queue + scheduler
- [x] Mail + file storage
- [x] Vite + Tailwind frontend integration
- [x] Metrics + health checks
- [x] Oni Deploy (Caddy + auto TLS)
- [x] OniGram — full-stack stress test app
- [ ] CLI code generation (`oni make:*`)
- [ ] NULL-safe database scanner
- [ ] Request validation binding
- [ ] Full test suite
- [ ] Oni Memory TCP gossip (multi-node)
- [ ] Official documentation site

See [ROADMAP.md](ROADMAP.md) for the full prioritised backlog.

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

```bash
git clone https://github.com/onipixel/oniworks
cd oniworks
go mod download
go test ./...
```

---

## License

MIT — see [LICENSE](LICENSE).

---

## License

MIT — see [LICENSE](LICENSE).
