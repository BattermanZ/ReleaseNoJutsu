package db

func (db *DB) migrateData(flags migrationFlags) error {
	if err := db.deduplicateLegacyChapters(flags.hasChaptersIsRead); err != nil {
		return err
	}
	if err := db.ensureChapterUniqueIndex(); err != nil {
		return err
	}
	if err := db.normalizeFuturePublishedAt(); err != nil {
		return err
	}
	if err := db.backfillMissingLastSeenAt(); err != nil {
		return err
	}
	if err := db.repairFutureLastSeenAt(); err != nil {
		return err
	}
	if flags.hasMangaLastReadAt && flags.hasChaptersIsRead {
		if err := db.backfillLastReadAtFromLegacyReadFlags(); err != nil {
			return err
		}
	}
	if flags.hasMangaLastReadAt {
		if err := db.backfillLastReadNumberFromLastReadAt(); err != nil {
			return err
		}
	}
	if flags.hasChaptersIsRead {
		if err := db.backfillLastReadNumberFromLegacyReadFlags(); err != nil {
			return err
		}
	}
	if err := db.recalculateMangaUnreadCount(); err != nil {
		return err
	}
	return nil
}

func (db *DB) deduplicateLegacyChapters(hasChaptersIsRead bool) error {
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
	return nil
}

func (db *DB) ensureChapterUniqueIndex() error {
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_chapters_manga_chapter ON chapters(manga_id, chapter_number)"); err != nil {
		return err
	}
	return nil
}

func (db *DB) normalizeFuturePublishedAt() error {
	// MangaDex may return publishAt sentinel values far in the future (e.g. 2037-12-31).
	// Normalize those legacy rows to reliable chapter timestamps when available.
	if _, err := db.Exec(`
		UPDATE chapters
		SET published_at = COALESCE(created_at, readable_at, published_at)
		WHERE published_at IS NOT NULL
		  AND datetime(published_at) > datetime('now', '+1 day')
		  AND (created_at IS NOT NULL OR readable_at IS NOT NULL)
	`); err != nil {
		return err
	}
	return nil
}

func (db *DB) backfillMissingLastSeenAt() error {
	if _, err := db.Exec(`
		UPDATE manga
		SET last_seen_at = COALESCE(
			(
				SELECT MAX(COALESCE(created_at, readable_at, CASE WHEN datetime(published_at) <= datetime('now', '+1 day') THEN published_at END))
				FROM chapters
				WHERE chapters.manga_id = manga.id
			),
			last_checked
		)
		WHERE last_seen_at IS NULL
	`); err != nil {
		return err
	}
	return nil
}

func (db *DB) repairFutureLastSeenAt() error {
	// Repair any watermark poisoned by future timestamps.
	if _, err := db.Exec(`
		UPDATE manga
		SET last_seen_at = COALESCE(
			(
				SELECT MAX(COALESCE(created_at, readable_at, CASE WHEN datetime(published_at) <= datetime('now', '+1 day') THEN published_at END))
				FROM chapters
				WHERE chapters.manga_id = manga.id
			),
			last_checked
		)
		WHERE last_seen_at IS NOT NULL
		  AND datetime(last_seen_at) > datetime('now', '+1 day')
	`); err != nil {
		return err
	}
	return nil
}

func (db *DB) backfillLastReadAtFromLegacyReadFlags() error {
	// Backfill last_read_at from legacy per-chapter flags if present.
	if _, err := db.Exec(`
		UPDATE manga
		SET last_read_at = COALESCE(
			(
				SELECT MAX(COALESCE(created_at, readable_at, CASE WHEN datetime(published_at) <= datetime('now', '+1 day') THEN published_at END))
				FROM chapters
				WHERE chapters.manga_id = manga.id AND is_read = true
			),
			last_read_at
		)
		WHERE last_read_at IS NULL
	`); err != nil {
		return err
	}
	return nil
}

func (db *DB) backfillLastReadNumberFromLastReadAt() error {
	// Backfill last_read_number from last_read_at (numeric chapters only).
	if _, err := db.Exec(`
		UPDATE manga
		SET last_read_number = (
			SELECT MAX(CAST(chapter_number AS REAL))
			FROM chapters
				WHERE chapters.manga_id = manga.id
				  AND chapter_number GLOB '[0-9]*'
				  AND chapter_number NOT GLOB '*[^0-9.]*'
				  AND chapter_number NOT GLOB '*.*.*'
				  AND COALESCE(created_at, readable_at, CASE WHEN datetime(published_at) <= datetime('now', '+1 day') THEN published_at END) <= COALESCE(manga.last_read_at, '1970-01-01T00:00:00Z')
			)
			WHERE last_read_number IS NULL AND last_read_at IS NOT NULL
		`); err != nil {
		return err
	}
	return nil
}

func (db *DB) backfillLastReadNumberFromLegacyReadFlags() error {
	// Backfill last_read_number from legacy per-chapter flags (numeric chapters only).
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
	return nil
}

func (db *DB) recalculateMangaUnreadCount() error {
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
