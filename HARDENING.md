# OniWorks — Hardening & Completion Roadmap

> Full-framework review, 2026-06-29. Baseline: `go build ./...` clean, `go test ./...` green
> (12/28 modules), one `go vet` warning. The framework is genuinely large and well-structured
> (~15K LOC, 28 modules) and its docs/stubs match the real API unusually well. The flaws below
> are concentrated in **security defaults**, **concurrency under load**, **data-layer correctness**,
> and the **CLI's project-integration layer** — not in the core design.
>
> Severity reflects exploitability/impact in a real deployment. Each item lists `file:line` and
> the concrete fix. This document is the work plan; the existing `ROADMAP.md` remains the
> *feature* roadmap (OAuth, image processing, etc.).

---

## Phase 0 — Security defaults (do first; blocks any public/production use)

| # | Sev | Location | Flaw | Fix |
|---|-----|----------|------|-----|
| 0.1 | 🔴 Critical | `framework/admin/admin.go:86` | Admin panel mounts dashboard/list/create/**delete** with no auth — anyone reaching `/admin/...` has full CRUD on every registered table. | Make an auth middleware a **mandatory** constructor arg (`New(db, WithAuth(mw))`) and wrap every route; refuse to start without it. |
| 0.2 | 🔴 Critical | `framework/database/builder.go` (`Select`/`OrderBy`/`GroupBy`/`Having`/`Join`/`Table`), `framework/admin/admin.go:135` | Identifiers and raw fragments are string-interpolated unescaped → SQL injection the moment any reaches user input (e.g. `OrderBy(sortParam)`, admin search column). The "injection-safe" claim is false for identifiers. | Route all identifiers through the existing escaping `QuoteIdent`; whitelist `OrderBy`/`Select`/`GroupBy` against known columns; document `WhereRaw`/`Join`/`Having` as raw sinks. |
| 0.3 | 🔴 Critical | `framework/migrations/{schema,table,grammar}.go`, `migrator.go:331` | DDL table/column names use bare `fmt.Sprintf("\"%s\"", name)` (no quote-doubling); `Default(string)` is unescaped. | Escape all DDL identifiers via grammar `QuoteIdent`; escape `Default` string values. |
| 0.4 | 🔴 Critical | `framework/auth/guard.go:119-124` | JWT verify returns `g.jwtSecret` with no empty-check (empty secret ⇒ forgeable tokens) and no required-exp / valid-methods enforcement (no-`exp` tokens valid forever). | Reject empty/short `jwtSecret` in `NewGuard`; add `jwt.WithValidMethods([]string{"HS256"})` + `jwt.WithExpirationRequired()`. |
| 0.5 | 🔴 Critical | `framework/memory/gossip.go:113-123` | Cluster gossip port accepts **unauthenticated**, unbounded `gob` from any network peer → DoS / arbitrary type instantiation into the KV map. | Shared-secret/HMAC handshake per peer; wrap reader in `io.LimitReader`; reject oversized/unknown messages before apply. (Until then, default gossip **off** and document the trust boundary.) |
| 0.6 | 🔴 Critical | `framework/backup/backup.go:213,224` | MySQL backup/restore passes the DB password as `-p<pw>` on argv → visible via `ps`/`/proc`. (Postgres path is correct.) | Use `MYSQL_PWD` env or a `0600` `--defaults-extra-file`. |
| 0.7 | 🟠 High | `framework/metrics/metrics.go:100,120` | `/metrics` is unauthenticated and labels series by **raw request path** → info disclosure + unbounded-cardinality DoS. | Require auth; use the matched route *pattern* as the label, not `req.URL.Path`. |
| 0.8 | 🟠 High | `framework/errors/handler.go:67-88` | Debug error pages reflect `err.Error()` + stack + file paths; `debugMode` is a plain bool with no prod guard. | Gate debug rendering behind an explicit non-prod assertion; never reflect `err.Error()` to untrusted clients. |
| 0.9 | 🟠 High | `framework/http/request.go:42-48` → `middleware/ratelimit.go:33` | `IP()` trusts client-supplied `X-Forwarded-For`/`X-Real-IP`; the rate limiter keys on it → trivial bypass by header rotation (and forged audit logs). | Honor forwarding headers only from configured trusted proxies; else use `RemoteAddr`. |

---

## Phase 1 — Crashes & races (process stability under load)

| # | Sev | Location | Flaw | Fix |
|---|-----|----------|------|-----|
| 1.1 | 🔴 Critical | `framework/http/context.go:62` | `WithContext` does `clone := *c`, copying the `sync.RWMutex` by value while sharing the `store` map → two Contexts guard one map with different locks (data race; confirmed by `go vet`). | Move `mu`+`store` into a heap `*ctxState` shared by pointer; cloning copies the pointer, not the lock. |
| 1.2 | 🔴 Critical | `framework/middleware/timeout.go:20-35` | On timeout the parent writes a response while the detached handler goroutine (still holding the writer) keeps running → concurrent `ResponseWriter` write race + goroutine leak. | Adopt `http.TimeoutHandler` semantics: buffered writer + `committed` flag; never write from the parent once the child owns the writer. |
| 1.3 | 🔴 Critical | `framework/memory/pubsub.go:46` vs `:82` | `publish` sends `s.buf <- msg` after releasing the lock; a concurrent `cancel` `close(sub.buf)` → **send-on-closed panic**; double-cancel → double-close panic. Reachable from normal hub/presence/broadcast traffic. | Guard delivery + close with a per-subscriber `sync.Once`+`closed` flag (idempotent cancel; publish never sends to a closed buffer). |
| 1.4 | 🔴 Critical | `framework/memory/gossip.go:145-152` | Every broadcast creates a fresh `gob.Encoder` on the **shared** conn in its own goroutine → interleaved bytes corrupt the wire stream. | One persistent `*gob.Encoder` per connection behind a per-conn mutex (or a single writer goroutine per peer). |
| 1.5 | 🟠 High | `framework/queue/queue.go:164` | A panicking job unwinds the worker goroutine → worker permanently lost and the job is neither retried nor dead-lettered. | Wrap `job.Handle` in `defer recover()` → retry/dead-letter. |
| 1.6 | 🟠 High | `framework/scheduler/scheduler.go:106-118,88` | A panicking scheduled job crashes the **whole process**; bare `AddFunc` allows a slow job to overlap itself. | `defer recover()` in `run`; register with `cron.SkipIfStillRunning`. |
| 1.7 | 🟠 High | `framework/memory/clock.go:66-82` | `ClockValue` fields are unexported → gob drops the vector clock on the wire, and `After` indexes both clocks by `cv.nodeID` → cross-node last-write-wins is effectively "always overwrite." | Export the fields (or add a marshalable DTO); do real vector-dominance comparison across the node-ID union. |
| 1.8 | 🟠 High | `framework/queue/drivers/redis.go:76-91` | `ZRANGEBYSCORE` + `ZREM` + `LPUSH` is non-atomic → two workers can promote the same delayed job → double processing. | Promote atomically with one `EVAL` Lua script. |
| 1.9 | 🟡 Medium | `framework/config/config.go:151` | `All()` returns a shallow copy; nested maps are shared, so `Merge`/reload races with `getNested` readers outside the lock. | Deep-copy on read, or swap a whole new map under the lock (immutable-on-reload). |

---

## Phase 2 — Data-layer correctness

| # | Sev | Location | Flaw | Fix |
|---|-----|----------|------|-----|
| 2.1 | 🟠 High | `framework/database/builder.go:297-306` | `Paginate`'s COUNT clone copies `withTrashed` but **not** `softDelete` (and drops `GROUP BY`/`HAVING`) → wrong `Total`/`LastPage`, leaks trashed-row counts. | Copy `softDelete` (and group/having) into the count builder, or COUNT via subquery wrapper. |
| 2.2 | 🟠 High | `framework/database/builder.go:277` | `Page[any].Items` is declared but never populated → callers reading `page.Items` get an empty slice. | Populate `Items` from `dest`, or remove the field. |
| 2.3 | 🟠 High | `framework/database/builder.go:195-197,483-492` | Soft delete is per-query opt-in; forgetting `.SoftDelete()` reads/destroys trashed rows, and `Delete()` always hard-deletes. | Derive soft-delete from model metadata (tags are already parsed); add a soft `Delete` that sets `deleted_at`. |
| 2.4 | 🟡 Medium | `framework/database/scanner.go:270-349` | `convertAssign` silently no-ops unconvertible non-NULL types (`[]byte`→bool/numeric, string/`[]byte`→time, `[]byte` `NUMERIC`) → "NULL-safe" but **silent data loss**. | Handle `[]byte`/string→numeric/bool/time; return an error instead of falling through to a zero value. |
| 2.5 | 🟡 Medium | `framework/database/relations.go:284` | Dead `relFieldIdx` line calls `rv.Type().Elem()` on a single struct → panics on `First(...).With(...)`. | Delete the unused `relFieldIdx` computation. |
| 2.6 | 🟡 Medium | `framework/database/relations.go:58,109,327-334` | Eager loads ignore soft-deletes, hardcode `r.id` as related PK, and resolve relations by `(table,kind)` so two relations to the same table collide. | Match on the stored `fieldIndex`; apply soft-delete filter; use the related struct's real PK column. |
| 2.7 | 🟡 Medium | `framework/validation/validator.go:221-277` | `required` rejects legitimate `0`/`false`; `min`/`max` **silently pass** on unsupported types (bypass). | Distinguish present-vs-nonzero (pointers/`Has`); return an error when `min`/`max`/`gt…` get an unsupported type. |
| 2.8 | 🟠 High | `framework/middleware/cors.go:66-69,96` | With `AllowOrigins: ["*"]` it reflects the request origin **and** sets `Allow-Credentials: true` → any site can make credentialed cross-origin requests; also no `Vary: Origin` (cache poisoning). | Never reflect origin + emit credentials for `*`; require an explicit allow-list when credentials are on; always `Add("Vary","Origin")`. |
| 2.9 | 🟡 Medium | `framework/middleware/csrf.go:74-76` (also `http` review) | `_, _ = rand.Read(b)` discards the CSPRNG error → all-zero predictable CSRF token on RNG failure. (Same pattern in session ID generation.) | Check the error and fail closed; use `subtle.ConstantTimeCompare` for token compare. |

---

## Phase 3 — CLI & developer experience (advertised-but-broken)

| # | Sev | Location | Flaw | Fix |
|---|-----|----------|------|-----|
| 3.1 | 🔴 Critical | `cmd/oni/commands.go:497,558` | `oni make:*` reads `./stubs/*.stub` from the project CWD, but `oni new` never copies stubs into the app → **every generator fails** in a scaffolded project. | Embed stubs in the CLI binary via `//go:embed stubs/*.stub`; generate from the embedded FS. |
| 3.2 | 🔴 Critical | `cmd/oni/commands.go:767-811` | Scaffolded `handleOniCmd` implements only `migrate*`; `route:list`, `db:seed`, `queue:work`, `schedule:run`, `health`, `backup`, `deploy`, `secrets`, etc. hit a `default` that just prints "command received" → ~12 advertised commands are no-ops. | Expand the scaffolded handler to wire the real subsystems (the framework APIs all exist), or have the CLI invoke them directly. |
| 3.3 | 🟠 High | `stubs/seeder.stub` | Generates `Run(ctx, *database.DB)` but `seeder.Seeder` requires `Run(ctx, seeder.DB)` → registering a generated seeder won't compile. | Change the stub to `seeder.DB` (or widen the interface). |
| 3.4 | 🟠 High | `examples/api/main.go:18-19,53-54` | Validation tags use `min:3` (colon); the validator parses `=` only → constraints silently never enforced. | Use `min=3,max=120`. |
| 3.5 | 🟡 Medium | `docs/routing.md:54`; `ROADMAP.md:26,48`; `docs/database.md:159`; `docs/getting-started.md:101`; `README.md:290` | Doc drift: `middleware.Auth(guard)` needs two args; `make:resource` is scaffold not "JSON transformer"; no `.air.toml` is scaffolded; `tx.Raw` transaction example is misleading; `key:generate` emits hex not `base64:`; README omits `make:test`/`make:policy`. | Correct each (low-risk doc edits). |

---

## Phase 4 — Tests, polish & leak cleanup

**Test coverage** — 16/28 modules have **zero tests**, including the highest-risk: `database`, `auth`, `migrations`, `secrets`, `roles`, `mail`, `storage`, `admin`, `deploy`, `backup`. Highest-value additions:
- `database`: SQL-string snapshot of `buildSelect`/`buildUpdate` (catches 0.2, 2.1), NULL round-trip into pointer **and** value fields (catches 2.4), `Paginate` total under soft-delete.
- `auth`: JWT parse (empty secret, no-exp, wrong alg), bcrypt, session regenerate-on-login.
- `roles`: wildcard over-match. `memory`: pub/sub concurrent subscribe/unsubscribe + gossip under `-race`. `middleware`: CSRF validate path.
- Add `-race` to CI (note: needs a C compiler for cgo; not available on the current box).

**Polish / smaller leaks**
- `framework/middleware/compress.go:34,54,64` — configurable `level` is a no-op; `Content-Encoding: gzip` set unconditionally (double-encodes images, mislabels 204s); wrapped writer never restored (corrupts logged status/size).
- `framework/routing/router.go:166` — no `405`/`Allow` header, no implicit `HEAD`→`GET` / auto-`OPTIONS`.
- `framework/http/upload.go:38,57` — `MultipartForm.RemoveAll()` never called (temp-file leak); nil-deref when `MultipartForm` is nil; over-permissive MIME prefix match.
- `framework/storage/s3.go:132` — `Exists` swallows all errors as "not found." `framework/storage/storage.go:57` / `config.go:145` — panic on unknown disk / missing env.
- `framework/mail/mail.go:89` — `NewFromConfig` defaults encryption to `"none"` (cleartext SMTP AUTH) while `New` defaults to `"tls"`.
- `framework/roles/rbac.go:150` — wildcard prefix match isn't segment-aware (`user*` over-grants); `framework/secrets/secrets.go:38` — APP_KEY via single unsalted SHA-256 (use a 32-byte random key / KDF); `framework/auth/guard.go:149` — bcrypt cost not configurable.

---

## Suggested execution order

1. **Phase 0** in one security pass — 0.1/0.2/0.3 (admin auth + the `QuoteIdent` identifier fix, which closes 0.2 and the admin-search injection together), then 0.4–0.9.
2. **Phase 1** — 1.1 (the `ctxState` refactor also unblocks 1.2), then 1.3/1.4 together (memory layer), then the queue/scheduler `recover()`s.
3. **Phase 2** in parallel with Phase 3 (disjoint files) once 0/1 land.
4. **Phase 4** continuously — add the test for each bug as it's fixed (regression-locks the audit).

Parallelism: 0.1, 0.4, 0.6, 0.7, 0.8 are independent files. 0.2 + 0.3 + admin-search share the `QuoteIdent` change. Phase 3 (CLI/docs) is fully disjoint from the framework fixes and can proceed any time.
