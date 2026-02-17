package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrate_RebuildsLegacyMangaTableWithoutUserID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.Exec(`
		CREATE TABLE manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mangadex_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			is_manga_plus INTEGER DEFAULT 0,
			last_checked TIMESTAMP,
			last_seen_at TIMESTAMP,
			last_read_at TIMESTAMP,
			last_read_number REAL,
			unread_count INTEGER DEFAULT 0
		);
		CREATE TABLE chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if _, err := database.Exec(`
		INSERT INTO manga (id, mangadex_id, title, last_checked, unread_count)
		VALUES (1, '37b87be0-b1f4-4507-affa-06c99ebb27f8', 'Dragon Ball Super', ?, 0)
	`, time.Now().UTC()); err != nil {
		t.Fatalf("seed manga row: %v", err)
	}

	if err := database.Migrate(123); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	var userID int64
	if err := database.QueryRow("SELECT user_id FROM manga WHERE id = 1").Scan(&userID); err != nil {
		t.Fatalf("select user_id: %v", err)
	}
	if userID != 123 {
		t.Fatalf("user_id=%d, want 123", userID)
	}

	var hasIndex int
	if err := database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_manga_user_mangadex'").Scan(&hasIndex); err != nil {
		t.Fatalf("index query: %v", err)
	}
	if hasIndex != 1 {
		t.Fatalf("idx_manga_user_mangadex count=%d, want 1", hasIndex)
	}
}

func TestMigrate_RebuildsLegacyMangaTableWithNullableUserID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.Exec(`
		CREATE TABLE manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			mangadex_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			is_manga_plus INTEGER DEFAULT 0,
			last_checked TIMESTAMP,
			last_seen_at TIMESTAMP,
			last_read_at TIMESTAMP,
			last_read_number REAL,
			unread_count INTEGER DEFAULT 0
		);
		CREATE TABLE chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if _, err := database.Exec(`
		INSERT INTO manga (id, user_id, mangadex_id, title, last_checked, unread_count)
		VALUES (1, NULL, '37b87be0-b1f4-4507-affa-06c99ebb27f8', 'Dragon Ball Super', ?, 0)
	`, time.Now().UTC()); err != nil {
		t.Fatalf("seed manga row: %v", err)
	}

	if err := database.Migrate(456); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	var userID int64
	if err := database.QueryRow("SELECT user_id FROM manga WHERE id = 1").Scan(&userID); err != nil {
		t.Fatalf("select user_id: %v", err)
	}
	if userID != 456 {
		t.Fatalf("user_id=%d, want 456", userID)
	}
}

func TestMigrate_BackfillsLegacyLastReadAtAndNumberFromReadFlags(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.Exec(`
		CREATE TABLE manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			mangadex_id TEXT NOT NULL,
			title TEXT NOT NULL,
			last_checked TIMESTAMP,
			last_seen_at TIMESTAMP,
			last_read_at TIMESTAMP,
			unread_count INTEGER DEFAULT 0
		);
		CREATE TABLE chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP,
			is_read BOOLEAN DEFAULT FALSE
		);
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if _, err := database.Exec(`
		INSERT INTO manga (id, user_id, mangadex_id, title, last_checked, last_read_at, unread_count)
		VALUES (1, NULL, '37b87be0-b1f4-4507-affa-06c99ebb27f8', 'Dragon Ball Super', ?, NULL, 0)
	`, time.Now().UTC()); err != nil {
		t.Fatalf("seed manga row: %v", err)
	}

	t1 := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 1, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 3, 1, 0, 0, 0, time.UTC)
	if _, err := database.Exec(`
		INSERT INTO chapters (manga_id, chapter_number, title, published_at, is_read)
		VALUES (1, '1', 'One', ?, 1),
			   (1, '2', 'Two', ?, 1),
			   (1, '3', 'Three', ?, 0)
	`, t1, t2, t3); err != nil {
		t.Fatalf("seed chapter rows: %v", err)
	}

	if err := database.Migrate(789); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	var lastReadAtStr sql.NullString
	if err := database.QueryRow("SELECT CAST(last_read_at AS TEXT) FROM manga WHERE id = 1").Scan(&lastReadAtStr); err != nil {
		t.Fatalf("select last_read_at: %v", err)
	}
	if !lastReadAtStr.Valid || lastReadAtStr.String == "" {
		t.Fatal("expected last_read_at to be backfilled")
	}

	lastReadAt, err := parseSQLiteTime(lastReadAtStr.String)
	if err != nil {
		t.Fatalf("parse last_read_at: %v", err)
	}
	if !lastReadAt.Equal(t2) {
		t.Fatalf("last_read_at=%v, want %v", lastReadAt, t2)
	}

	var lastReadNumber sql.NullFloat64
	if err := database.QueryRow("SELECT last_read_number FROM manga WHERE id = 1").Scan(&lastReadNumber); err != nil {
		t.Fatalf("select last_read_number: %v", err)
	}
	if !lastReadNumber.Valid || lastReadNumber.Float64 != 2 {
		t.Fatalf("last_read_number=%v, want 2", lastReadNumber)
	}
}

func TestMigrate_BackfillsLastReadNumberFromLegacyReadFlagsWithoutLastReadAt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.Exec(`
		CREATE TABLE manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			mangadex_id TEXT NOT NULL,
			title TEXT NOT NULL,
			last_checked TIMESTAMP,
			last_seen_at TIMESTAMP,
			unread_count INTEGER DEFAULT 0
		);
		CREATE TABLE chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP,
			is_read BOOLEAN DEFAULT FALSE
		);
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if _, err := database.Exec(`
		INSERT INTO manga (id, user_id, mangadex_id, title, last_checked, unread_count)
		VALUES (1, NULL, '37b87be0-b1f4-4507-affa-06c99ebb27f8', 'Dragon Ball Super', ?, 0)
	`, time.Now().UTC()); err != nil {
		t.Fatalf("seed manga row: %v", err)
	}

	t1 := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 1, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 3, 1, 0, 0, 0, time.UTC)
	if _, err := database.Exec(`
		INSERT INTO chapters (manga_id, chapter_number, title, published_at, is_read)
		VALUES (1, '1', 'One', ?, 1),
			   (1, '5', 'Five', ?, 1),
			   (1, '7', 'Seven', ?, 0)
	`, t1, t2, t3); err != nil {
		t.Fatalf("seed chapter rows: %v", err)
	}

	if err := database.Migrate(111); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	var lastReadNumber sql.NullFloat64
	if err := database.QueryRow("SELECT last_read_number FROM manga WHERE id = 1").Scan(&lastReadNumber); err != nil {
		t.Fatalf("select last_read_number: %v", err)
	}
	if !lastReadNumber.Valid || lastReadNumber.Float64 != 5 {
		t.Fatalf("last_read_number=%v, want 5", lastReadNumber)
	}
}
