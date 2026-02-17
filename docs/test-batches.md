# Test Implementation Batches

This file captures the agreed phased approach to close current test gaps safely.

## Batch 1: Auth, Pairing, and Migration Safety

### Goal
Cover the highest-risk paths first: user authorization, pairing flow, and legacy DB migration branches.

### Scope
- `internal/bot/telegram.go`
- `internal/bot/pairing.go`
- `internal/db/migrate_schema.go`
- `internal/db/migrate_data.go`

### Tests to add
1. `isPrivateChat` and private-chat gate behavior for messages and callbacks.
2. `isAuthorized` behavior:
- cache hit
- admin bypass
- DB authorized user
- DB error path
3. `tryHandlePairingCode` behavior:
- invalid format returns false
- valid code in group returns private-only warning
- already authorized user returns already-auth message
- expired/used/unknown code returns invalid
- successful redemption authorizes and stores user
4. `/genpair` handler behavior:
- admin allowed
- non-admin blocked
- storage error handled
5. migration legacy branches:
- `rebuildMangaTable` path for legacy unique `mangadex_id`
- `backfillLastReadAtFromLegacyReadFlags`
- `backfillLastReadNumberFromLastReadAt`
- `backfillLastReadNumberFromLegacyReadFlags`

### Done criteria
- `go test ./...` green.
- No migration regression on existing DB fixtures.

---

## Batch 2: Core User Workflows (Read/Unread/Manga Actions)

### Goal
Stabilize the most-used interactive flows and callback parsing.

### Scope
- `internal/bot/mark_read.go`
- `internal/bot/mark_unread.go`
- `internal/bot/manga.go`
- `internal/bot/manga_add.go`
- `internal/bot/callbacks.go`
- `internal/bot/menus_util.go`

### Tests to add
1. Callback parse and callback builder roundtrip coverage:
- `parsePick`, `parsePage`, `parseChapterPage`, `parseSingleManga`, `parseStartBack`
- malformed payloads return errors
2. Mark-read bucket flow:
- direct chapters when unread <= 10
- thousands/hundreds/tens navigation when large unread count
- chapter page pagination bounds (negative page, too high page)
- back navigation callback wiring
3. Mark-unread symmetry:
- same scenarios as mark-read
- chapter labels with/without title
4. Manga action handlers:
- remove confirmation message and destructive action path
- mark-all-read confirm and success path
- details rendering with/without optional fields
- toggle Manga Plus success and error path
5. Add manga flow:
- manga title fallback behavior
- confirm-add DB insert path
- sync start/sync failure message path

### Done criteria
- `go test ./...` green.
- No callback payload parse panics.

---

## Batch 3: Platform and Reliability Tests

### Goal
Add guardrails for config/runtime behavior and API retry logic.

### Scope
- `internal/config/config.go`
- `internal/cron/cron.go`
- `internal/mangadex/client.go`
- `internal/db/system_status.go`
- `internal/db/users.go`
- `internal/notify/notifier.go`
- `internal/logger/logger.go`
- `cmd/releasenojutsu/main.go` (light smoke coverage)

### Tests to add
1. Config load/validate:
- missing token
- empty `TELEGRAM_ALLOWED_USERS`
- malformed user IDs
- whitespace handling
2. Scheduler behavior:
- `Run()` lifecycle start/stop
- overlap protection (`running` CAS) under concurrent triggers
3. MangaDex client retry logic:
- 429 with numeric `Retry-After`
- 429 with HTTP-date `Retry-After`
- context cancellation during retry sleep
- `GetManga` error handling
4. Status and user DB methods:
- `UpdateCronLastRun`, `GetStatus`, `GetStatusByUser`
- `ListUsers`, `IsUserAuthorized`
5. notify/logger smoke tests:
- `SendHTML` message shape through mock API
- logger init/logging non-crash behavior
6. main wiring smoke:
- non-network startup failure modes are reported cleanly

### Done criteria
- `go test ./...` green.
- Coverage increase in `bot`, `db`, `mangadex`, `config`, `cron`.

---

## Execution Notes

1. Keep each batch as a separate commit for easy rollback.
2. Run after each batch:
- `go test ./...`
- `go vet ./...`
3. If a flaky async test appears, fix determinism before moving to next batch.
