# OniWorks Roadmap

This document tracks planned improvements, features, and known gaps in the OniWorks framework.
Items are grouped by priority. Checked items are complete.

---

## v1.1 — Developer Experience ✅

### CLI Code Generation (`oni make:*`) ✅

All generators are implemented in `cmd/oni`:

| Command | Output |
|---|---|
| `oni make:model Post` | `app/models/post.go` with struct, db tags |
| `oni make:controller PostController` | CRUD handler stubs |
| `oni make:migration create_posts_table` | timestamped migration file |
| `oni make:middleware RequireAdmin` | middleware template |
| `oni make:job SendEmail` | queue job template |
| `oni make:mail WelcomeMail` | mailer template |
| `oni make:channel ChatChannel` | realtime channel handler |
| `oni make:seeder PostSeeder` | database seeder |
| `oni make:policy PostPolicy` | authorization policy |
| `oni make:test PostTest` | test helper scaffold |
| `oni make:resource PostResource` | JSON resource transformer |

### NULL-Safe Database Scanner ✅

Fixed in `framework/database/scanner.go`. The `nullableScanner` type wraps non-pointer fields and leaves them at zero value when the column is `NULL`. Pointer fields (`*string`, `*int64`) remain `nil` for `NULL` — existing behaviour preserved.

### Request Validation Binding ✅

`c.Validate(&req)` binds the request body and runs `validate` struct tag rules in one call. On failure, returns `422 Unprocessable Entity` with a structured error map:

```go
var req struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}
if err := c.Validate(&req); err != nil {
    return err // 422 {"message":"validation failed","errors":{"email":["..."]}}
}
```

### Hot Reload Integration ✅

`oni serve` detects and uses [Air](https://github.com/air-verse/air) for live reload. A `.air.toml` is scaffolded into new projects.

---

## v1.2 — Data Layer ✅

### Pagination Helper ✅

`Builder.Paginate(page, perPage int, dest any)` runs a COUNT and a SELECT in sequence and returns a `*Page[any]`:

```go
var posts []Post
page, err := db.Table("posts").
    Where("user_id = ?", userID).
    OrderBy("created_at DESC").
    Paginate(1, 20, &posts)

// page.Total, page.LastPage, page.From, page.To, page.CurrentPage, page.PerPage
```

### Seeder Framework ✅

`oni db:seed` and `oni db:seed --class=PostSeeder` are implemented. See [CLI docs](/docs/cli).

### Soft Deletes ✅

`Builder.SoftDelete()` adds `deleted_at IS NULL` automatically. Use `.WithTrashed()` to include deleted rows. Use `.SoftDelete().Where("id = ?", id).Update(Map{"deleted_at": time.Now()})` to soft-delete.

---

## v1.3 — Auth & Security

### OAuth / Social Login

- [ ] `framework/auth/oauth.go` — OAuth2 flow (state, callback, token exchange)
- [ ] Built-in providers: Google, GitHub, Discord
- [ ] Provider config via `.env`
- [ ] Extensible `Provider` interface
- [ ] Auto-creates user on first login; links by email

---

## v1.4 — Media & Storage

### Image Processing

- [ ] `framework/storage/image.go` — resize pipeline
- [ ] Named variants: `thumbnail` (150×150), `medium` (600px wide), `original`
- [ ] Convert to WebP with JPEG/PNG fallback
- [ ] Strip EXIF on ingest
- [ ] Lazy variant generation
- [ ] Works with local and S3 drivers

---

## Backlog (Unscheduled)

- **GraphQL layer** — optional `framework/graphql` package
- **OpenAPI generation** — Swagger docs from route + struct definitions
- **Multi-tenancy helpers** — scoped query builder
- **Two-factor auth** — TOTP support in `framework/auth`
- **Redis session driver** — `framework/session/drivers/redis.go`
- **Rate limiter per-user** — current limiter is IP-based only
- **Broadcast queue** — offload WebSocket broadcasts to the job queue

---

## Fixed Issues

| Issue | Status |
|---|---|
| `notifications` endpoint returned `null` instead of `[]` | Fixed v1.0 |
| Feed `INNER JOIN` excluded own posts | Fixed v1.0 |
| `Builder` lacked public `Exec()` for raw write queries | Fixed v1.0 |
| Scanner crashed on `NULL` for non-pointer fields | Fixed v1.1 |
| `Guard.cachedUser` shared across requests (auth bypass) | Fixed v1.1 |
| Container singleton race (double construction) | Fixed v1.1 |
| `Hub.ServeHTTP` not mountable on OniWorks router | Fixed v1.1 — use `hub.Handler()` |
| `Delete()`/`Update()` without WHERE silently mutated all rows | Fixed v1.1 — returns error |
| `globalDB` data race on concurrent access | Fixed v1.1 — atomic.Pointer |
| Transaction did not rollback on panic | Fixed v1.1 |
| `CompareAndSwap` used string comparison for equality | Fixed v1.1 |
| Default query log level was INFO (flooded logs) | Fixed v1.1 — defaults to DEBUG |
| `callSliceHook` AfterFind not called per element | Fixed v1.1 |
