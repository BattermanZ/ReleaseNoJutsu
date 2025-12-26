package db

import (
	"database/sql"

	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"releasenojutsu/internal/logger"
)

var dbMutex sync.Mutex

// DB wraps the sql.DB connection.

type DB struct {
	*sql.DB
}

type Manga struct {
	ID          int
	MangaDexID  string
	Title       string
	LastChecked time.Time
	LastSeenAt  time.Time
	LastReadAt  time.Time
}

type Status struct {
	MangaCount     int
	ChapterCount   int
	UserCount      int
	UnreadTotal    int
	CronLastRun    time.Time
	HasCronLastRun bool
}

// New opens a connection to the database.

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// Close closes the database connection.

func (db *DB) Close() error {
	return db.DB.Close()
}

// CreateTables creates the necessary tables in the database.

func (db *DB) CreateTables() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mangadex_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			last_checked TIMESTAMP,
			last_seen_at TIMESTAMP,
			last_read_at TIMESTAMP,
			unread_count INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP,
			readable_at TIMESTAMP,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			is_read BOOLEAN DEFAULT FALSE,
			FOREIGN KEY (manga_id) REFERENCES manga (id)
		);

		CREATE TABLE IF NOT EXISTS users (
			chat_id INTEGER PRIMARY KEY
		);

		CREATE TABLE IF NOT EXISTS system_status (
			key TEXT PRIMARY KEY,
			last_update TIMESTAMP
		);
	`)
	return err
}

func (db *DB) Migrate() error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	hasMangaLastSeenAt, err := db.hasColumn("manga", "last_seen_at")
	if err != nil {
		return err
	}
	if !hasMangaLastSeenAt {
		if _, err := db.Exec("ALTER TABLE manga ADD COLUMN last_seen_at TIMESTAMP"); err != nil {
			return err
		}
	}

	hasMangaLastReadAt, err := db.hasColumn("manga", "last_read_at")
	if err != nil {
		return err
	}
	if !hasMangaLastReadAt {
		if _, err := db.Exec("ALTER TABLE manga ADD COLUMN last_read_at TIMESTAMP"); err != nil {
			return err
		}
	}

	hasChaptersReadableAt, err := db.hasColumn("chapters", "readable_at")
	if err != nil {
		return err
	}
	if !hasChaptersReadableAt {
		if _, err := db.Exec("ALTER TABLE chapters ADD COLUMN readable_at TIMESTAMP"); err != nil {
			return err
		}
	}

	hasChaptersCreatedAt, err := db.hasColumn("chapters", "created_at")
	if err != nil {
		return err
	}
	if !hasChaptersCreatedAt {
		if _, err := db.Exec("ALTER TABLE chapters ADD COLUMN created_at TIMESTAMP"); err != nil {
			return err
		}
	}

	hasChaptersUpdatedAt, err := db.hasColumn("chapters", "updated_at")
	if err != nil {
		return err
	}
	if !hasChaptersUpdatedAt {
		if _, err := db.Exec("ALTER TABLE chapters ADD COLUMN updated_at TIMESTAMP"); err != nil {
			return err
		}
	}

	// Deduplicate legacy data before adding the unique index.
	// Historically, chapters were inserted with INSERT OR REPLACE but without a unique constraint,
	// so duplicates could accumulate. We keep the latest row (by id) per (manga_id, chapter_number),
	// while preserving is_read=true if any duplicate row was marked read.
	if _, err := db.Exec(`
		UPDATE chapters
		SET is_read = (
			SELECT MAX(is_read)
			FROM chapters c2
			WHERE c2.manga_id = chapters.manga_id AND c2.chapter_number = chapters.chapter_number
		)
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		DELETE FROM chapters
		WHERE id NOT IN (
			SELECT MAX(id)
			FROM chapters
			GROUP BY manga_id, chapter_number
		)
	`); err != nil {
		return err
	}

	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_chapters_manga_chapter ON chapters(manga_id, chapter_number)"); err != nil {
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_chapters_manga_is_read ON chapters(manga_id, is_read)"); err != nil {
		return err
	}

	if _, err := db.Exec(`
		UPDATE manga
		SET last_seen_at = COALESCE(
			(
				SELECT MAX(COALESCE(created_at, readable_at, published_at))
				FROM chapters
				WHERE chapters.manga_id = manga.id
			),
			last_checked
		)
		WHERE last_seen_at IS NULL
	`); err != nil {
		return err
	}

	// Backfill last_read_at from legacy per-chapter flags if present.
	if _, err := db.Exec(`
		UPDATE manga
		SET last_read_at = COALESCE(
			(
				SELECT MAX(COALESCE(created_at, readable_at, published_at))
				FROM chapters
				WHERE chapters.manga_id = manga.id AND is_read = true
			),
			last_read_at
		)
		WHERE last_read_at IS NULL
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		UPDATE manga
		SET unread_count = (
			SELECT COUNT(*)
			FROM chapters
			WHERE chapters.manga_id = manga.id
			  AND COALESCE(created_at, readable_at, published_at) > COALESCE(manga.last_read_at, '1970-01-01T00:00:00Z')
		)
	`); err != nil {
		return err
	}

	return nil
}

func (db *DB) hasColumn(tableName, columnName string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (db *DB) AddManga(mangaID, title string) (int64, error) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	result, err := db.Exec("INSERT INTO manga (mangadex_id, title, last_checked) VALUES (?, ?, ?)",
		mangaID, title, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) AddChapter(mangaID int64, chapterNumber, title string, publishedAt, readableAt, createdAt, updatedAt time.Time) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec(`
		INSERT INTO chapters (manga_id, chapter_number, title, published_at, readable_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(manga_id, chapter_number) DO UPDATE SET
			title = excluded.title,
			published_at = excluded.published_at,
			readable_at = excluded.readable_at,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`, mangaID, chapterNumber, title, publishedAt, readableAt, createdAt, updatedAt)
	return err
}

func (db *DB) GetManga(mangaID int) (string, string, time.Time, time.Time, error) {
	var mangadexID, title string
	var lastChecked time.Time
	var lastSeenAt sql.NullTime
	err := db.QueryRow("SELECT mangadex_id, title, last_checked, last_seen_at FROM manga WHERE id = ?", mangaID).
		Scan(&mangadexID, &title, &lastChecked, &lastSeenAt)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	if lastSeenAt.Valid {
		return mangadexID, title, lastChecked, lastSeenAt.Time, nil
	}
	return mangadexID, title, lastChecked, time.Time{}, nil
}

func (db *DB) UpdateMangaLastChecked(mangaID int) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec("UPDATE manga SET last_checked = ? WHERE id = ?",
		time.Now().UTC(), mangaID)
	return err
}

func (db *DB) UpdateMangaLastSeenAt(mangaID int, seenAt time.Time) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec("UPDATE manga SET last_seen_at = ? WHERE id = ?",
		seenAt.UTC(), mangaID)
	return err
}

func (db *DB) SetLastReadAt(mangaID int, readAt time.Time) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec("UPDATE manga SET last_read_at = ? WHERE id = ?", readAt.UTC(), mangaID)
	return err
}

func (db *DB) GetLastReadAt(mangaID int) (time.Time, error) {
	var lastReadAt sql.NullTime
	err := db.QueryRow("SELECT last_read_at FROM manga WHERE id = ?", mangaID).Scan(&lastReadAt)
	if err != nil {
		return time.Time{}, err
	}
	if lastReadAt.Valid {
		return lastReadAt.Time, nil
	}
	return time.Time{}, nil
}

func (db *DB) GetUnreadCount(mangaID int) (int, error) {
	return db.CountUnreadChapters(mangaID)
}

func (db *DB) CountUnreadChapters(mangaID int) (int, error) {
	var unreadCount int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM chapters
		JOIN manga ON manga.id = chapters.manga_id
		WHERE chapters.manga_id = ?
		  AND COALESCE(chapters.created_at, chapters.readable_at, chapters.published_at) > COALESCE(manga.last_read_at, '1970-01-01T00:00:00Z')
	`, mangaID).Scan(&unreadCount)
	return unreadCount, err
}

func (db *DB) RecalculateUnreadCount(mangaID int) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	return db.recalculateUnreadCountLocked(mangaID)
}

func (db *DB) recalculateUnreadCountLocked(mangaID int) error {
	_, err := db.Exec(`
		UPDATE manga
		SET unread_count = (
			SELECT COUNT(*)
			FROM chapters
			WHERE chapters.manga_id = manga.id
			  AND COALESCE(chapters.created_at, chapters.readable_at, chapters.published_at) > COALESCE(manga.last_read_at, '1970-01-01T00:00:00Z')
		)
		WHERE id = ?
	`, mangaID)
	return err
}

func (db *DB) GetAllManga() (*sql.Rows, error) {
	return db.Query("SELECT id, mangadex_id, title, last_checked, last_seen_at, last_read_at FROM manga")
}

func (db *DB) ListManga() ([]Manga, error) {
	rows, err := db.GetAllManga()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var manga []Manga
	for rows.Next() {
		var row Manga
		var lastSeenAt sql.NullTime
		var lastReadAt sql.NullTime
		if err := rows.Scan(&row.ID, &row.MangaDexID, &row.Title, &row.LastChecked, &lastSeenAt, &lastReadAt); err != nil {
			return nil, err
		}
		if lastSeenAt.Valid {
			row.LastSeenAt = lastSeenAt.Time
		}
		if lastReadAt.Valid {
			row.LastReadAt = lastReadAt.Time
		}
		manga = append(manga, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return manga, nil
}

func (db *DB) GetAllUsers() (*sql.Rows, error) {
	return db.Query("SELECT chat_id FROM users")
}

func (db *DB) ListUsers() ([]int64, error) {
	rows, err := db.GetAllUsers()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, chatID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chatIDs, nil
}

func (db *DB) EnsureUser(chatID int64) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec("INSERT OR IGNORE INTO users (chat_id) VALUES (?)", chatID)
	return err
}

func (db *DB) MarkChapterAsRead(mangaID int, chapterNumber string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	var chapterSeenAt time.Time
	err := db.QueryRow(`
		SELECT COALESCE(created_at, readable_at, published_at)
		FROM chapters
		WHERE manga_id = ? AND chapter_number = ?
	`, mangaID, chapterNumber).Scan(&chapterSeenAt)
	if err != nil {
		return err
	}

	var currentLastReadAt sql.NullTime
	err = db.QueryRow("SELECT last_read_at FROM manga WHERE id = ?", mangaID).Scan(&currentLastReadAt)
	if err != nil {
		return err
	}
	if currentLastReadAt.Valid && currentLastReadAt.Time.After(chapterSeenAt) {
		// Don't move backwards.
		chapterSeenAt = currentLastReadAt.Time
	}

	if _, err := db.Exec("UPDATE manga SET last_read_at = ? WHERE id = ?", chapterSeenAt.UTC(), mangaID); err != nil {
		return err
	}
	return db.recalculateUnreadCountLocked(mangaID)
}

func (db *DB) MarkChapterAsUnread(mangaID int, chapterNumber string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	var chapterSeenAt time.Time
	err := db.QueryRow(`
		SELECT COALESCE(created_at, readable_at, published_at)
		FROM chapters
		WHERE manga_id = ? AND chapter_number = ?
	`, mangaID, chapterNumber).Scan(&chapterSeenAt)
	if err != nil {
		return err
	}

	// Move last_read_at back to the previous chapter (by seenAt time), making this chapter and newer unread.
	var prevSeenAt sql.NullTime
	err = db.QueryRow(`
		SELECT MAX(COALESCE(created_at, readable_at, published_at))
		FROM chapters
		WHERE manga_id = ?
		  AND COALESCE(created_at, readable_at, published_at) < ?
	`, mangaID, chapterSeenAt).Scan(&prevSeenAt)
	if err != nil {
		return err
	}

	if prevSeenAt.Valid {
		if _, err := db.Exec("UPDATE manga SET last_read_at = ? WHERE id = ?", prevSeenAt.Time.UTC(), mangaID); err != nil {
			return err
		}
	} else {
		if _, err := db.Exec("UPDATE manga SET last_read_at = NULL WHERE id = ?", mangaID); err != nil {
			return err
		}
	}

	return db.recalculateUnreadCountLocked(mangaID)
}

func (db *DB) GetUnreadChapters(mangaID int) (*sql.Rows, error) {
	return db.Query(`
		SELECT chapter_number, title 
		FROM chapters 
		WHERE manga_id = ?
		  AND COALESCE(created_at, readable_at, published_at) > COALESCE((SELECT last_read_at FROM manga WHERE id = ?), '1970-01-01T00:00:00Z')
		ORDER BY 
			COALESCE(created_at, readable_at, published_at) DESC
		LIMIT 3
	`, mangaID, mangaID)
}

func (db *DB) GetReadChapters(mangaID int) (*sql.Rows, error) {
	return db.Query(`
		SELECT chapter_number, title 
		FROM chapters 
		WHERE manga_id = ?
		  AND COALESCE(created_at, readable_at, published_at) <= COALESCE((SELECT last_read_at FROM manga WHERE id = ?), '1970-01-01T00:00:00Z')
		ORDER BY 
			COALESCE(created_at, readable_at, published_at) DESC
		LIMIT 3
	`, mangaID, mangaID)
}

func (db *DB) GetMangaTitle(mangaID int) (string, error) {
	var title string
	err := db.QueryRow("SELECT title FROM manga WHERE id = ?", mangaID).Scan(&title)
	return title, err
}

func (db *DB) UpdateCronLastRun() {
	dbMutex.Lock()
	_, err := db.Exec("INSERT OR REPLACE INTO system_status (key, last_update) VALUES ('cron_last_run', ?)",
		time.Now().UTC())
	dbMutex.Unlock()
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to update cron last run time: %v", err)
	}
}

func (db *DB) DeleteManga(mangaID int) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r) // re-throw panic
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	// Delete associated chapters first
	_, err = tx.Exec("DELETE FROM chapters WHERE manga_id = ?", mangaID)
	if err != nil {
		return err
	}

	// Delete the manga
	_, err = tx.Exec("DELETE FROM manga WHERE id = ?", mangaID)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) GetStatus() (Status, error) {
	var s Status

	if err := db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&s.MangaCount); err != nil {
		return Status{}, err
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM chapters").Scan(&s.ChapterCount); err != nil {
		return Status{}, err
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&s.UserCount); err != nil {
		return Status{}, err
	}
	if err := db.QueryRow("SELECT COALESCE(SUM(unread_count), 0) FROM manga").Scan(&s.UnreadTotal); err != nil {
		return Status{}, err
	}

	var lastRun sql.NullTime
	if err := db.QueryRow("SELECT last_update FROM system_status WHERE key = 'cron_last_run'").Scan(&lastRun); err != nil && err != sql.ErrNoRows {
		return Status{}, err
	}
	if lastRun.Valid {
		s.CronLastRun = lastRun.Time
		s.HasCronLastRun = true
	}
	return s, nil
}
