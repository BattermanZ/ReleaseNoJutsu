package db

import (
	"database/sql"
	"time"

	"releasenojutsu/internal/logger"
)

func (db *DB) UpdateCronLastRun() {
	dbMutex.Lock()
	_, err := db.Exec("INSERT OR REPLACE INTO system_status (key, last_update) VALUES ('cron_last_run', ?)",
		time.Now().UTC())
	dbMutex.Unlock()
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to update cron last run time: %v", err)
	}
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
