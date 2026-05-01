# Changelog — Unreleased

Changes staged for the next version. Move entries to a versioned file on release.

---

## Added

- `Builder.Exec()` — public method on the database query builder for raw write statements
  (`INSERT ... ON CONFLICT DO NOTHING`, etc.) that return no rows. Previously there was no
  way to run raw DML without going through `All()` or `Scan()`.

- `framework/database/builder.go` — `Raw(...).Exec()` path now normalises placeholders
  (`$1`, `$2`, ...) consistently with the rest of the query builder.

- OniGram test application (`testing/onigram`) demonstrating a full Instagram-style SPA
  built on OniWorks: authentication, posts, stories, real-time DMs, explore, bookmarks,
  notifications, and a seeder with real image content from free CDNs.

---

## Fixed

- **Notifications endpoint** — returned JSON `null` instead of `[]` for users with no
  notifications, causing a frontend `TypeError` ("Failed to load notifications"). Fixed by
  initialising the slice with `make([]models.Notification, 0)`.

- **Comments endpoint** — same `null` vs `[]` issue; same fix applied.

- **Feed query** — used `INNER JOIN` on the follows table, which excluded the authenticated
  user's own posts from their own feed. Changed to `LEFT JOIN` with
  `OR posts.user_id = ?` and `SELECT DISTINCT posts.*`.

- **Notification model missing `updated_at`** — migration `20260501000014` added an
  `updated_at` column to the `notifications` table but the `Notification` struct did not
  include a matching field. The scanner failed with a column-mapping error on every
  notifications request. Added `UpdatedAt *time.Time` to the model.

- **NULL `post_id` crash** — `Notification.PostID` was `int64`; follow-type notifications
  store `NULL` in that column. The scanner panicked with
  `converting NULL to int64 is unsupported`. Changed to `*int64`.

- **NULL `avatar_path` crash in raw queries** — `Suggestions` controller selected
  `avatar_path` from `users` directly; old test users had `NULL` avatar paths. The scanner
  panicked. Fixed by wrapping with `COALESCE(avatar_path, '')` in the raw SQL.

- **Production asset serving** — Vite production build outputs to `public/build/assets/`
  but no route served `/assets/*`. Added a static file route so the Go backend can fully
  self-host the compiled CSS and JS without the Vite dev server.

- **Dynamic Tailwind class names stripped** — Avatar sizes were generated as
  `w-${size} h-${size}` at runtime. Tailwind's static scanner does not detect dynamic
  class interpolations, so those classes were purged from the production CSS bundle,
  leaving images unsized. Replaced with inline `style` attributes for pixel dimensions.

---

## Changed

- Nothing yet.

---

## Removed

- Nothing yet.

---

## Known Issues / Planned Fixes

See [ROADMAP.md](../ROADMAP.md) for the full prioritised backlog.

**Critical (scheduled for v1.1):**
- Scanner does not auto-coerce `NULL` to zero values for non-pointer struct fields.
  Developers must use `*T` pointers or `COALESCE` in raw queries as a workaround.
- Request validation is not wired to `c.Bind()`. No `c.Validate()` helper exists yet.

**High (scheduled for v1.1):**
- No CLI code generation (`oni make:model`, `oni make:controller`, `oni make:migration`).
  Every file must be written by hand.

**Medium (scheduled for v1.2–v1.4):**
- No built-in pagination helper on the query builder.
- `framework/seeder` package has no CLI hook (`oni db:seed` does not exist yet).
- No OAuth / social login providers.
- No image processing (resize, WebP conversion, EXIF strip).
- No hot reload; developers must rebuild manually or configure `air` themselves.
