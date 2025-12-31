package db

import (
	"database/sql"
	"strings"
	"time"
)

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

func (db *DB) GetMangaTitle(mangaID int) (string, error) {
	var title string
	err := db.QueryRow("SELECT title FROM manga WHERE id = ?", mangaID).Scan(&title)
	return title, err
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
