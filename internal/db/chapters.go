package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (db *DB) AddChapter(mangaID int64, chapterNumber, title string, publishedAt, readableAt, createdAt, updatedAt time.Time) error {
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
	return db.recalculateUnreadCount(mangaID)
}

func (db *DB) recalculateUnreadCount(mangaID int) error {
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

func (db *DB) MarkChapterAsRead(mangaID int, chapterNumber string) error {
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
	return db.recalculateUnreadCount(mangaID)
}

func (db *DB) MarkChapterAsUnread(mangaID int, chapterNumber string) error {
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

	return db.recalculateUnreadCount(mangaID)
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
		ORDER BY CAST(chapter_number AS REAL) ASC
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
		ORDER BY CAST(chapter_number AS REAL) DESC
		LIMIT 3
	`, mangaID, mangaID)
}
