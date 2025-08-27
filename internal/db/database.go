package db

import (
	"database/sql"
	
	"sync"
	"time"

	"releasenojutsu/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

var dbMutex sync.Mutex

// DB wraps the sql.DB connection.

type DB struct {
	*sql.DB
}

// New opens a connection to the database.

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL")
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
			unread_count INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP,
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

func (db *DB) AddManga(mangaID, title string) (int64, error) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	result, err := db.Exec("INSERT INTO manga (mangadex_id, title, last_checked) VALUES (?, ?, ?)",
		mangaID, title, time.Now())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) AddChapter(mangaID int64, chapterNumber, title string, publishedAt time.Time) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec(`
		INSERT OR REPLACE INTO chapters (manga_id, chapter_number, title, published_at) 
		VALUES (?, ?, ?, ?)
	`, mangaID, chapterNumber, title, publishedAt)
	return err
}

func (db *DB) GetManga(mangaID int) (string, string, time.Time, error) {
	var mangadexID, title string
	var lastChecked time.Time
	err := db.QueryRow("SELECT mangadex_id, title, last_checked FROM manga WHERE id = ?", mangaID).
		Scan(&mangadexID, &title, &lastChecked)
	return mangadexID, title, lastChecked, err
}

func (db *DB) UpdateMangaLastChecked(mangaID int) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec("UPDATE manga SET last_checked = ? WHERE id = ?",
		time.Now().UTC(), mangaID)
	return err
}

func (db *DB) UpdateMangaUnreadCount(mangaID int, count int) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec("UPDATE manga SET unread_count = unread_count + ? WHERE id = ?",
		count, mangaID)
	return err
}

func (db *DB) GetUnreadCount(mangaID int) (int, error) {
	var unreadCount int
	err := db.QueryRow("SELECT unread_count FROM manga WHERE id = ?", mangaID).Scan(&unreadCount)
	return unreadCount, err
}

func (db *DB) GetAllManga() (*sql.Rows, error) {
	return db.Query("SELECT id, mangadex_id, title, last_checked FROM manga")
}

func (db *DB) GetAllUsers() (*sql.Rows, error) {
	return db.Query("SELECT chat_id FROM users")
}

func (db *DB) MarkChapterAsRead(mangaID int, chapterNumber string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec(`
		UPDATE chapters 
		SET is_read = true 
		WHERE manga_id = ? AND 
		CAST(
			CASE 
				WHEN chapter_number GLOB '[0-9]*.[0-9]*' THEN chapter_number
				WHEN chapter_number GLOB '[0-9]*' THEN chapter_number || '.0'
				ELSE '999999.0'
			END 
		AS DECIMAL) <= CAST(
			CASE 
				WHEN ? GLOB '[0-9]*.[0-9]*' THEN ?
				WHEN ? GLOB '[0-9]*' THEN ? || '.0'
				ELSE '999999.0'
			END 
		AS DECIMAL)
	`, mangaID, chapterNumber, chapterNumber, chapterNumber, chapterNumber)

	return err
}

func (db *DB) MarkChapterAsUnread(mangaID int, chapterNumber string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec(`
		UPDATE chapters 
		SET is_read = false 
		WHERE manga_id = ? AND chapter_number = ?
	`, mangaID, chapterNumber)

	return err
}

func (db *DB) GetUnreadChapters(mangaID int) (*sql.Rows, error) {
	return db.Query(`
		SELECT chapter_number, title 
		FROM chapters 
		WHERE manga_id = ? AND is_read = false
		ORDER BY 
			CAST(
				CASE 
					WHEN chapter_number GLOB '[0-9]*.[0-9]*' THEN chapter_number
					WHEN chapter_number GLOB '[0-9]*' THEN chapter_number || '.0'
					ELSE '999999.0'
				END 
			AS DECIMAL) DESC
		LIMIT 3
	`, mangaID)
}

func (db *DB) GetReadChapters(mangaID int) (*sql.Rows, error) {
	return db.Query(`
		SELECT chapter_number, title 
		FROM chapters 
		WHERE manga_id = ? AND is_read = true
		ORDER BY 
			CAST(
				CASE 
					WHEN chapter_number GLOB '[0-9]*.[0-9]*' THEN chapter_number
					WHEN chapter_number GLOB '[0-9]*' THEN chapter_number || '.0'
					ELSE '999999.0'
				END 
			AS DECIMAL) DESC
		LIMIT 3
	`, mangaID)
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
