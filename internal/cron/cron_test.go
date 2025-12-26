package cron

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
)

func TestPerformUpdate_DoesNotDeadlockOnSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}

	if _, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super"); err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mangadex.ChapterFeedResponse{Data: []struct {
			Attributes struct {
				Chapter     string    `json:"chapter"`
				Title       string    `json:"title"`
				PublishedAt time.Time `json:"publishAt"`
			} `json:"attributes"`
		}{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	client := mangadex.NewClient()
	client.BaseURL = srv.URL

	s := NewScheduler(database, nil, client)

	done := make(chan struct{}, 1)
	go func() {
		s.performUpdate()
		done <- struct{}{}
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("performUpdate() appears blocked (SQLite deadlock/rows leak)")
	}
}

func TestCheckNewChaptersForManga_AddsNewChaptersAndUpdatesLastChecked(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}

	mangaDexID := "37b87be0-b1f4-4507-affa-06c99ebb27f8"
	mangaTitle := "Dragon Ball Super"
	mangaDBID, err := database.AddManga(mangaDexID, mangaTitle)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	lastChecked := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := database.Exec("UPDATE manga SET last_checked = ? WHERE id = ?", lastChecked, mangaDBID); err != nil {
		t.Fatalf("seed last_checked: %v", err)
	}

	ch1Time := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	ch2Time := time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mangadex.ChapterFeedResponse{
			Data: []struct {
				Attributes struct {
					Chapter     string    `json:"chapter"`
					Title       string    `json:"title"`
					PublishedAt time.Time `json:"publishAt"`
				} `json:"attributes"`
			}{
				{Attributes: struct {
					Chapter     string    `json:"chapter"`
					Title       string    `json:"title"`
					PublishedAt time.Time `json:"publishAt"`
				}{Chapter: "2", Title: "Two", PublishedAt: ch2Time}},
				{Attributes: struct {
					Chapter     string    `json:"chapter"`
					Title       string    `json:"title"`
					PublishedAt time.Time `json:"publishAt"`
				}{Chapter: "1", Title: "One", PublishedAt: ch1Time}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	client := mangadex.NewClient()
	client.BaseURL = srv.URL

	s := NewScheduler(database, nil, client)
	newChapters := s.checkNewChaptersForManga(int(mangaDBID), mangaDexID, mangaTitle, lastChecked)

	if len(newChapters) != 2 {
		t.Fatalf("newChapters len=%d, want 2", len(newChapters))
	}

	var chaptersCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM chapters WHERE manga_id = ?", mangaDBID).Scan(&chaptersCount); err != nil {
		t.Fatalf("count chapters: %v", err)
	}
	if chaptersCount != 2 {
		t.Fatalf("stored chapters=%d, want 2", chaptersCount)
	}

	var unread int
	if err := database.QueryRow("SELECT unread_count FROM manga WHERE id = ?", mangaDBID).Scan(&unread); err != nil {
		t.Fatalf("read unread_count: %v", err)
	}
	if unread != 2 {
		t.Fatalf("unread_count=%d, want 2", unread)
	}

	var updated time.Time
	if err := database.QueryRow("SELECT last_checked FROM manga WHERE id = ?", mangaDBID).Scan(&updated); err != nil {
		t.Fatalf("read last_checked: %v", err)
	}
	if !updated.After(lastChecked) {
		t.Fatalf("last_checked=%v, want after %v", updated, lastChecked)
	}
}
