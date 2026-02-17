package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSystemStatus_GlobalAndByUser(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	adminID := int64(1)
	userID := int64(42)
	if err := database.EnsureUser(adminID, true); err != nil {
		t.Fatalf("EnsureUser(admin): %v", err)
	}
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(user): %v", err)
	}

	m1, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Admin Manga", adminID)
	if err != nil {
		t.Fatalf("AddManga(admin): %v", err)
	}
	m2, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "User Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(user): %v", err)
	}

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := database.AddChapter(m1, "1", "One", ts, ts, ts, ts); err != nil {
		t.Fatalf("AddChapter(admin): %v", err)
	}
	if err := database.AddChapter(m2, "1", "One", ts, ts, ts, ts); err != nil {
		t.Fatalf("AddChapter(user1): %v", err)
	}
	if err := database.AddChapter(m2, "2", "Two", ts, ts, ts, ts); err != nil {
		t.Fatalf("AddChapter(user2): %v", err)
	}
	if err := database.MarkChapterAsRead(int(m2), "1"); err != nil {
		t.Fatalf("MarkChapterAsRead(): %v", err)
	}

	database.UpdateCronLastRun()

	global, err := database.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus(): %v", err)
	}
	if global.MangaCount != 2 || global.ChapterCount != 3 || global.UserCount != 2 {
		t.Fatalf("global status mismatch: %+v", global)
	}
	if !global.HasCronLastRun {
		t.Fatalf("expected HasCronLastRun=true, got %+v", global)
	}

	userScoped, err := database.GetStatusByUser(userID)
	if err != nil {
		t.Fatalf("GetStatusByUser(): %v", err)
	}
	if userScoped.MangaCount != 1 || userScoped.ChapterCount != 2 || userScoped.UserCount != 1 {
		t.Fatalf("user status mismatch: %+v", userScoped)
	}
	if userScoped.UnreadTotal != 1 {
		t.Fatalf("user unread=%d, want 1", userScoped.UnreadTotal)
	}
	if !userScoped.HasCronLastRun {
		t.Fatalf("expected user HasCronLastRun=true, got %+v", userScoped)
	}
}

func TestUsersListAndAuthorization(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	if err := database.EnsureUser(1, true); err != nil {
		t.Fatalf("EnsureUser(admin): %v", err)
	}
	if err := database.EnsureUser(42, false); err != nil {
		t.Fatalf("EnsureUser(user): %v", err)
	}

	users, err := database.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers(): %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("ListUsers len=%d, want 2", len(users))
	}

	ok, isAdmin, err := database.IsUserAuthorized(1)
	if err != nil || !ok || !isAdmin {
		t.Fatalf("IsUserAuthorized(1)=(%v,%v,%v), want (true,true,nil)", ok, isAdmin, err)
	}
	ok, isAdmin, err = database.IsUserAuthorized(42)
	if err != nil || !ok || isAdmin {
		t.Fatalf("IsUserAuthorized(42)=(%v,%v,%v), want (true,false,nil)", ok, isAdmin, err)
	}
	ok, isAdmin, err = database.IsUserAuthorized(999)
	if err != nil || ok || isAdmin {
		t.Fatalf("IsUserAuthorized(999)=(%v,%v,%v), want (false,false,nil)", ok, isAdmin, err)
	}
}
