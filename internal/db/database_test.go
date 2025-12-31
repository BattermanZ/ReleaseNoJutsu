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

func TestGetLastReadChapterAndListUnreadChapters(t *testing.T) {
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

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	if err := database.AddChapter(mangaID, "1", "One", t1, t1, t1, t1); err != nil {
		t.Fatalf("AddChapter(1): %v", err)
	}
	if err := database.AddChapter(mangaID, "2", "Two", t2, t2, t2, t2); err != nil {
		t.Fatalf("AddChapter(2): %v", err)
	}
	if err := database.AddChapter(mangaID, "3", "Three", t3, t3, t3, t3); err != nil {
		t.Fatalf("AddChapter(3): %v", err)
	}

	// No last_read_number set yet.
	if num, title, ok, err := database.GetLastReadChapter(int(mangaID)); err != nil || ok || num != "" || title != "" {
		t.Fatalf("GetLastReadChapter(no read) = (%q,%q,%v,%v), want ok=false and empty", num, title, ok, err)
	}

	// Mark chapter 1 as read -> unread should be 2 and 3.
	if err := database.MarkChapterAsRead(int(mangaID), "1"); err != nil {
		t.Fatalf("MarkChapterAsRead(1): %v", err)
	}

	if num, title, ok, err := database.GetLastReadChapter(int(mangaID)); err != nil || !ok || num != "1" || title != "One" {
		t.Fatalf("GetLastReadChapter() = (%q,%q,%v,%v), want (1,One,true,nil)", num, title, ok, err)
	}

	items, err := database.ListUnreadChapters(int(mangaID), 10, 0)
	if err != nil {
		t.Fatalf("ListUnreadChapters(): %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ListUnreadChapters() len=%d, want 2", len(items))
	}
	if items[0].Number != "2" || items[1].Number != "3" {
		t.Fatalf("ListUnreadChapters() numbers=%q,%q, want 2,3", items[0].Number, items[1].Number)
	}

	page1, err := database.ListUnreadChapters(int(mangaID), 1, 0)
	if err != nil {
		t.Fatalf("ListUnreadChapters(page1): %v", err)
	}
	page2, err := database.ListUnreadChapters(int(mangaID), 1, 1)
	if err != nil {
		t.Fatalf("ListUnreadChapters(page2): %v", err)
	}
	if len(page1) != 1 || len(page2) != 1 || page1[0].Number != "2" || page2[0].Number != "3" {
		t.Fatalf("pagination got %v then %v, want 2 then 3", page1, page2)
	}
}

func TestListUnreadBucketStartsAndRangeListing(t *testing.T) {
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

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	add := func(num string) {
		t.Helper()
		if err := database.AddChapter(mangaID, num, "t", ts, ts, ts, ts); err != nil {
			t.Fatalf("AddChapter(%s): %v", num, err)
		}
	}
	add("1")
	add("5")
	add("100")
	add("250")
	add("999")
	add("1000")
	add("1100")
	add("1115")
	add("1999")
	add("2000")

	got, err := database.ListUnreadBucketStarts(int(mangaID), 1000, 1, 1.0e18)
	if err != nil {
		t.Fatalf("ListUnreadBucketStarts(1000): %v", err)
	}
	want := []int{1, 1000, 2000}
	if len(got) != len(want) {
		t.Fatalf("ListUnreadBucketStarts(1000)=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListUnreadBucketStarts(1000)=%v, want %v", got, want)
		}
	}

	got, err = database.ListUnreadBucketStarts(int(mangaID), 100, 1, 1000)
	if err != nil {
		t.Fatalf("ListUnreadBucketStarts(100): %v", err)
	}
	want = []int{1, 100, 200, 900}
	if len(got) != len(want) {
		t.Fatalf("ListUnreadBucketStarts(100)=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListUnreadBucketStarts(100)=%v, want %v", got, want)
		}
	}

	got, err = database.ListUnreadBucketStarts(int(mangaID), 10, 1100, 1200)
	if err != nil {
		t.Fatalf("ListUnreadBucketStarts(10): %v", err)
	}
	want = []int{1100, 1110}
	if len(got) != len(want) {
		t.Fatalf("ListUnreadBucketStarts(10)=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListUnreadBucketStarts(10)=%v, want %v", got, want)
		}
	}

	chs, err := database.ListUnreadNumericChaptersInRange(int(mangaID), 1110, 1120, 50, 0)
	if err != nil {
		t.Fatalf("ListUnreadNumericChaptersInRange(): %v", err)
	}
	if len(chs) != 1 || chs[0].Number != "1115" {
		t.Fatalf("ListUnreadNumericChaptersInRange()=%v, want [1115]", chs)
	}
}

func TestListReadBucketStartsAndRangeListing(t *testing.T) {
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

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	add := func(num string) {
		t.Helper()
		if err := database.AddChapter(mangaID, num, "t", ts, ts, ts, ts); err != nil {
			t.Fatalf("AddChapter(%s): %v", num, err)
		}
	}
	add("1")
	add("5")
	add("100")
	add("250")
	add("999")
	add("1000")
	add("1100")
	add("1115")
	add("1999")
	add("2000")

	// Mark read up to 1115.
	if err := database.MarkChapterAsRead(int(mangaID), "1115"); err != nil {
		t.Fatalf("MarkChapterAsRead(1115): %v", err)
	}

	got, err := database.ListReadBucketStarts(int(mangaID), 1000, 1, 1.0e18)
	if err != nil {
		t.Fatalf("ListReadBucketStarts(1000): %v", err)
	}
	want := []int{1000, 1}
	if len(got) != len(want) {
		t.Fatalf("ListReadBucketStarts(1000)=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListReadBucketStarts(1000)=%v, want %v", got, want)
		}
	}

	got, err = database.ListReadBucketStarts(int(mangaID), 100, 1, 1000)
	if err != nil {
		t.Fatalf("ListReadBucketStarts(100): %v", err)
	}
	want = []int{900, 200, 100, 1}
	if len(got) != len(want) {
		t.Fatalf("ListReadBucketStarts(100)=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListReadBucketStarts(100)=%v, want %v", got, want)
		}
	}

	got, err = database.ListReadBucketStarts(int(mangaID), 10, 1100, 1200)
	if err != nil {
		t.Fatalf("ListReadBucketStarts(10): %v", err)
	}
	want = []int{1110, 1100}
	if len(got) != len(want) {
		t.Fatalf("ListReadBucketStarts(10)=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListReadBucketStarts(10)=%v, want %v", got, want)
		}
	}

	chs, err := database.ListReadNumericChaptersInRange(int(mangaID), 1110, 1120, 50, 0)
	if err != nil {
		t.Fatalf("ListReadNumericChaptersInRange(): %v", err)
	}
	if len(chs) != 1 || chs[0].Number != "1115" {
		t.Fatalf("ListReadNumericChaptersInRange()=%v, want [1115]", chs)
	}
}
