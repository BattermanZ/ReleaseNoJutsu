package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
	"releasenojutsu/internal/updater"
)

type recordingNotifier struct {
	sent map[int64][]string
}

func (n *recordingNotifier) SendHTML(chatID int64, text string) error {
	if n.sent == nil {
		n.sent = map[int64][]string{}
	}
	n.sent[chatID] = append(n.sent[chatID], text)
	return nil
}

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
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	if _, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super"); err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mangadex.ChapterFeedResponse{Data: []mangadex.Chapter{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	client := mangadex.NewClient()
	client.BaseURL = srv.URL

	upd := updater.New(database, client, client)
	s := NewScheduler(database, &recordingNotifier{}, upd, nil)

	done := make(chan struct{}, 1)
	go func() {
		s.performUpdate(context.Background())
		done <- struct{}{}
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("performUpdate() appears blocked (SQLite deadlock/rows leak)")
	}
}

func TestPerformUpdate_SendsNotificationsToRegisteredChats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	chatID := int64(42)
	if err := database.EnsureUser(chatID); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	mangaDexID := "37b87be0-b1f4-4507-affa-06c99ebb27f8"
	mangaTitle := "Dragon Ball Super"
	mangaDBID, err := database.AddManga(mangaDexID, mangaTitle)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	lastSeenAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := database.UpdateMangaLastSeenAt(int(mangaDBID), lastSeenAt); err != nil {
		t.Fatalf("UpdateMangaLastSeenAt(): %v", err)
	}

	chTime := time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mangadex.ChapterFeedResponse{
			Data: []mangadex.Chapter{
				{Attributes: mangadex.ChapterAttributes{Chapter: "1", Title: "One", PublishedAt: chTime, ReadableAt: chTime, CreatedAt: chTime, UpdatedAt: chTime}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	client := mangadex.NewClient()
	client.BaseURL = srv.URL

	n := &recordingNotifier{}
	upd := updater.New(database, client, client)
	s := NewScheduler(database, n, upd, []int64{chatID})

	s.performUpdate(context.Background())

	if len(n.sent[chatID]) != 1 {
		t.Fatalf("messages sent to %d = %d, want 1", chatID, len(n.sent[chatID]))
	}
	if got := n.sent[chatID][0]; got == "" {
		t.Fatal("expected non-empty notification text")
	}
	if want := fmt.Sprintf("<b>%s</b> has new chapters:", mangaTitle); !strings.Contains(n.sent[chatID][0], want) {
		t.Fatalf("notification missing title line: want contains %q", want)
	}
}
