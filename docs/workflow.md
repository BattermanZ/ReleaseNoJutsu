# ReleaseNoJutsu Workflow

This document explains how ReleaseNoJutsu works end-to-end (startup, Telegram interactions, scheduled updates, and persistence), with diagrams you can keep open while reading the code.

## High-Level Purpose

ReleaseNoJutsu is a Telegram bot that:

- Lets you add MangaDex titles (by URL or UUID)
- Stores tracked manga + chapters in SQLite
- Periodically polls MangaDex for new chapters
- Notifies registered Telegram chats when new chapters appear

## Runtime Topology

```mermaid
flowchart TD
  subgraph Telegram[Telegram]
    U[User]
    TG[Telegram Servers]
  end

  subgraph App[ReleaseNoJutsu process]
    BOT[Bot loop<br/>internal/bot]
    SCH[Scheduler<br/>internal/cron]
    DB[(SQLite DB<br/>database/ReleaseNoJutsu.db)]
    LOG[Logger<br/>logs/ReleaseNoJutsu.log]
    CFG[Config<br/>.env / env vars]
    MD[MangaDex client<br/>internal/mangadex]
  end

  subgraph MangaDex[MangaDex]
    API[MangaDex API<br/>api.mangadex.org]
  end

  U -- "messages / button presses" --> TG
  TG -- "updates (polling)" --> BOT
  BOT -- "send messages" --> TG

  BOT -- "read/write" --> DB
  SCH -- "read/write" --> DB
  BOT -- "HTTP" --> API
  SCH -- "HTTP" --> API

  CFG --> BOT
  CFG --> SCH

  BOT --> LOG
  SCH --> LOG
  MD --> LOG
```

## Startup Workflow

Entry point: `cmd/releasenojutsu/main.go`

```mermaid
flowchart TD
  S[Process starts] --> L[Init logger<br/>internal/logger.InitLogger]
  L --> C[Load config<br/>internal/config.Load]
  C --> D[Ensure database dir exists]
  D --> O[Open SQLite<br/>internal/db.New]
  O --> T[Create tables<br/>DB.CreateTables]
  T --> M[Create MangaDex client<br/>mangadex.NewClient]
  M --> B[Create Telegram Bot API client]
  B --> A[Construct app bot<br/>bot.New]
  A --> R[Start scheduler<br/>cron.NewScheduler.Start]
  R --> P[Start bot update loop<br/>Bot.Start]
```

Notes:

- Scheduler runs once immediately at startup and then every 6 hours.
- The bot update loop is a long-running poll (not a webhook).

## Authorization and “Who Gets Notifications”

There are two related concepts:

1. **Who is allowed to use the bot** (authorization)
2. **Who receives proactive notifications** (users table)

Authorization is controlled by `.env`:

- `TELEGRAM_ALLOWED_USERS`: Telegram user IDs that are allowed to interact

Notification recipients are stored in SQLite:

- Table `users(chat_id)` contains chat IDs to notify

The bot ensures that when an authorized chat interacts, its chat ID is inserted into `users`.

```mermaid
sequenceDiagram
  participant U as User
  participant T as Telegram
  participant B as Bot (internal/bot)
  participant DB as SQLite (users table)

  U->>T: /start (or any command)
  T->>B: Update (polling)
  B->>B: Check allowed user ID
  alt authorized
    B->>DB: INSERT OR IGNORE users(chat_id)
    B->>T: Reply / show menu
  else not authorized
    B->>T: "not authorised"
  end
```

## Core Telegram Workflows

### Main Menu -> Action Routing

Buttons send callback data (strings like `add_manga`, `select_manga:<id>:mark_read`, etc.), and the bot dispatches based on those values.

```mermaid
flowchart TD
  MM[Main menu<br/>inline keyboard] -->|add_manga| AM[Prompt for URL/ID]
  MM -->|list_manga| LM[List followed manga]
  MM -->|check_new| CN[Pick manga]
  MM -->|mark_read| MR[Pick manga]
  MM -->|list_read| LR[Pick manga]
  MM -->|remove_manga| RM[Pick manga]

  CN --> SM1["select_manga:manga_id:check_new"]
  MR --> SM2["select_manga:manga_id:mark_read"]
  LR --> SM3["select_manga:manga_id:list_read"]
  RM --> SM4["select_manga:manga_id:remove_manga"]

  SM1 --> CHK[Check MangaDex now]
  SM2 --> UCH[Show unread chapters menu]
  SM3 --> RCH[Show read chapters menu]
  SM4 --> DEL[Delete manga + chapters]
```

### Add Manga (URL or UUID)

When you add a manga, the bot fetches metadata from MangaDex to resolve a human title and stores it in SQLite. It also fetches the latest few chapters to seed your chapter list.

```mermaid
sequenceDiagram
  participant U as User
  participant T as Telegram
  participant B as Bot
  participant MD as MangaDex API
  participant DB as SQLite

  U->>T: "Add manga" button
  T->>B: callback "add_manga"
  B->>T: prompt for MangaDex URL/ID (ForceReply)

  U->>T: sends URL or UUID
  T->>B: message/update
  B->>B: parse UUID (from URL or raw)

  B->>MD: GET /manga/{uuid}
  MD-->>B: manga metadata (title)
  B->>DB: INSERT manga(mangadex_id,title,last_checked=now)
  B->>MD: GET /manga/{uuid}/feed?limit=100&offset=0 (paged)
  MD-->>B: chapter feed pages
  B->>DB: INSERT/UPDATE chapters (all pages)
  B->>T: confirmation message
```

### Manual “Check New”

The “Check New” menu option does a one-off poll for that manga and stores any new chapters it finds (same logic as the scheduler).

```mermaid
sequenceDiagram
  participant U as User
  participant T as Telegram
  participant B as Bot
  participant MD as MangaDex API
  participant DB as SQLite

  U->>T: "Check for new chapters"
  T->>B: callback "select_manga:manga_id:check_new"
  B->>DB: SELECT manga(last_checked, mangadex_id, title)
  B->>MD: GET /manga/{uuid}/feed?...
  MD-->>B: chapter feed (createdAt/readableAt/publishAt)
  B->>DB: INSERT chapters newer than last_seen_at
  B->>DB: UPDATE manga(last_seen_at=maxSeenAt, last_checked=now)
  B->>DB: Recalculate unread_count
  B->>T: show results (or "no new chapters")
```

### Mark Read / Unread (Progress Tracking)

Progress is stored per manga as a numeric watermark: `manga.last_read_number` (the highest chapter number you consider read).

Unread chapters are derived by comparing numeric chapter numbers (`CAST(chapters.chapter_number AS REAL)`) against `manga.last_read_number`. The per-manga `unread_count` is maintained as a cached summary of that query.

```mermaid
flowchart TD
  M1[Pick manga] --> R1[Pick range 1000s]
  R1 --> R2[Pick range 100s]
  R2 --> R3[Pick range 10s]
  R3 --> CH[Pick chapter X]
  CH --> SET[Set last_read_number to X]

  M2[Pick manga] --> READ[Show read chapters]
  READ --> UN[Pick chapter X]
  UN --> UNSET[Set last_read_number to previous]
```

## Scheduled Update + Notifications

The scheduler periodically scans all tracked manga and compares each chapter’s `seenAt` timestamp against `manga.last_seen_at`.

```mermaid
sequenceDiagram
  participant SCH as Scheduler
  participant DB as SQLite
  participant MD as MangaDex API
  participant TG as Telegram API

  SCH->>DB: ListManga()  (read all rows, close result set)
  loop for each manga
    SCH->>MD: GET /manga/{uuid}/feed?...
    MD-->>SCH: chapter feed
    SCH->>DB: INSERT chapters newer than last_seen_at
    SCH->>DB: UPDATE manga(last_seen_at=maxSeenAt, last_checked=now)
    SCH->>DB: Recalculate unread_count
    alt N > 0
      SCH->>DB: SELECT users(chat_id)
      loop for each chat_id
        SCH->>TG: send message
      end
    end
  end
```

### Why We Read All Manga First (SQLite Constraint)

SQLite connections are limited to 1 (`SetMaxOpenConns(1)`). Holding a `Rows` iterator open while also trying to write in the same loop can deadlock. The scheduler avoids this by reading all manga into memory first (`ListManga`) and closing the iterator before doing any writes.

## Persistence Model (SQLite)

```mermaid
erDiagram
  manga {
    int id PK
    string mangadex_id "UUID, unique"
    string title
    datetime last_checked
    datetime last_seen_at
    datetime last_read_at
    float last_read_number
    int unread_count
  }

  chapters {
    int id PK
    int manga_id FK
    string chapter_number
    string title
    datetime published_at
    datetime readable_at
    datetime created_at
    datetime updated_at
  }

  users {
    long chat_id PK
  }

  system_status {
    string key PK
    datetime last_update
  }

  manga ||--o{ chapters : "has many"
```

## Operational Notes

- Environment:
  - `TELEGRAM_BOT_TOKEN` must be set or the bot won’t start.
  - `TELEGRAM_ALLOWED_USERS` must include your Telegram **user ID** (not chat ID) or you’ll be rejected.
- Files:
  - Logs: `logs/ReleaseNoJutsu.log`
  - DB: `database/ReleaseNoJutsu.db`
- Network:
  - MangaDex calls use a 10s timeout with retries/backoff; slow networking can make updates take longer.

## Known Limitations / Implementation Details

- `unread_count` increments when new chapters are found; it is not decremented when you mark chapters read (so it can drift from “actual unread chapters”).
- Chapter insertion uses `INSERT OR REPLACE` without a unique constraint on `(manga_id, chapter_number)`, so “replace” behavior depends on SQLite row identity rather than a logical unique key.

## Tests Map

These tests provide coverage for core behavior:

- MangaDex client parsing and URL extraction: `internal/mangadex/client_test.go`
- SQLite invariants (including “no deadlock after listing manga”): `internal/db/database_test.go`
- Scheduler update behavior and deadlock regression: `internal/cron/cron_test.go`
