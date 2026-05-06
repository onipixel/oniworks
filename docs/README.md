# OniWorks Documentation

**The Go framework that thinks in realtime.**

OniWorks is a batteries-included Go web framework inspired by Laravel, built for the era of live, connected applications.

```
event → socket → memory → broadcast → UI
                    ↓
             persist → PostgreSQL (when needed)
```

> 📖 Full documentation site — coming soon at **oniworks.dev**

---

## Guides

| Guide | Description |
|-------|-------------|
| [Getting Started](getting-started.md) | Install the CLI, scaffold a project, run the dev server |
| [Routing](routing.md) | Routes, URL parameters, groups, middleware, named routes |
| [Database & ORM](database.md) | Query builder, eager loading, migrations, lifecycle hooks |
| [Authentication](auth.md) | JWT, session auth, password hashing, CSRF, RBAC |
| [Realtime — Oni Socket](realtime.md) | WebSockets, channel routing, presence, reconnect/resume |
| [Oni Memory](memory.md) | Distributed KV store, pub/sub, TTL, gossip sync |
| [Frontend](frontend.md) | Vite + TypeScript + Tailwind v4 integration |
| [CLI Reference](cli.md) | All `oni` commands — generators, migrations, queue, deploy |
| [Testing](testing.md) | HTTP test helpers, fluent assertions, request builders |

---

## Quick Start

```bash
go install github.com/onipixel/oniworks/cmd/oni@latest

oni new my-app
cd my-app
oni migrate
oni serve
```

---

## Core Architecture

```
framework/
├── app/          IoC container + application bootstrap
├── http/         Router, context, middleware, validation binding
├── routing/      Trie-based route dispatch, groups, named routes
├── database/     Query builder, NULL-safe scanner, eager loading
├── migrations/   Schema builder, migration runner, rollback
├── auth/         JWT, session guard, bcrypt, CSRF
├── roles/        RBAC — roles & permissions with wildcard support
├── realtime/     Oni Socket — WebSocket hub, channels, presence
├── memory/       Oni Memory — distributed KV + pub/sub + TTL
├── queue/        Background jobs (memory + Redis drivers)
├── scheduler/    Cron-style task scheduler
├── mail/         SMTP mailer with HTML template rendering
├── storage/      File storage (local + S3-compatible)
├── validation/   Struct-tag validation, c.Validate()
├── config/       .env + YAML config loader
├── secrets/      AES-256-GCM encrypted secrets
├── logging/      Structured logging (slog)
├── errors/       Rich debug pages (dev) / clean JSON (prod)
├── health/       Health check endpoints
├── metrics/      Prometheus metrics
├── admin/        Auto-generated CRUD admin panel
├── deploy/       Caddy integration + auto TLS
└── frontend/     Vite manifest loader + TypeScript type generator
```
