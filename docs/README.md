# OniWorks

**The Go framework that thinks in realtime.**

OniWorks is a batteries-included Go web framework inspired by Laravel, built for the era of live, connected applications. It combines a familiar request/response layer with a first-class realtime platform backed by a distributed in-memory database.

```
event → socket → memory → broadcast → UI
                    ↓
             persist → Postgres (when needed)
```

## Quick Start

```bash
# Install the CLI
go install github.com/onipixel/oniworks/cmd/oni@latest

# Create a new project
oni new my-app
cd my-app
go mod init example.com/my-app
go get github.com/onipixel/oniworks

# Start the dev server
oni serve
```

## Core Architecture

```
Oni Core
├── HTTP layer      (framework/http, framework/routing)
├── Socket layer    (framework/realtime)     ← realtime nervous system
├── Memory layer    (framework/memory)       ← distributed KV + pub/sub
├── DB layer        (framework/database)     ← PostgreSQL / MySQL ORM
└── Worker layer    (framework/queue)        ← async jobs
```

## Documentation

| Section | File |
|---------|------|
| Getting Started | [getting-started.md](getting-started.md) |
| Routing | [routing.md](routing.md) |
| Database & ORM | [database.md](database.md) |
| Authentication | [auth.md](auth.md) |
| Realtime (Oni Socket) | [realtime.md](realtime.md) |
| Oni Memory | [memory.md](memory.md) |
| Queue & Jobs | [queue.md](queue.md) |
| Mail | [mail.md](mail.md) |
| Storage | [storage.md](storage.md) |
| Scheduler | [scheduler.md](scheduler.md) |
| Validation | [validation.md](validation.md) |
| Secrets | [secrets.md](secrets.md) |
| Deploy (Caddy + TLS) | [deploy.md](deploy.md) |
| CLI Reference | [cli.md](cli.md) |
| Testing | [testing.md](testing.md) |
| Admin Panel | [admin.md](admin.md) |
| Health & Metrics | [observability.md](observability.md) |
