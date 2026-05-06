# OniGram

A full Instagram clone built entirely on the OniWorks framework. Used as a real-world stress test to validate every layer of the framework against production-complexity requirements.

## Features

- **Auth** — JWT-based register/login with `c.Validate()` struct-tag validation, protected routes, optional guest access
- **Feed** — personalized home feed from followed users, infinite scroll, live trending hashtags sidebar
- **Posts** — photo upload (single or carousel up to 10 images), captions, like/unlike, bookmark, comment count
- **Carousel** — multi-image posts with swipeable carousel in feed and lightbox
- **Comments** — threaded replies (parent/child), likes, user attribution, batch-enriched (no N+1)
- **Hashtags** — auto-extracted from captions, dedicated feed per tag, trending ranked by post count
- **Stories** — 24-hour expiry, story bar, viewer with progress bars, 5s auto-advance
- **Explore** — discovery grid with user/hashtag search, trending hashtags sidebar, hashtag feed mode
- **Profiles** — avatar upload, bio/website editing, follower/following lists, follow/unfollow
- **Bookmarks** — saved posts collection per user
- **Direct Messages** — 1-on-1 conversations, inbox, real-time incoming messages
- **Notifications** — like/comment/follow events, mark read, real-time badge updates
- **Real-time** — WebSocket push for notifications and DMs via Oni Socket

## Stack

- **Go 1.25** — OniWorks framework (routing, auth, DB, realtime, uploads)
- **PostgreSQL** — 14-table schema with proper indexes and constraints
- **TypeScript + Vite + Tailwind CSS v4** — SPA frontend, dark theme, mobile-responsive
- **Oni Socket** — WebSocket hub for live notifications and DMs
- **JWT** — stateless authentication via `Authorization: Bearer <token>`

## Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL running on `localhost:5432`
- Node.js 18+ (for frontend)

### Setup

```bash
cd testing/onigram

# Create the database
createdb onigram

# Configure environment
cp .env.example .env
# Edit .env with your DB credentials

# Run migrations
oni migrate

# Seed with sample data (12 users, 50+ posts, stories, DMs, notifications)
go run ./cmd/seed

# Start backend
go run . 
# or with hot reload:
air

# Start frontend dev server (separate terminal)
npm install
npm run dev
```

App runs at **http://localhost:8080** (backend) — Vite proxies `/api`, `/storage`, `/ws` automatically.

### Environment

```env
APP_NAME=OniGram
APP_ENV=local
APP_KEY=your-32-byte-secret-here

DB_DRIVER=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=onigram
DB_USER=postgres
DB_PASSWORD=password
```

### Production Build

```bash
npm run build     # compiles frontend to public/build/
APP_ENV=production go run .
```

## Project Structure

```
testing/onigram/
├── main.go                      # Routes, middleware, hub setup, CLI handler
├── app/
│   ├── http/
│   │   ├── controllers/         # 10 controllers (auth, user, post, comment, hashtag, ...)
│   │   └── middleware/          # auth.go (required + optional JWT)
│   ├── channels/                # notify_channel.go, dm_channel.go
│   └── models/                  # 12 model structs with JSON tags
├── database/
│   └── migrations/              # 18 timestamped migration files
├── cmd/
│   └── seed/                    # Full seeder: 12 users, posts, hashtags, stories, DMs
├── resources/
│   └── ts/                      # 12 TypeScript modules (api, router, feed, explore, ...)
├── config/
│   └── app.yaml
├── go.mod                       # replaces github.com/onipixel/oniworks => ../../
├── package.json
└── vite.config.ts
```

## Seeded Accounts

After running the seeder, log in with any of these (all share password `password`):

`ryu_street` · `bella_bakes` · `omar_lens` · `zoe_moves` · `alex_captures` · `sarah_travels` · `mike_eats` · `luna_art` · `jake_fitness` · `emma_style` · `david_builds` · `maya_nature`

## OniWorks APIs Used

| Feature | Framework Package |
|---------|------------------|
| HTTP routing + groups | `framework/routing` |
| Request context / auth | `framework/http` |
| Bind + validate (struct tags) | `framework/http` (`c.Validate`) |
| File uploads (single & multi) | `framework/http` (`c.ParseUpload`) |
| JWT auth | `framework/auth` |
| PostgreSQL query builder | `framework/database` |
| Database migrations | `framework/migrations` |
| WebSocket hub | `framework/realtime` |
| Vite asset injection | `framework/frontend` |
| Structured logging | `framework/logging` |
| CORS / Logger / Recovery | `framework/middleware` |
