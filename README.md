# ReleaseNoJutsu

ReleaseNoJutsu is a personal Telegram bot that tracks MangaDex and notifies you when new chapters are released. It stores what you follow and your reading progress in SQLite, and checks for updates on a schedule (runs immediately on startup, then every 6 hours).

For a deeper architectural/workflow walkthrough (with diagrams), see `docs/workflow.md`.

## What you can do

- Track manga by MangaDex URL or UUID
- List followed manga
- Manually check a specific manga for new chapters
- Get automatic notifications for newly released chapters
- Track reading progress (mark read/unread) and keep an “unread chapters” count per manga
- Sync a manga’s full chapter history from MangaDex (so you can start from scratch)
- Use `/status` to see basic health/state (tracked counts, total unread, last scheduler run)

When a manga reaches **3+ unread chapters**, notifications include a warning.

## Quick start (Docker Compose)

1. Create your env file:
   ```bash
   cp .env.example .env
   ```
2. Edit `.env` and set:
   - `TELEGRAM_BOT_TOKEN`
   - `TELEGRAM_ALLOWED_USERS` (comma-separated Telegram user IDs)
3. Start the app:
   ```bash
   docker compose up -d --build
   ```

### Example `docker-compose.yml`

This app uses a distroless runtime image and runs as **non-root** by default, so make sure your bind-mounted folders are writable by the chosen UID/GID.

```yaml
services:
  app:
    build:
      context: .
    image: releasenojutsu:local
    container_name: releasenojutsu
    init: true
    restart: unless-stopped
    env_file:
      - ./.env
    volumes:
      - ./logs:/app/logs
      - ./database:/app/database
    user: "${PUID:-65532}:${PGID:-65532}"
```

Notes:
- If you want to run it as your host user, add to `.env`: `PUID=1000` and `PGID=1000` (or whatever `id -u` / `id -g` returns).
- If you don’t care about running non-root, you can omit `user:` and instead `chown` the host folders to `65532:65532`.

## Quick start (local Go)

Requirements:
- Go `1.25.5+`
- A C toolchain for CGO (because `github.com/mattn/go-sqlite3`): on Debian/Ubuntu, install `build-essential`

Run:
```bash
cp .env.example .env
go run ./cmd/releasenojutsu
```

It creates:
- DB at `database/ReleaseNoJutsu.db`
- Logs at `logs/ReleaseNoJutsu.log`

## Telegram setup

1. Create a bot via `@BotFather` and copy the token.
2. Get your Telegram numeric user ID (e.g. via `@userinfobot`).
3. Put them in `.env`:
   ```env
   TELEGRAM_BOT_TOKEN=...
   TELEGRAM_ALLOWED_USERS=123456789,987654321
   ```

Only users listed in `TELEGRAM_ALLOWED_USERS` can use the bot.

Important: this app uses Telegram long-polling (`getUpdates`), so **only one instance** of the bot should run for a given token. If you run multiple containers/processes you’ll see `Conflict: terminated by other getUpdates request`.

## Using the bot

Commands:
- `/start` – show the main menu
- `/help` – show help
- `/status` – status/health summary

Main menu actions:
- **Add manga**: send a MangaDex URL (e.g. `https://mangadex.org/title/<uuid>/...`) or a raw UUID
- **List followed manga**
- **Check for new chapters** (manual poll for one manga)
- **Mark chapter as read** (advances your “last read” point for that manga)
- **Sync all chapters** (imports the full chapter list for a manga; useful when starting from scratch)
- **List read chapters** (and mark a chapter as unread)
- **Remove manga**

Notifications:
- The scheduler checks for new chapters every 6 hours and sends a message when something new is found.
- Your chat is automatically registered for notifications after you interact with the bot (and you’re authorized).

## How it works (high level)

Entry point:
- `cmd/releasenojutsu/main.go` wires everything together and starts the scheduler + bot loop.

Core packages:
- `internal/bot`: Telegram commands/menus, input parsing (URL/UUID), and calling update/progress actions.
- `internal/cron`: scheduler that runs updates immediately and then every 6 hours.
- `internal/updater`: shared “check MangaDex → store chapters → update unread count → return results” logic used by both manual checks and the scheduler.
- `internal/mangadex`: HTTP client + response parsing for MangaDex endpoints.
- `internal/db`: SQLite schema + migrations and all read/write operations (manga, chapters, users, unread counts, status).
- `internal/notify`: notification sender (Telegram implementation).
- `internal/logger`: writes to stdout and `logs/ReleaseNoJutsu.log`.

Update detection:
- Update polling uses a timestamp watermark (`manga.last_seen_at`) to detect newly released chapters.
- Reading progress uses a numeric watermark (`manga.last_read_number`) so everything below that chapter number is treated as read.
- Full sync uses MangaDex paging to import the entire chapter feed into SQLite.

## Development & validation

Common checks (similar intent to “cargo fmt / cargo check”):
- Format: `gofmt -w .`
- Tests: `go test ./...`
- Vet/static checks: `go vet ./...`
- (Optional) Race detector: `go test -race ./...`
- Dependencies tidy: `go mod tidy`

## Troubleshooting

- `Conflict: terminated by other getUpdates request`: stop the other running instance; only one long-poller per bot token.
- Permission errors in Docker: your host `./logs` and `./database` must be writable by the container user (see the compose `user:` note above).
- “not authorised”: your Telegram user ID is not included in `TELEGRAM_ALLOWED_USERS`.

## License

GPLv3 (see `LICENSE`).
