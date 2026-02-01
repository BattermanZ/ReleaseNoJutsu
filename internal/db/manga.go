package db

import (
	"database/sql"
	"strings"
	"time"
)

func (db *DB) AddManga(mangaID, title string, userID int64) (int64, error) {
	result, err := db.Exec("INSERT INTO manga (user_id, mangadex_id, title, is_manga_plus, last_checked) VALUES (?, ?, ?, ?, ?)",
		userID, mangaID, title, 0, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) AddMangaWithMangaPlus(mangaID, title string, isMangaPlus bool, userID int64) (int64, error) {
	val := 0
	if isMangaPlus {
		val = 1
	}
	result, err := db.Exec("INSERT INTO manga (user_id, mangadex_id, title, is_manga_plus, last_checked) VALUES (?, ?, ?, ?, ?)",
		userID, mangaID, title, val, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) IsMangaPlus(mangaID int) (bool, error) {
	var v int
	if err := db.QueryRow("SELECT is_manga_plus FROM manga WHERE id = ?", mangaID).Scan(&v); err != nil {
		return false, err
	}
	return v != 0, nil
}

func (db *DB) SetMangaPlus(mangaID int, isMangaPlus bool) error {
	val := 0
	if isMangaPlus {
		val = 1
	}
	_, err := db.Exec("UPDATE manga SET is_manga_plus = ? WHERE id = ?", val, mangaID)
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
	_, err := db.Exec("UPDATE manga SET last_checked = ? WHERE id = ?",
		time.Now().UTC(), mangaID)
	return err
}

func (db *DB) UpdateMangaLastSeenAt(mangaID int, seenAt time.Time) error {
	_, err := db.Exec("UPDATE manga SET last_seen_at = ? WHERE id = ?",
		seenAt.UTC(), mangaID)
	return err
}

func (db *DB) GetAllManga() (*sql.Rows, error) {
	// Use GetAllMangaByUser in normal flows to avoid accidental cross-user leakage.
	return db.Query("SELECT id, user_id, mangadex_id, title, is_manga_plus, last_checked, last_seen_at, last_read_number, unread_count FROM manga")
}

func (db *DB) GetAllMangaByUser(userID int64) (*sql.Rows, error) {
	return db.Query("SELECT id, user_id, mangadex_id, title, is_manga_plus, last_checked, last_seen_at, last_read_number, unread_count FROM manga WHERE user_id = ? ORDER BY id", userID)
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
		var isMangaPlus int
		var lastSeenAt sql.NullTime
		var lastReadNumber sql.NullFloat64
		if err := rows.Scan(&row.ID, &row.UserID, &row.MangaDexID, &row.Title, &isMangaPlus, &row.LastChecked, &lastSeenAt, &lastReadNumber, &row.UnreadCount); err != nil {
			return nil, err
		}
		row.IsMangaPlus = isMangaPlus != 0
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

func (db *DB) GetMangaDetails(mangaID int, userID int64) (MangaDetails, error) {
	var (
		lastCheckedStr string
		lastSeenAtStr  string
		isMangaPlus    int
		lastReadNum    sql.NullFloat64
		minNum         sql.NullFloat64
		maxNum         sql.NullFloat64
	)

	var d MangaDetails
	err := db.QueryRow(`
		SELECT
			m.id,
			m.user_id,
			m.mangadex_id,
			m.title,
			m.is_manga_plus,
			COALESCE(CAST(m.last_checked AS TEXT), ''),
			COALESCE(CAST(m.last_seen_at AS TEXT), ''),
			m.last_read_number,
			m.unread_count,
			(SELECT COUNT(*) FROM chapters c WHERE c.manga_id = m.id),
			(SELECT COUNT(*) FROM chapters c WHERE c.manga_id = m.id AND c.chapter_number GLOB '[0-9]*' AND c.chapter_number NOT GLOB '*[^0-9.]*' AND c.chapter_number NOT GLOB '*.*.*'),
			(SELECT MIN(CAST(c.chapter_number AS REAL)) FROM chapters c WHERE c.manga_id = m.id AND c.chapter_number GLOB '[0-9]*' AND c.chapter_number NOT GLOB '*[^0-9.]*' AND c.chapter_number NOT GLOB '*.*.*'),
			(SELECT MAX(CAST(c.chapter_number AS REAL)) FROM chapters c WHERE c.manga_id = m.id AND c.chapter_number GLOB '[0-9]*' AND c.chapter_number NOT GLOB '*[^0-9.]*' AND c.chapter_number NOT GLOB '*.*.*')
		FROM manga m
		WHERE m.id = ? AND m.user_id = ?
	`, mangaID, userID).Scan(
		&d.ID,
		&d.UserID,
		&d.MangaDexID,
		&d.Title,
		&isMangaPlus,
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
	d.IsMangaPlus = isMangaPlus != 0

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

func (db *DB) GetMangaTitle(mangaID int, userID int64) (string, error) {
	var title string
	err := db.QueryRow("SELECT title FROM manga WHERE id = ? AND user_id = ?", mangaID, userID).Scan(&title)
	return title, err
}

func (db *DB) DeleteManga(mangaID int, userID int64) error {
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
	_, err = tx.Exec("DELETE FROM chapters WHERE manga_id = ? AND manga_id IN (SELECT id FROM manga WHERE id = ? AND user_id = ?)", mangaID, mangaID, userID)
	if err != nil {
		return err
	}

	// Delete the manga
	_, err = tx.Exec("DELETE FROM manga WHERE id = ? AND user_id = ?", mangaID, userID)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) MangaBelongsToUser(mangaID int, userID int64) (bool, error) {
	var id int
	err := db.QueryRow("SELECT id FROM manga WHERE id = ? AND user_id = ?", mangaID, userID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
