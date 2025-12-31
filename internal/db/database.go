package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
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
	ID             int
	MangaDexID     string
	Title          string
	LastChecked    time.Time
	LastSeenAt     time.Time
	LastReadNumber float64
	UnreadCount    int
}

type Status struct {
	MangaCount     int
	ChapterCount   int
	UserCount      int
	UnreadTotal    int
	CronLastRun    time.Time
	HasCronLastRun bool
}

type ChapterListItem struct {
	Number string
	Title  string
	SeenAt time.Time
}

type MangaDetails struct {
	ID                  int
	MangaDexID           string
	Title               string
	HasLastChecked       bool
	LastChecked          time.Time
	HasLastSeenAt        bool
	LastSeenAt           time.Time
	HasLastReadNumber    bool
	LastReadNumber       float64
	UnreadCount          int
	ChaptersTotal        int
	NumericChaptersTotal int
	HasMinNumber         bool
	MinNumber            float64
	HasMaxNumber         bool
	MaxNumber            float64
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
			last_read_number REAL,
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

	hasMangaLastReadNumber, err := db.hasColumn("manga", "last_read_number")
	if err != nil {
		return err
	}
	if !hasMangaLastReadNumber {
		if _, err := db.Exec("ALTER TABLE manga ADD COLUMN last_read_number REAL"); err != nil {
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

	hasChaptersIsRead, err := db.hasColumn("chapters", "is_read")
	if err != nil {
		return err
	}

	// Deduplicate legacy data before adding the unique index.
	// Historically, chapters were inserted with INSERT OR REPLACE but without a unique constraint,
	// so duplicates could accumulate. We keep the latest row (by id) per (manga_id, chapter_number),
	// while preserving is_read=true if any duplicate row was marked read.
	if hasChaptersIsRead {
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
	if hasMangaLastReadAt && hasChaptersIsRead {
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
	}

	// Backfill last_read_number from last_read_at (numeric chapters only).
	if hasMangaLastReadAt {
		if _, err := db.Exec(`
			UPDATE manga
				SET last_read_number = (
					SELECT MAX(CAST(chapter_number AS REAL))
					FROM chapters
					WHERE chapters.manga_id = manga.id
					  AND chapter_number GLOB '[0-9]*'
					  AND chapter_number NOT GLOB '*[^0-9.]*'
					  AND chapter_number NOT GLOB '*.*.*'
					  AND COALESCE(created_at, readable_at, published_at) <= COALESCE(manga.last_read_at, '1970-01-01T00:00:00Z')
				)
				WHERE last_read_number IS NULL AND last_read_at IS NOT NULL
			`); err != nil {
			return err
		}
	}

	// Backfill last_read_number from legacy per-chapter flags (numeric chapters only).
	if hasChaptersIsRead {
		if _, err := db.Exec(`
			UPDATE manga
				SET last_read_number = (
					SELECT MAX(CAST(chapter_number AS REAL))
					FROM chapters
					WHERE chapters.manga_id = manga.id
					  AND chapter_number GLOB '[0-9]*'
					  AND chapter_number NOT GLOB '*[^0-9.]*'
					  AND chapter_number NOT GLOB '*.*.*'
					  AND is_read = true
				)
				WHERE last_read_number IS NULL
			`); err != nil {
			return err
		}
	}

	if _, err := db.Exec(`
		UPDATE manga
			SET unread_count = (
				SELECT COUNT(*)
				FROM chapters
				WHERE chapters.manga_id = manga.id
				  AND chapter_number GLOB '[0-9]*'
				  AND chapter_number NOT GLOB '*[^0-9.]*'
				  AND chapter_number NOT GLOB '*.*.*'
				  AND CAST(chapter_number AS REAL) > COALESCE(manga.last_read_number, -1)
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

func (db *DB) GetLastReadNumber(mangaID int) (float64, bool, error) {
	var n sql.NullFloat64
	if err := db.QueryRow("SELECT last_read_number FROM manga WHERE id = ?", mangaID).Scan(&n); err != nil {
		return 0, false, err
	}
	if !n.Valid {
		return 0, false, nil
	}
	return n.Float64, true, nil
}

func (db *DB) GetUnreadCount(mangaID int) (int, error) {
	return db.CountUnreadChapters(mangaID)
}

func (db *DB) CountReadChapters(mangaID int) (int, error) {
	var readCount int
	err := db.QueryRow(`
			SELECT COUNT(*)
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) <= COALESCE(manga.last_read_number, -1)
		`, mangaID).Scan(&readCount)
		return readCount, err
	}

func (db *DB) ListUnreadBucketStarts(mangaID int, bucketSize int, rangeStart, rangeEnd float64) ([]int, error) {
	if bucketSize <= 0 {
		return nil, fmt.Errorf("invalid bucketSize: %d", bucketSize)
	}

	expr := "CAST(CAST(chapters.chapter_number AS REAL) / ? AS INT) * ?"
	args := []any{bucketSize, bucketSize}
	// When the parent range starts at 1, we'd rather show a "1-..." bucket instead of "0-...".
	if rangeStart == 1 {
		expr = "CASE WHEN CAST(chapters.chapter_number AS REAL) < ? THEN 1 ELSE CAST(CAST(chapters.chapter_number AS REAL) / ? AS INT) * ? END"
		args = []any{bucketSize, bucketSize, bucketSize}
	}

	q := fmt.Sprintf(`
			SELECT DISTINCT %s AS bucket_start
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) > COALESCE(manga.last_read_number, -1)
			  AND CAST(chapters.chapter_number AS REAL) >= ?
			  AND CAST(chapters.chapter_number AS REAL) < ?
			ORDER BY bucket_start ASC
	`, expr)

	args = append(args, mangaID, rangeStart, rangeEnd)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var starts []int
	for rows.Next() {
		var start int
		if err := rows.Scan(&start); err != nil {
			return nil, err
		}
		starts = append(starts, start)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return starts, nil
}

func (db *DB) ListReadBucketStarts(mangaID int, bucketSize int, rangeStart, rangeEnd float64) ([]int, error) {
	if bucketSize <= 0 {
		return nil, fmt.Errorf("invalid bucketSize: %d", bucketSize)
	}

	expr := "CAST(CAST(chapters.chapter_number AS REAL) / ? AS INT) * ?"
	args := []any{bucketSize, bucketSize}
	if rangeStart == 1 {
		expr = "CASE WHEN CAST(chapters.chapter_number AS REAL) < ? THEN 1 ELSE CAST(CAST(chapters.chapter_number AS REAL) / ? AS INT) * ? END"
		args = []any{bucketSize, bucketSize, bucketSize}
	}

	q := fmt.Sprintf(`
			SELECT DISTINCT %s AS bucket_start
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) <= COALESCE(manga.last_read_number, -1)
			  AND CAST(chapters.chapter_number AS REAL) >= ?
			  AND CAST(chapters.chapter_number AS REAL) < ?
			ORDER BY bucket_start DESC
	`, expr)

	args = append(args, mangaID, rangeStart, rangeEnd)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var starts []int
	for rows.Next() {
		var start int
		if err := rows.Scan(&start); err != nil {
			return nil, err
		}
		starts = append(starts, start)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return starts, nil
}

func (db *DB) CountUnreadNumericChaptersInRange(mangaID int, start, end float64) (int, error) {
	var cnt int
	err := db.QueryRow(`
			SELECT COUNT(*)
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) > COALESCE(manga.last_read_number, -1)
			  AND CAST(chapters.chapter_number AS REAL) >= ?
			  AND CAST(chapters.chapter_number AS REAL) < ?
	`, mangaID, start, end).Scan(&cnt)
	return cnt, err
}

func (db *DB) CountReadNumericChaptersInRange(mangaID int, start, end float64) (int, error) {
	var cnt int
	err := db.QueryRow(`
			SELECT COUNT(*)
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) <= COALESCE(manga.last_read_number, -1)
			  AND CAST(chapters.chapter_number AS REAL) >= ?
			  AND CAST(chapters.chapter_number AS REAL) < ?
	`, mangaID, start, end).Scan(&cnt)
	return cnt, err
}

func (db *DB) ListUnreadNumericChaptersInRange(mangaID int, start, end float64, limit, offset int) ([]ChapterListItem, error) {
	rows, err := db.Query(`
		SELECT chapter_number, COALESCE(chapters.title, ''), COALESCE(created_at, readable_at, published_at) AS seen_at
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) > COALESCE(manga.last_read_number, -1)
			  AND CAST(chapters.chapter_number AS REAL) >= ?
			  AND CAST(chapters.chapter_number AS REAL) < ?
		ORDER BY CAST(chapters.chapter_number AS REAL) ASC
		LIMIT ? OFFSET ?
	`, mangaID, start, end, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []ChapterListItem
	for rows.Next() {
		var it ChapterListItem
		var seenAtStr string
		if err := rows.Scan(&it.Number, &it.Title, &seenAtStr); err != nil {
			return nil, err
		}
		seenAt, err := parseSQLiteTime(seenAtStr)
		if err != nil {
			return nil, err
		}
		it.SeenAt = seenAt
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (db *DB) ListReadNumericChaptersInRange(mangaID int, start, end float64, limit, offset int) ([]ChapterListItem, error) {
	rows, err := db.Query(`
		SELECT chapter_number, COALESCE(chapters.title, ''), COALESCE(created_at, readable_at, published_at) AS seen_at
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) <= COALESCE(manga.last_read_number, -1)
			  AND CAST(chapters.chapter_number AS REAL) >= ?
			  AND CAST(chapters.chapter_number AS REAL) < ?
		ORDER BY CAST(chapters.chapter_number AS REAL) DESC
		LIMIT ? OFFSET ?
	`, mangaID, start, end, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []ChapterListItem
	for rows.Next() {
		var it ChapterListItem
		var seenAtStr string
		if err := rows.Scan(&it.Number, &it.Title, &seenAtStr); err != nil {
			return nil, err
		}
		seenAt, err := parseSQLiteTime(seenAtStr)
		if err != nil {
			return nil, err
		}
		it.SeenAt = seenAt
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (db *DB) GetMangaDetails(mangaID int) (MangaDetails, error) {
	var (
		lastCheckedStr string
		lastSeenAtStr  string
		lastReadNum    sql.NullFloat64
		minNum         sql.NullFloat64
		maxNum         sql.NullFloat64
	)

	var d MangaDetails
	err := db.QueryRow(`
		SELECT
			m.id,
			m.mangadex_id,
			m.title,
			COALESCE(CAST(m.last_checked AS TEXT), ''),
			COALESCE(CAST(m.last_seen_at AS TEXT), ''),
				m.last_read_number,
				m.unread_count,
				(SELECT COUNT(*) FROM chapters c WHERE c.manga_id = m.id),
				(SELECT COUNT(*) FROM chapters c WHERE c.manga_id = m.id AND c.chapter_number GLOB '[0-9]*' AND c.chapter_number NOT GLOB '*[^0-9.]*' AND c.chapter_number NOT GLOB '*.*.*'),
				(SELECT MIN(CAST(c.chapter_number AS REAL)) FROM chapters c WHERE c.manga_id = m.id AND c.chapter_number GLOB '[0-9]*' AND c.chapter_number NOT GLOB '*[^0-9.]*' AND c.chapter_number NOT GLOB '*.*.*'),
				(SELECT MAX(CAST(c.chapter_number AS REAL)) FROM chapters c WHERE c.manga_id = m.id AND c.chapter_number GLOB '[0-9]*' AND c.chapter_number NOT GLOB '*[^0-9.]*' AND c.chapter_number NOT GLOB '*.*.*')
			FROM manga m
			WHERE m.id = ?
		`, mangaID).Scan(
		&d.ID,
		&d.MangaDexID,
		&d.Title,
		&lastCheckedStr,
		&lastSeenAtStr,
		&lastReadNum,
		&d.UnreadCount,
		&d.ChaptersTotal,
		&d.NumericChaptersTotal,
		&minNum,
		&maxNum,
	)
	if err != nil {
		return MangaDetails{}, err
	}

	if strings.TrimSpace(lastCheckedStr) != "" {
		if t, err := parseSQLiteTime(lastCheckedStr); err == nil {
			d.LastChecked = t
			d.HasLastChecked = true
		}
	}
	if strings.TrimSpace(lastSeenAtStr) != "" {
		if t, err := parseSQLiteTime(lastSeenAtStr); err == nil {
			d.LastSeenAt = t
			d.HasLastSeenAt = true
		}
	}
	if lastReadNum.Valid {
		d.LastReadNumber = lastReadNum.Float64
		d.HasLastReadNumber = true
	}
	if minNum.Valid {
		d.MinNumber = minNum.Float64
		d.HasMinNumber = true
	}
	if maxNum.Valid {
		d.MaxNumber = maxNum.Float64
		d.HasMaxNumber = true
	}

	return d, nil
}

func (db *DB) CountUnreadChapters(mangaID int) (int, error) {
	var unreadCount int
	err := db.QueryRow(`
			SELECT COUNT(*)
			FROM chapters
			JOIN manga ON manga.id = chapters.manga_id
			WHERE chapters.manga_id = ?
			  AND chapters.chapter_number GLOB '[0-9]*'
			  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapters.chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapters.chapter_number AS REAL) > COALESCE(manga.last_read_number, -1)
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
				  AND chapters.chapter_number GLOB '[0-9]*'
				  AND chapters.chapter_number NOT GLOB '*[^0-9.]*'
				  AND chapters.chapter_number NOT GLOB '*.*.*'
				  AND CAST(chapters.chapter_number AS REAL) > COALESCE(manga.last_read_number, -1)
			)
			WHERE id = ?
		`, mangaID)
	return err
}

func (db *DB) GetAllManga() (*sql.Rows, error) {
	return db.Query("SELECT id, mangadex_id, title, last_checked, last_seen_at, last_read_number, unread_count FROM manga")
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
		var lastReadNumber sql.NullFloat64
		if err := rows.Scan(&row.ID, &row.MangaDexID, &row.Title, &row.LastChecked, &lastSeenAt, &lastReadNumber, &row.UnreadCount); err != nil {
			return nil, err
		}
		if lastSeenAt.Valid {
			row.LastSeenAt = lastSeenAt.Time
		}
		if lastReadNumber.Valid {
			row.LastReadNumber = lastReadNumber.Float64
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

	num, err := strconv.ParseFloat(strings.TrimSpace(chapterNumber), 64)
	if err != nil {
		// Non-numeric chapters (extras) are not part of numeric progress tracking.
		return nil
	}

	if _, err := db.Exec(`
		UPDATE manga
		SET last_read_number = CASE
			WHEN last_read_number IS NULL THEN ?
			WHEN last_read_number < ? THEN ?
			ELSE last_read_number
		END
		WHERE id = ?
	`, num, num, num, mangaID); err != nil {
		return err
	}
	return db.recalculateUnreadCountLocked(mangaID)
}

func (db *DB) MarkChapterAsUnread(mangaID int, chapterNumber string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	num, err := strconv.ParseFloat(strings.TrimSpace(chapterNumber), 64)
	if err != nil {
		// Non-numeric chapters (extras) are not part of numeric progress tracking.
		return nil
	}

	var prev sql.NullFloat64
	err = db.QueryRow(`
			SELECT MAX(CAST(chapter_number AS REAL))
			FROM chapters
			WHERE manga_id = ?
			  AND chapter_number GLOB '[0-9]*'
			  AND chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapter_number AS REAL) < ?
		`, mangaID, num).Scan(&prev)
		if err != nil {
			return err
		}

	if prev.Valid {
		if _, err := db.Exec("UPDATE manga SET last_read_number = ? WHERE id = ?", prev.Float64, mangaID); err != nil {
			return err
		}
	} else {
		if _, err := db.Exec("UPDATE manga SET last_read_number = NULL WHERE id = ?", mangaID); err != nil {
			return err
		}
	}

	return db.recalculateUnreadCountLocked(mangaID)
}

func (db *DB) GetLastReadChapter(mangaID int) (chapterNumber string, title string, ok bool, err error) {
	var num, t string
	err = db.QueryRow(`
			SELECT chapter_number, COALESCE(title, '')
			FROM chapters
			WHERE manga_id = ?
			  AND chapter_number GLOB '[0-9]*'
			  AND chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapter_number AS REAL) <= COALESCE((SELECT last_read_number FROM manga WHERE id = ?), -1)
			ORDER BY CAST(chapter_number AS REAL) DESC
			LIMIT 1
		`, mangaID, mangaID).Scan(&num, &t)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return num, t, true, nil
}

func (db *DB) ListUnreadChapters(mangaID int, limit, offset int) ([]ChapterListItem, error) {
	rows, err := db.Query(`
			SELECT chapter_number, COALESCE(title, ''), COALESCE(created_at, readable_at, published_at) AS seen_at
			FROM chapters
			WHERE manga_id = ?
			  AND chapter_number GLOB '[0-9]*'
			  AND chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapter_number AS REAL) > COALESCE((SELECT last_read_number FROM manga WHERE id = ?), -1)
			ORDER BY CAST(chapter_number AS REAL) ASC
			LIMIT ? OFFSET ?
		`, mangaID, mangaID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]ChapterListItem, 0, limit)
	for rows.Next() {
		var it ChapterListItem
		var seenAtStr string
		if err := rows.Scan(&it.Number, &it.Title, &seenAtStr); err != nil {
			return nil, err
		}
		seenAt, err := parseSQLiteTime(seenAtStr)
		if err != nil {
			return nil, err
		}
		it.SeenAt = seenAt
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (db *DB) GetUnreadChapters(mangaID int) (*sql.Rows, error) {
	return db.Query(`
			SELECT chapter_number, title 
			FROM chapters 
			WHERE manga_id = ?
			  AND chapter_number GLOB '[0-9]*'
			  AND chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapter_number AS REAL) > COALESCE((SELECT last_read_number FROM manga WHERE id = ?), -1)
			ORDER BY 
				CAST(chapter_number AS REAL) ASC
			LIMIT 3
	`, mangaID, mangaID)
}

func (db *DB) GetReadChapters(mangaID int) (*sql.Rows, error) {
	return db.Query(`
			SELECT chapter_number, title 
			FROM chapters 
			WHERE manga_id = ?
			  AND chapter_number GLOB '[0-9]*'
			  AND chapter_number NOT GLOB '*[^0-9.]*'
			  AND chapter_number NOT GLOB '*.*.*'
			  AND CAST(chapter_number AS REAL) <= COALESCE((SELECT last_read_number FROM manga WHERE id = ?), -1)
			ORDER BY 
				CAST(chapter_number AS REAL) DESC
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

func parseSQLiteTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	// Formats observed in practice with SQLite/go-sqlite3.
	layoutsWithZone := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
	}
	for _, layout := range layoutsWithZone {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	layoutsNoZone := []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layoutsNoZone {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported timestamp format: %q", s)
}
