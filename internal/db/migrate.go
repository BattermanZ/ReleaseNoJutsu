package db

func (db *DB) Migrate() error {
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
