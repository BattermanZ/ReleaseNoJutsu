package cron

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
	"releasenojutsu/internal/updater"
)

func setupSchedulerForRunTests(t *testing.T, handler http.HandlerFunc) (*Scheduler, *db.DB, int64) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}
	chatID := int64(42)
	if err := database.Migrate(chatID); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}
	if err := database.EnsureUser(chatID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	if _, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", chatID); err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := mangadex.NewClient()
	client.BaseURL = srv.URL
	upd := updater.New(database, client, client)
	return NewScheduler(database, &recordingNotifier{}, upd), database, chatID
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	s, _, _ := setupSchedulerForRunTests(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mangadex.ChapterFeedResponse{Data: []mangadex.Chapter{}})
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after context cancel")
	}
}

func TestPerformUpdate_SkipsOverlappingRun(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	s, _, _ := setupSchedulerForRunTests(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		_ = json.NewEncoder(w).Encode(mangadex.ChapterFeedResponse{Data: []mangadex.Chapter{}})
	})

	done1 := make(chan struct{})
	go func() {
		s.performUpdate(context.Background())
		close(done1)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first performUpdate did not start")
	}

	start := time.Now()
	s.performUpdate(context.Background())
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("overlapping performUpdate should return quickly, took %s", time.Since(start))
	}

	close(release)
	select {
	case <-done1:
	case <-time.After(2 * time.Second):
		t.Fatal("first performUpdate did not finish after release")
	}
}
