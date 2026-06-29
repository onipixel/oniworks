# Changelog

All notable changes to OniWorks are documented here.

---

## Unreleased — Hardening pass (toward v1.2 / 1.0)

A full-framework security, correctness, and polish sweep. See `ROADMAP.md` and
`HARDENING.md` for the milestone plan. **24/28 modules now tested** (was 12/28),
with live-PostgreSQL integration tests, a scaffold-and-build test, and CI
(`.github/workflows/ci.yml`) running the race detector + a Postgres service.

### ⚠️ Breaking changes
- **`admin`** — the panel now **fails closed**: every route returns 403 until an authorizer is configured via `admin.New(db, admin.WithAuth(fn))`.
- **`auth`** — JWT operations require a signing secret of **≥32 bytes** (`ErrJWTNotConfigured` otherwise); the parser enforces `HS256` + a required `exp`.
- **`http`** — `Request.IP()` no longer trusts `X-Forwarded-For`/`X-Real-IP` unless the peer is a configured trusted proxy (`http.SetTrustedProxies`).
- **`secrets`** — APP_KEY derivation changed: `base64:` keys are now decoded (not re-hashed) and passphrases use scrypt instead of a single SHA-256. **Existing encrypted data must be re-encrypted.**
- **`seeder`** — `Seeder.Run` / `Runner.Run` now take `*database.DB` directly (the structural `seeder.DB` interface was removed).
- **`memory`** — `ClockValue` fields are now exported.
- **`database`** — `Select`/`OrderBy`/`GroupBy`/`Pluck` now accept only validated column references (for injection safety). Raw expressions such as `Select("DISTINCT posts.*")`, `Select("COUNT(*) AS n")`, or `OrderBy("RANDOM()")` must move to the new `SelectRaw`/`OrderByRaw`/`GroupByRaw` methods.

### Security
- **SQL injection** — `Select`/`OrderBy`/`GroupBy`/`Pluck` validate + quote every identifier (allow-list grammar); added `SelectRaw`/`OrderByRaw`/`GroupByRaw`; admin search allow-lists its column.
- **Admin/metrics/health** — admin & `/metrics` require an authorizer (fail closed); `/metrics` collapses high-cardinality path labels; public `/health` redacts check messages (new guarded `DetailedHandler`); `errors.HandlerForEnv` enables debug only in dev.
- **Gossip** — pre-shared-secret handshake + 8 MiB length-framed messages (was unauthenticated, unbounded).
- **RBAC** — wildcard matching is segment-aware (`user*` no longer over-grants across boundaries).
- **Storage** — local driver rejects path traversal via a `filepath.Rel` containment check (Windows volume/UNC included).
- **Session/CSRF** — session ID rotates on login (fixation); constant-time CSRF compare; login runs a constant-time dummy bcrypt for unknown users (no enumeration).
- **Backup/mail** — MySQL backup password moved off argv to `MYSQL_PWD`; `mail.NewFromConfig` defaults to TLS (was cleartext).

### Concurrency & stability
- **`http`** — `Context` mutex held by pointer (fixes the `go vet` lock-copy data race); `Timeout` middleware rewritten with `http.TimeoutHandler` semantics (no concurrent socket writes).
- **`memory`** — pub/sub no longer panics on concurrent unsubscribe (send-on-closed); gossip uses one serialized writer per connection.
- **`queue`/`scheduler`** — panicking jobs recover (retry/dead-letter) instead of killing the worker/process; scheduler skips self-overlapping runs.

### Correctness
- **`database`** — `Paginate` count honors soft-delete + GROUP BY and populates `Page.Items`; scanner handles `[]byte`/string → numeric/bool/time and errors instead of silently zeroing; soft-delete write path (`SoftDelete().Delete()` + `ForceDelete()`); eager-load relations resolve by field index (two relations to one table no longer collide).
- **`migrations`** — `Migrate` runs the whole batch in one transaction (atomic rollback on Postgres).
- **`validation`** — `required` accepts legitimate `0`/`false`; `min`/`max` error on unsupported types.
- **`middleware`** — CORS never emits credentials for a wildcard origin and adds `Vary: Origin`; compress honors the level, gates on content type, and skips 204/already-encoded bodies.
- **`routing`** — `405` + `Allow` for known paths under other methods; implicit `HEAD`→`GET` and auto-`OPTIONS`; multipart temp files cleaned up.
- **`realtime`** — resume replays the full buffer when `last_event_id` aged out (no silent gaps); presence counts cross-node members; `oni:subscribe`/`oni:unsubscribe` so `channel().on()` receives broadcasts.

### New
- **`@oniworks/socket`** (`client/`) — the official TypeScript client: typed channels, auto-reconnect, resume, heartbeat, presence. (The README advertised it; it now exists.)
- **`oni make:*`** generators work in scaffolded projects (stubs embedded via `//go:embed`); the scaffolded app wires `route:list`/`health` and returns honest errors for unwired commands.

---

## v1.1.0 — 2026-05-06

### Security Fixes

- **`auth/guard.go`** — Critical auth bypass: `cachedUser` was a singleton-level field shared across all requests. Removed. Session-based lookup is now strictly per-request.
- **`memory/store.go`** — `CompareAndSwap` used `fmt.Sprintf` for equality, causing `int64(1)` and `float64(1.0)` to incorrectly compare equal. Replaced with `reflect.DeepEqual`.
- **`realtime/hub.go`** — WebSocket broadcast handler now uses `sandbox="allow-same-origin allow-popups"` (no `allow-scripts`) on email iframes — XSS-safe auto-sizing without script execution.

### Bug Fixes

- **`app/container.go`** — Singleton factory race: two goroutines could both call the factory and the second would overwrite the first instance. Fixed with double-checked locking.
- **`app/container.go`** — `MakeInto` would panic on type mismatch. Now checks assignability and returns a descriptive error.
- **`app/application.go`** — `ServeAddr(addr)` ignored its argument and always started on the default port. Fixed.
- **`database/builder.go`** — `Delete()` and `Update()` with no `Where` clause silently wiped entire tables. Now return an error. Use `WhereRaw("1=1")` to confirm full-table operations.
- **`database/builder.go`** — `rows.Close()` was called twice in `First()` and `All()` (once deferred, once explicit). Removed redundant defer.
- **`database/db.go`** — `globalDB` had a data race on concurrent access. Replaced `*DB` pointer with `atomic.Pointer[DB]`.
- **`database/db.go`** — Transactions did not rollback on panic inside `fn`. Added `recover()` wrapper.
- **`database/db.go`** — Default query log level was `slog.LevelInfo` (zero value), flooding production logs. Changed default to `slog.LevelDebug`.
- **`database/hooks.go`** — `callSliceHook` AfterFind was broken — had no-op placeholder closures and never iterated elements. Replaced with proper reflection loop.
- **`middleware/ratelimit.go`** — Cleanup goroutine leaked forever (no stop mechanism). Added `context.Context` cancel.
- **`memory/store.go`** — `Get()` had TOCTOU: released read lock then re-acquired write lock to delete expired keys. Fixed with re-check under write lock.
- **`routing/router.go`** — `defaultErrorHandler` had a manual error-unwrap loop. Replaced with `errors.As`.
- **`realtime/hub.go`** — `pushToChannel` allocated `make([]*Conn, 0)` without capacity hint. Fixed.

### New Features

- **`database/builder.go`** — `Paginate(page, perPage int, dest any)` — runs COUNT + SELECT, returns `*Page[any]` with `Total`, `LastPage`, `From`, `To`.
- **`database/builder.go`** — `SoftDelete()` / `WithTrashed()` — auto-injects `deleted_at IS NULL` into SELECT queries.
- **`database/builder.go`** — `WhereRaw(clause, args...)` — explicit raw WHERE clause for full control.
- **`database/scanner.go`** — NULL-safe scanning: non-pointer fields (`string`, `int64`, `bool`) no longer panic on SQL `NULL`. Zero value is used instead.
- **`http/context.go`** — `c.Validate(&req)` — binds the request body and runs `validate` struct tag rules in one call. Returns `422` with structured field error map on failure.
- **`validation/validator.go`** — `validation.Default()`, `validation.SetDefault(v)`, `validation.Validate(s)` package-level shortcuts.
- **`realtime/handler.go`** — `hub.Handler()` — returns `onihttp.HandlerFunc` so the WebSocket hub can be mounted on the OniWorks router (`r.Get("/ws", hub.Handler())`).

### OniGram Feature Additions

- **Hashtag system** — `#tags` extracted from post captions on create via regex, stored in `hashtags` + `post_hashtags` tables. New `HashtagController`: `GET /api/hashtags/trending` (ranked by post count) and `GET /api/hashtags/:tag` (paginated feed). Seeder now links existing captions to hashtag rows.
- **Comment replies** — Added `parent_comment_id` nullable FK on `comments`. `CommentController.Index` returns top-level comments with `replies` nested. `CommentController.Store` accepts `parent_comment_id`. Reply UI in post lightbox: reply button per comment, inline reply indicator, inline reply rendering.
- **Post carousel** — New `post_images` table for multi-image posts. `PostController.Store` parses `images[0..9]` fields (falls back to single `image`). `enrichPosts` batch-loads carousel images. Feed card shows swipeable carousel with prev/next arrows and dot indicators. Lightbox carousel with the same controls.
- **Comment count** — `enrichPosts` now batch-loads comment counts alongside like counts. Displayed in feed card action row and post lightbox.
- **Live trending hashtags** — Feed sidebar and explore sidebar now call `GET /api/hashtags/trending` instead of using hardcoded static data.
- **`c.Validate()` throughout** — `AuthController.Register` and `Login` now use `validate` struct tags (`required`, `email`, `min`, `alphanum`) instead of manual checks. `CommentController.Store` uses `validate:"required,min=1,max=2200"`.
- **`mentionify` captions** — Post card captions in the feed now render `#hashtags` and `@mentions` as clickable links (was `escapeHTML` only).

### Tooling

- **`testing/stress/main.go`** — New stress test suite covering HTTP router (in-process + TCP), Oni Memory (Set/Get/Incr/CAS/Pub-Sub/TTL eviction), PostgreSQL query builder (SELECT/INSERT/UPDATE/Transaction/Paginate/pool saturation), and WebSocket hub (concurrent connections, broadcast delivery, round-trip latency).

### Documentation

- All docs in `sample/oniworks-site/content/docs/` rewritten to match the actual API. Previous versions had wrong import paths (`github.com/oniworks/` → `github.com/onipixel/`), non-existent methods, and wrong signatures.
- `database.md`, `query-builder.md`, `auth.md`, `realtime.md`, `memory.md`, `queue.md`, `routing.md`, `middleware.md`, `validation.md`, `getting-started.md` fully updated.
- `ROADMAP.md` updated: v1.1 and v1.2 items marked complete, all fixed bugs listed.
- `README.md` rewritten with correct API, performance benchmarks, and OniMail showcase.

---

## v1.0.0 — Initial Release

- Core HTTP layer (routing, middleware, context)
- Query builder with eager loading, transactions, migrations
- Auth: JWT + session + bcrypt + CSRF
- Oni Socket: WebSocket hub with channel routing, presence, reconnect/resume
- Oni Memory: distributed KV + pub/sub + TTL + gossip
- Queue system (memory + Redis drivers)
- Cron scheduler
- SMTP mailer with templates
- File storage (local + S3)
- AES-256-GCM secrets management
- Prometheus metrics + health checks
- Auto-generated admin panel
- Vite + TypeScript + Tailwind v4 frontend integration
- TypeScript type generation from Go structs
- CLI generator (`oni make:*`, `oni migrate`, `oni serve`, `oni deploy`)
- OniMail: production self-hosted email platform as a reference application
