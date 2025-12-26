package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureUser_IsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	chatID := int64(12345)
	if err := database.EnsureUser(chatID); err != nil {
		t.Fatalf("EnsureUser(1): %v", err)
	}
	if err := database.EnsureUser(chatID); err != nil {
		t.Fatalf("EnsureUser(2): %v", err)
	}

	rows, err := database.GetAllUsers()
	if err != nil {
		t.Fatalf("GetAllUsers(): %v", err)
	}
	defer func() { _ = rows.Close() }()

	var count int
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(): %v", err)
	}
	if count != 1 {
		t.Fatalf("users count=%d, want 1", count)
	}
}

func TestListManga_DoesNotHoldConnectionOpen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super")
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	if err := database.SetLastReadAt(int(mangaID), time.Now().UTC()); err != nil {
		t.Fatalf("SetLastReadAt(): %v", err)
	}

	manga, err := database.ListManga()
	if err != nil {
		t.Fatalf("ListManga(): %v", err)
	}
	if len(manga) != 1 {
		t.Fatalf("ListManga() len=%d, want 1", len(manga))
	}

	done := make(chan error, 1)
	go func() {
		done <- database.UpdateMangaLastChecked(int(mangaID))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("UpdateMangaLastChecked(): %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("UpdateMangaLastChecked() appears blocked (possible connection/rows leak)")
	}
}
