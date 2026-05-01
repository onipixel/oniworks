# Changelog — Unreleased

Changes staged for the next version. Move entries to a versioned file on release.

---

## Added

- Nothing yet

## Fixed

- `notifications` endpoint returned JSON `null` instead of `[]` for users with no notifications, causing a frontend TypeError ("Failed to load notifications")
- `comments` endpoint returned JSON `null` instead of `[]` for posts with no comments
- Feed query used `INNER JOIN` on follows table, excluding the authenticated user's own posts; changed to `LEFT JOIN` with `OR posts.user_id = ?` so the feed shows own posts plus followed users' posts

## Changed

- Nothing yet

## Removed

- Nothing yet
