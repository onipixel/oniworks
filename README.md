<div align="center">

<img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go 1.22+">
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

| Feature | OniWorks |
|---|---|
| HTTP routing + middleware | ✅ |
| ORM (PostgreSQL + MySQL) | ✅ |
| Migrations & seeders | ✅ |
| Auth (JWT + sessions + roles) | ✅ |
| WebSocket realtime (Oni Socket) | ✅ |
| Distributed in-memory store (Oni Memory) | ✅ |
| Queue / background jobs | ✅ |
| Scheduler / cron | ✅ |
| Mail / SMTP | ✅ |
| File storage (local + S3) | ✅ |
| Auto TLS via Caddy (Oni Deploy) | ✅ |
| CLI code generator | ✅ |
| Admin panel | ✅ |
| Metrics + health checks | ✅ |

---

## Quick Start

```bash
# Install the CLI
go install github.com/onipixel/oniworks/cmd/oni@latest

# Create a new project
oni new my-app
cd my-app

# Start the dev server
oni serve
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
oni new <name>              Create a new project
oni serve                   Start dev server with hot reload
oni make:controller <Name>  Generate a controller
oni make:model <Name>       Generate a model + migration
oni make:migration <Name>   Generate a migration file
oni make:job <Name>         Generate a background job
oni make:middleware <Name>  Generate middleware
oni migrate                 Run pending migrations
oni migrate:rollback        Rollback last migration batch
oni seed                    Run database seeders
oni key:generate            Generate application key
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

## Examples

- [`examples/basic-api`](examples/basic-api/) — REST API with auth
- [`examples/realtime-chat`](examples/realtime-chat/) — WebSocket chat app
- [`examples/fullstack-app`](examples/fullstack-app/) — Full-stack with Vite + Tailwind

---

## Status

OniWorks is currently in **alpha**. The core API is stable but may have breaking changes before v1.0.

- [x] Core HTTP layer
- [x] ORM + migrations
- [x] Auth (JWT + sessions + roles)
- [x] Oni Socket (realtime)
- [x] Oni Memory (distributed cache)
- [x] Queue + scheduler
- [x] Mail + storage
- [x] CLI generator
- [x] Admin panel
- [x] Oni Deploy (Caddy + TLS)
- [ ] Full test suite
- [ ] Oni Memory TCP gossip (multi-node)
- [ ] Official documentation site

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
