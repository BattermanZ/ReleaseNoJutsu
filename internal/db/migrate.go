package db

import (
	"database/sql"
	"strings"
)

func (db *DB) Migrate(adminUserID int64) error {
	// Disable FK checks for the duration of migration to avoid legacy data conflicts.
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	defer func() {
		_, _ = db.Exec("PRAGMA foreign_keys = ON")
	}()

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			chat_id INTEGER PRIMARY KEY,
			is_admin INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP
		)
	`); err != nil {
		return err
	}

	hasUsersIsAdmin, err := db.hasColumn("users", "is_admin")
	if err != nil {
		return err
	}
	if !hasUsersIsAdmin {
		if _, err := db.Exec("ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}

	hasUsersCreatedAt, err := db.hasColumn("users", "created_at")
	if err != nil {
		return err
	}
	if !hasUsersCreatedAt {
		if _, err := db.Exec("ALTER TABLE users ADD COLUMN created_at TIMESTAMP"); err != nil {
			return err
		}
	}

	if adminUserID > 0 {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO users (chat_id, is_admin, created_at)
			VALUES (?, 1, CURRENT_TIMESTAMP)
		`, adminUserID); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE users SET is_admin = 1 WHERE chat_id = ?", adminUserID); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE users SET created_at = COALESCE(created_at, CURRENT_TIMESTAMP)"); err != nil {
			return err
		}
	}

	hasMangaUserID, err := db.hasColumn("manga", "user_id")
	if err != nil {
		return err
	}

	needsRebuild, err := db.mangaTableNeedsRebuild()
	if err != nil {
		return err
	}
	if needsRebuild {
		if err := db.rebuildMangaTable(adminUserID, hasMangaUserID); err != nil {
			return err
		}
	}

	hasMangaLastSeenAt, err := db.hasColumn("manga", "last_seen_at")
	if err != nil {
		return err
	}
	if !hasMangaLastSeenAt {
		if _, err := db.Exec("ALTER TABLE manga ADD COLUMN last_seen_at TIMESTAMP"); err != nil {
			return err
		}
	}

	hasMangaIsMangaPlus, err := db.hasColumn("manga", "is_manga_plus")
	if err != nil {
		return err
	}
	if !hasMangaIsMangaPlus {
		if _, err := db.Exec("ALTER TABLE manga ADD COLUMN is_manga_plus INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}

	hasMangaUserID, err = db.hasColumn("manga", "user_id")
	if err != nil {
		return err
	}
	if !hasMangaUserID {
		if _, err := db.Exec("ALTER TABLE manga ADD COLUMN user_id INTEGER"); err != nil {
			return err
		}
	}
	if adminUserID > 0 {
		if _, err := db.Exec("UPDATE manga SET user_id = ? WHERE user_id IS NULL", adminUserID); err != nil {
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

	hasPairingCodes, err := db.hasTable("pairing_codes")
	if err != nil {
		return err
	}
	if !hasPairingCodes {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS pairing_codes (
				code TEXT PRIMARY KEY,
				expires_at TIMESTAMP NOT NULL,
				used_by_chat_id INTEGER,
				used_at TIMESTAMP,
				created_by_admin INTEGER NOT NULL,
				created_at TIMESTAMP NOT NULL
			)
		`); err != nil {
			return err
		}
	}

	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_manga_user_mangadex ON manga(user_id, mangadex_id)"); err != nil {
		return err
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

func (db *DB) mangaTableNeedsRebuild() (bool, error) {
	row := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='manga'")
	var sqlText string
	if err := row.Scan(&sqlText); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(strings.ToLower(sqlText), "mangadex_id text not null unique"), nil
}

func (db *DB) rebuildMangaTable(adminUserID int64, hasUserID bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	if _, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS manga_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			mangadex_id TEXT NOT NULL,
			title TEXT NOT NULL,
			is_manga_plus INTEGER NOT NULL DEFAULT 0,
			last_checked TIMESTAMP,
			last_seen_at TIMESTAMP,
			last_read_number REAL,
			unread_count INTEGER DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users (chat_id)
		)
	`); err != nil {
		return err
	}

	if hasUserID {
		if _, err = tx.Exec(`
			INSERT INTO manga_new (id, user_id, mangadex_id, title, is_manga_plus, last_checked, last_seen_at, last_read_number, unread_count)
			SELECT id, COALESCE(user_id, ?), mangadex_id, title, COALESCE(is_manga_plus, 0), last_checked, last_seen_at, last_read_number, unread_count
			FROM manga
		`, adminUserID); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(`
			INSERT INTO manga_new (id, user_id, mangadex_id, title, is_manga_plus, last_checked, last_seen_at, last_read_number, unread_count)
			SELECT id, ?, mangadex_id, title, COALESCE(is_manga_plus, 0), last_checked, last_seen_at, last_read_number, unread_count
			FROM manga
		`, adminUserID); err != nil {
			return err
		}
	}

	if _, err = tx.Exec("DROP TABLE manga"); err != nil {
		return err
	}
	if _, err = tx.Exec("ALTER TABLE manga_new RENAME TO manga"); err != nil {
		return err
	}
	return nil
}

func (db *DB) hasTable(tableName string) (bool, error) {
	row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name = ?", tableName)
	var name string
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return name == tableName, nil
}
