# OniGram — Framework Testing Notes

> Discoveries, issues, and fixes found while building OniGram (an Instagram clone)
> using only the OniWorks framework. Each issue includes root cause and fix applied.

---

## Issue #1 — `--oni-cmd` flag not parsed in scaffold `main.go`

**Symptom**: Running `oni migrate` from within a generated app project did nothing.
`runAppCommand` (in `cmd/oni/commands.go`) correctly forwards the command as
`go run . --oni-cmd=migrate`, but the scaffold's `main.go` contained no code to
parse `--oni-cmd` — so the flag was silently ignored and the server started normally.

**Root cause**: The `mainGoStub` constant in `cmd/oni/commands.go` only called `oni.Serve()`.
It had no `os.Args` parsing or migration dispatch logic.

**Fix**: Updated `mainGoStub` to:
1. Loop over `os.Args[1:]` and detect `--oni-cmd=<cmd>` before normal boot.
2. Added `handleOniCmd(cmd string) error` which opens a DB, calls
   `migrations.LoadRegistry()`, and dispatches to `Migrate`, `Rollback`, etc.
3. Added `getEnvOrDefault` helper used by `handleOniCmd`.

**Files changed**: `cmd/oni/commands.go` (`mainGoStub`)

---

## Issue #2 — `make:channel` output went to project root instead of `app/channels/`

**Symptom**: Running `oni make:channel NotifyChannel` created `notify_channel.go`
in the current directory (project root) instead of `app/channels/notify_channel.go`.

**Root cause**: The `stubOutputPath` function in `cmd/oni/commands.go` had no explicit
`case "channel"` — it fell through to `default: return ".", snake + ".go"`.

**Fix**: Added:
```go
case "channel":
    return filepath.Join("app", "channels"), snake + "_channel.go"
```

**Files changed**: `cmd/oni/commands.go` (`stubOutputPath`)

---

## Issue #3 — `oni serve -p <port>` flag not applied to the running process

**Symptom**: Running `oni serve -p 3001` printed "Starting on :3001" but the Go
process still listened on port 8080 (from `config/app.yaml`).

**Root cause**: `serveCmd` captured the `port` flag but passed it only to the `fmt.Printf`
line — it was never forwarded to the spawned `go run .` process.

**Fix**: Set `APP_PORT=<port>` as an environment variable on the spawned process:
```go
env := append(os.Environ(), "APP_PORT="+port)
c.Env = env
```
The scaffold `main.go` should read `APP_PORT` to override `server.port` from config.
For OniGram we read the port via `config/app.yaml` directly; this fix makes the
env var available for apps that choose to read it.

**Files changed**: `cmd/oni/commands.go` (`serveCmd`)

---

## Issue #4 — Documentation says Go 1.22+, but `go.mod` uses `go 1.25.0`

**Symptom**: `docs/getting-started.md` stated "OniWorks requires Go 1.22 or later"
but `go.mod` specifies `go 1.25.0`, causing potential confusion.

**Fix**: Updated `docs/getting-started.md` to say "Go 1.25 or later".

**Files changed**: `docs/getting-started.md`

---

## Issue #5 — `docs/routing.md` was missing entirely

**Symptom**: There was no routing documentation despite routing being a core framework feature.

**Fix**: Created `docs/routing.md` covering route registration, URL parameters, groups,
middleware application, wildcards, static file serving, and the full method reference.

**Files changed**: `docs/routing.md` (new)

---

## Issue #6 — `docs/auth/` folder was missing entirely

**Symptom**: No authentication documentation existed despite the framework shipping a
full `framework/auth` package with JWT + session guard.

**Fix**: Created `docs/auth.md` covering the `auth.User` interface, guard setup, password
hashing, JWT issue/validate flow, auth middleware pattern, session auth, and error reference.

**Files changed**: `docs/auth.md` (new)

---

## Issue #7 — `gorm.Expr` referenced in `docs/database.md` — not a framework type

**Symptom**: The transactions example in `docs/database.md` used `gorm.Expr("balance - ?", amount)`.
`gorm` is not imported anywhere in OniWorks — using it would cause a compile error.

**Root cause**: Copy-paste from GORM documentation during initial authoring.

**Fix**: Replaced with the correct OniWorks pattern using a raw string or `db.Raw()` expression,
and added a note about simple arithmetic updates using string literals.

**Files changed**: `docs/database.md`

---

## Issue #8 — `Pluck` documented with wrong signature

**Symptom**: `docs/database.md` showed `emails, err := db.Table("users").Pluck("email")` returning
`[]any`. The actual `Builder.Pluck` signature is `Pluck(col string, dest any) error`.

**Fix**: Updated the example:
```go
var emails []string
err = db.Table("users").Pluck("email", &emails)
```

**Files changed**: `docs/database.md`

---

## Issue #9 — Migration stub had wrong `Up`/`Down` signatures (returned `error`)

**Symptom**: `stubs/migration.stub` generated:
```go
func (m *Migration) Up(schema *migrations.Schema) error {
    return schema.Create(...)  // compile error: schema.Create returns nothing
}
```
Two problems:
1. `schema.Create()` has no return value, so `return schema.Create(...)` is a compile error.
2. The `Migration` interface in `migrator.go` declares `Up(s *Schema)` (no error return),
   so any struct with `Up(...) error` does not satisfy the interface.

**Fix**: Rewrote `stubs/migration.stub` with correct signatures:
```go
func (m *MigrationXXX) Up(s *onimig.Schema) {
    s.Create("table", func(t *onimig.Table) { ... })
}
func (m *MigrationXXX) Down(s *onimig.Schema) {
    s.DropIfExists("table")
}
```
Also used the aliased import `onimig "github.com/onipixel/oniworks/framework/migrations"` to
avoid a package name conflict (user's `database/migrations` package is also named `migrations`).

**Files changed**: `stubs/migration.stub`

---

## Issue #10 — No `oni db:create` / `oni db:drop` command

**Symptom**: The framework had no way to create the application database from the CLI.
Users had to drop to `psql` or a GUI to create the database before running migrations.
This is a friction point that every new project encounters.

**Fix**: Added `db:create` and `db:drop` commands to `cmd/oni/commands.go`:
- `db:create` reads `DB_DRIVER`, `DB_NAME`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`
  from `.env` and creates the database using a direct SQL connection to the system DB.
  Supports PostgreSQL (`dbname=postgres`) and MySQL.
- `db:drop` prompts for confirmation then drops the database.
- Added `readDotEnv()` and `getEnv()` helpers in `cmd/oni/commands.go`.
- Added `db:create` and `db:drop` to the CLI documentation in `docs/cli.md`.

**Files changed**: `cmd/oni/commands.go`, `cmd/oni/main.go`, `docs/cli.md`

---

## Issue #11 — `oni make:*` generators fail inside generated app projects

**Symptom**: Running `oni make:controller Post` from within a generated app project fails with:
```
stub not found: ./stubs/controller.stub
```
The `makeStub()` function looks for stubs in `./stubs/<kind>.stub` relative to CWD,
but generated projects don't have a `stubs/` directory.

**Root cause**: Stubs are stored in the framework source tree (`voidfw/stubs/`) and read
at runtime from CWD. There is no mechanism to copy stubs into generated projects or to
embed them in the CLI binary.

**Recommended fix** (not yet applied): Embed the stubs into the `oni` binary using Go's
`//go:embed` directive:
```go
//go:embed stubs/*
var stubsFS embed.FS
```
Then read from `stubsFS` instead of `os.ReadFile`.

**Workaround**: Run generators from the framework source directory, or copy the
`stubs/` folder into the generated project.

---

## Issue #12 — `vite.config.ts` `publicDir` warning

**Symptom**: `npm run build` warns:
```
(!) The public directory feature may not work correctly. outDir and publicDir are not separate folders.
```

**Root cause**: The scaffold sets `outDir: 'public/build'` (inside `public/`) while Vite's
default `publicDir` is `public/`. Since `outDir` is a subdirectory of `publicDir`,
Vite warns about the overlap.

**Fix**: Set `publicDir: false` in `vite.config.ts` (same fix as OniMail project).
Updated the docs in `docs/frontend.md` with a note about this.

---

---

## Issue #13 — `normalizePlaceholders(buildSelect())` wraps args in extra slice

**Symptom**: All `Where()` queries with parameters failed at runtime with
`"expected N arguments, got 1"` from the PostgreSQL driver (pgx). The query SQL
was built correctly (e.g. `$1`, `$2`) but the driver received only 1 argument —
a `[]any` slice — instead of the individual values.

**Root cause**: In `framework/database/builder.go`, all terminal methods
(`First`, `All`, `Count`, `Pluck`) called:
```go
query, args := b.normalizePlaceholders(b.buildSelect())
```
In Go, `f(g())` where `g` returns `(string, []any)` and `f` is declared as
`func(string, ...any)` passes the `[]any` as a **single `any` element** in the
variadic — it is **not** spread. So `args` inside `normalizePlaceholders` became
`[]any{[]any{...}}` (one element: the whole args slice), and the database driver
received 1 argument regardless of how many `$N` placeholders were in the query.

**Fix**: Split the call at all four sites:
```go
// Before (broken):
query, args := b.normalizePlaceholders(b.buildSelect())

// After (correct):
rawQ, rawA := b.buildSelect()
query, args := b.normalizePlaceholders(rawQ, rawA...)
```

**Files changed**: `framework/database/builder.go` (4 call sites in `First`,
`All`, `Count`, `Pluck`)

---

## Issue #14 — Missing `primaryKey,autoIncrement` in model `db` tags

**Symptom**: After fixing issue #13, `Insert()` included `id = 0` in the SQL for
all new records, preventing the framework from using `RETURNING "id"` to populate
the auto-incremented primary key. The `user.ID` field remained 0 after insert.

**Root cause**: The OniWorks database scanner only treats an `id` column as an
auto-increment primary key when the `db` struct tag explicitly includes
`primaryKey,autoIncrement` options (e.g. `db:"id,primaryKey,autoIncrement"`).
All OniGram model structs used the bare tag `db:"id"` which has no options, so
`hasAutoID()` returned false and the framework included `id` in the INSERT
column list (with value 0) and skipped the `RETURNING "id"` path.

**Fix**: Updated all OniGram model struct `ID` fields:
```go
// Before:
ID int64 `db:"id" json:"id"`

// After:
ID int64 `db:"id,primaryKey,autoIncrement" json:"id"`
```

**Files changed**: `testing/onigram/app/models/user.go`, `post.go`, `like.go`,
`comment.go`, `follow.go`, `notification.go`

---

## Issue #15 — `Post.Show` enriched a value copy, discarding like_count / is_liked

**Symptom**: `GET /api/posts/:id` returned a post without `like_count` or
`is_liked`, even though the same data was correct in the feed.

**Root cause**: The `Show` handler called:
```go
enrichPosts([]models.Post{post}, viewerID)
return c.JSON(200, post)
```
`[]models.Post{post}` creates a new slice from the `post` value — `enrichPosts`
modifies elements of that temporary slice, but the original `post` local variable
is unaffected.

**Fix**: Keep the slice and return from it:
```go
ps := []models.Post{post}
enrichPosts(ps, viewerID)
return c.JSON(200, ps[0])
```

**Files changed**: `testing/onigram/app/http/controllers/post_controller.go`

---

## Issue #16 — Framework `defaultErrorHandler` silently discarded non-HTTP errors

**Symptom**: Internal server errors (500s) showed only `{"error":"internal server
error"}` with no diagnostic information — impossible to debug without modifying
handler code.

**Root cause**: `defaultErrorHandler` in `framework/routing/router.go` only
called `slog.Error` for `HTTPError` values. For any other error (DB errors,
panics, etc.) the error was silently swallowed.

**Fix**: Added an `slog.Error` log for unhandled errors:
```go
} else {
    slog.Error("unhandled handler error", "method", c.Method(), "path", c.Path(), "error", err)
}
```

**Files changed**: `framework/routing/router.go`

---

---

## Issue #17 — No dev-mode error page; 500s are silent and undiagnosable

**Symptom**: Any unhandled server error returned `{"error":"internal server error"}`
with no stack trace, no error type, and no request context — making debugging
impossible without adding `fmt.Println` to the handler.

**Root cause**: The framework had a single `defaultErrorHandler` that returned
clean JSON for all errors regardless of environment. There was no concept of a
debug/dev error page.

**Fix**: Created `framework/errors` package with a `Handler(debugMode bool)` function:
- **Production** (`APP_DEBUG=false`): returns `{"error":"internal server error"}` (unchanged)
- **Dev** (`APP_DEBUG=true`) + browser request (`Accept: text/html`): renders a
  full-screen HTML error page showing error type, message, and a colour-coded stack
  trace (application frames highlighted in blue, framework/stdlib dimmed)
- **Dev** + API request: returns verbose JSON with error type, message, and parsed
  stack frames

`Application.Serve()` now calls `Router.OnError(fwerrors.Handler(a.IsDebug()))` 
automatically, so every OniWorks app gets the dev error page for free when
`APP_DEBUG=true` is set in `.env`.

Known `HTTPError` values (4xx + explicit 5xx) always get clean JSON regardless of
mode — the detailed page only appears for genuinely unexpected errors.

**Files changed**:
- `framework/errors/handler.go` (new)
- `framework/app/application.go` (auto-wires error handler in `Serve()`)
- `framework/routing/router.go` (removed manual `slog.Error` workaround)

---

## Summary of All Fixes Applied

| # | Issue | Files Changed |
|---|-------|--------------|
| 1 | `--oni-cmd` not parsed | `cmd/oni/commands.go` |
| 2 | `make:channel` wrong output path | `cmd/oni/commands.go` |
| 3 | `oni serve -p` port not forwarded | `cmd/oni/commands.go` |
| 4 | Go version mismatch in docs | `docs/getting-started.md` |
| 5 | Routing docs missing | `docs/routing.md` (new) |
| 6 | Auth docs missing | `docs/auth.md` (new) |
| 7 | `gorm.Expr` in docs | `docs/database.md` |
| 8 | `Pluck` wrong signature in docs | `docs/database.md` |
| 9 | Migration stub compile error | `stubs/migration.stub`, `framework/migrations/migrator.go` |
| 10 | No `db:create` / `db:drop` CLI | `cmd/oni/commands.go`, `cmd/oni/main.go`, `docs/cli.md` |
| 11 | `make:*` fails in app projects | (documented, `//go:embed` fix recommended) |
| 12 | Vite `publicDir` warning | `vite.config.ts`, `docs/frontend.md` |
| 13 | `normalizePlaceholders(buildSelect())` wraps args in extra slice | `framework/database/builder.go` |
| 14 | Missing `primaryKey,autoIncrement` in model `db` tags | all 6 model files |
| 15 | `Post.Show` enriched a value copy | `app/http/controllers/post_controller.go` |
| 16 | `defaultErrorHandler` silently discarded non-HTTP errors | `framework/routing/router.go` |
| 17 | No dev-mode error page; 500s undiagnosable | `framework/errors/handler.go` (new), `framework/app/application.go` |

---

## OniGram Project Summary

Built a full Instagram-clone API + TypeScript/Tailwind frontend:

- **Auth**: JWT-based, `POST /api/auth/register`, `POST /api/auth/login`
- **Posts**: Photo upload (`multipart/form-data`), caption, like/unlike
- **Feed**: Posts from followed users with batch-loaded authors + like counts (no N+1)
- **Comments**: Nested under posts, batch-loaded authors
- **Follows**: Asymmetric follow/unfollow, follower/following counts
- **Notifications**: Like/comment/follow events, realtime delivery via Oni Socket
- **Realtime**: WebSocket hub with per-user `notify.{user_id}` channels; JWT auth on upgrade
- **Frontend**: SPA with client-side router, Tailwind CSS dark theme, TypeScript fetch client

All 6 migrations ran successfully. Backend builds and starts. Frontend builds with Vite (40KB JS + 16KB CSS gzipped).
