package db

import (
	"fmt"
	"strings"
	"time"
)

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
