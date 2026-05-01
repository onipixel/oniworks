# OniWorks Roadmap

This document tracks planned improvements, features, and known gaps in the OniWorks framework.
Items are grouped by priority. Checked items are complete.

---

## v1.1 — Developer Experience

The highest-impact work that reduces friction for new developers the most.

### CLI Code Generation (`oni make:*`)

The `cmd/oni` CLI currently handles database operations only. The next milestone adds code generators so developers never write boilerplate by hand.

| Command | Output |
|---|---|
| `oni make:model Post` | `app/models/post.go` with struct, TableName(), JSON tags |
| `oni make:controller PostController` | `app/http/controllers/post_controller.go` with CRUD stubs |
| `oni make:migration create_posts_table` | `database/migrations/<timestamp>_create_posts_table.go` |
| `oni make:seeder PostSeeder` | `database/seeders/post_seeder.go` wired to seeder registry |
| `oni make:channel PostChannel` | `app/channels/post_channel.go` with Subscribe stub |
| `oni make:middleware RateLimitMiddleware` | `app/http/middleware/rate_limit.go` |

- [ ] Generator engine in `cmd/oni`
- [ ] Embedded stub templates per generator type
- [ ] Timestamp-prefixed migration filenames
- [ ] Model introspection (detect existing struct, skip if present)

### NULL-Safe Database Scanner

The scanner currently panics when a non-pointer struct field (e.g., `string`, `int64`) receives a SQL `NULL`. This is a footgun that breaks apps in production when optional columns are involved.

**Fix:** Before calling `rows.Scan`, inspect the destination type via reflection. For non-pointer scalars, substitute a zero value when the column is `NULL` rather than propagating the conversion error.

- [ ] Reflect-based null coercion in `framework/database/scanner.go`
- [ ] Coerce `NULL → ""` for `string`, `NULL → 0` for numeric types, `NULL → false` for `bool`
- [ ] Keep pointer fields (`*string`, `*int64`) as `nil` — existing behaviour preserved
- [ ] Add tests covering nullable columns against all scalar types

### Request Validation Binding

The `framework/validation` package exists but is disconnected from the HTTP layer. Developers manually bind structs and write their own validation logic per handler.

**Goal:** One-line validation in any controller:

```go
var req struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}
if err := c.Validate(&req); err != nil {
    return err // automatically returns 422 with field errors
}
```

- [ ] `c.Validate(dest any) error` on `framework/http/context.go`
- [ ] Automatic 422 response with structured field error map
- [ ] Built-in rules: `required`, `email`, `min`, `max`, `url`, `uuid`, `in`, `numeric`
- [ ] Custom rule registration via `validation.RegisterRule`

### Hot Reload Integration

Developers currently rebuild the binary manually on every backend change. The framework should make this effortless.

- [ ] Document `air` integration with a ready-made `.air.toml` in the project scaffold
- [ ] Add `oni serve --watch` flag that shells out to `air` if detected
- [ ] Include `air` in the recommended dev dependency list in the docs

---

## v1.2 — Data Layer

### Pagination Helper

Every controller currently reimplements offset/limit math manually. A first-class paginator removes this.

**API:**

```go
result, err := database.Table("posts").
    Where("user_id = ?", userID).
    OrderBy("created_at DESC").
    Paginate(page, 20)

// result.Data      []any (or use generic Paginate[T])
// result.Total     int64
// result.Page      int
// result.PerPage   int
// result.LastPage  int
// result.HasMore   bool
```

- [ ] `Paginate(page, perPage int)` method on `Builder`
- [ ] Generic `PaginateTyped[T](page, perPage int)` variant
- [ ] COUNT query runs in parallel with data query
- [ ] JSON serialisation matches common API conventions (`meta.total`, `meta.last_page`)

### Seeder Framework Wire-up

`framework/seeder` exists but has no CLI hook and no documentation. The OniGram seeder was written as a standalone binary as a workaround.

- [ ] `oni db:seed` — runs all registered seeders in order
- [ ] `oni db:seed --class=PostSeeder` — run a single named seeder
- [ ] Seeders respect `oni migrate:fresh && oni db:seed` for full reset workflow
- [ ] Document seeder registration pattern in the README

---

## v1.3 — Auth & Security

### OAuth / Social Login

Only JWT with email/password is supported. Most modern apps require at least one social login option.

- [ ] `framework/auth/oauth.go` — OAuth2 flow handler (state, callback, token exchange)
- [ ] Built-in providers: Google, GitHub, Discord
- [ ] Provider config via `.env` (`OAUTH_GOOGLE_CLIENT_ID`, etc.)
- [ ] Extensible `Provider` interface for custom OAuth2 sources
- [ ] Auto-creates user on first login; links provider to existing account by email

---

## v1.4 — Media & Storage

### Image Processing

Every app that handles user uploads needs resizing and format conversion. Without it, developers reach for external services immediately.

- [ ] `framework/storage/image.go` — image processing pipeline
- [ ] Resize to multiple named variants: `thumbnail` (150×150), `medium` (600px wide), `original`
- [ ] Convert to WebP automatically with JPEG/PNG fallback
- [ ] Strip EXIF metadata on ingest for privacy
- [ ] Lazy variant generation (generate on first request, cache thereafter)
- [ ] Works with both local disk and S3 storage drivers

---

## Backlog (Unscheduled)

These are valid improvements without a target version yet.

- **GraphQL layer** — optional `framework/graphql` package wrapping the router
- **OpenAPI generation** — auto-generate Swagger docs from route + struct definitions
- **Multi-tenancy helpers** — scoped query builder with tenant isolation
- **Soft deletes** — `deleted_at` support in the query builder (`WithTrashed`, `OnlyTrashed`)
- **Model events** — `BeforeCreate`, `AfterSave`, `BeforeDelete` hooks wired to the ORM
- **Two-factor auth** — TOTP support in `framework/auth`
- **WebSocket presence page** — who is online, typing indicators
- **Rate limiter per-user** — current rate limiter is IP-based only
- **Redis session driver** — `framework/session/drivers/redis.go`
- **Broadcast queue** — offload WebSocket broadcasts to the job queue for scale

---

## Discovered During OniGram (v1.0 Retrospective)

Issues found while building the OniGram Instagram clone test case.

| Issue | Severity | Status |
|---|---|---|
| `notifications` endpoint returned `null` instead of `[]` | Medium | Fixed in v1.0 |
| Feed `INNER JOIN` excluded own posts | High | Fixed in v1.0 |
| `Builder` lacked public `Exec()` for raw write queries | Medium | Fixed in v1.0 |
| Scanner crashed on `NULL post_id` (non-pointer `int64`) | High | Workaround: use `*int64`; permanent fix in v1.1 |
| Scanner crashed on `NULL avatar_path` in raw queries | High | Workaround: `COALESCE`; permanent fix in v1.1 |
| Dynamic Tailwind class names stripped by build scanner | Medium | Workaround: inline styles; document in frontend guide |
| `frontend.ViteTag` in dev mode requires Vite running separately | Low | By design; document workflow |
