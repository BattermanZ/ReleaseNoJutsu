package db

import (
	"path/filepath"
	"testing"
	"time"
)

func setupDBCoverageTest(t *testing.T) *DB {
	t.Helper()
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
	return database
}

func TestChapterProgressWrappersAndMarkUnread(t *testing.T) {
	database := setupDBCoverageTest(t)

	userID := int64(1)
	ensureTestUser(t, database, userID)
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range []string{"1", "2", "3"} {
		if err := database.AddChapter(mangaID, n, "t"+n, ts, ts, ts, ts); err != nil {
			t.Fatalf("AddChapter(%s): %v", n, err)
		}
	}

	if lastRead, ok, err := database.GetLastReadNumber(int(mangaID)); err != nil || ok || lastRead != 0 {
		t.Fatalf("GetLastReadNumber(initial)=(%v,%v,%v), want (0,false,nil)", lastRead, ok, err)
	}

	if err := database.MarkChapterAsRead(int(mangaID), "3"); err != nil {
		t.Fatalf("MarkChapterAsRead(3): %v", err)
	}
	if unread, err := database.GetUnreadCount(int(mangaID)); err != nil || unread != 0 {
		t.Fatalf("GetUnreadCount(after read all)=(%d,%v), want (0,nil)", unread, err)
	}
	if cnt, err := database.CountReadNumericChaptersInRange(int(mangaID), 1, 4); err != nil || cnt != 3 {
		t.Fatalf("CountReadNumericChaptersInRange(after read)=(%d,%v), want (3,nil)", cnt, err)
	}

	if err := database.MarkChapterAsUnread(int(mangaID), "2"); err != nil {
		t.Fatalf("MarkChapterAsUnread(2): %v", err)
	}
	if lastRead, ok, err := database.GetLastReadNumber(int(mangaID)); err != nil || !ok || lastRead != 1 {
		t.Fatalf("GetLastReadNumber(after unread)=(%v,%v,%v), want (1,true,nil)", lastRead, ok, err)
	}
	if unread, err := database.GetUnreadCount(int(mangaID)); err != nil || unread != 2 {
		t.Fatalf("GetUnreadCount(after unread)=(%d,%v), want (2,nil)", unread, err)
	}
	if cnt, err := database.CountReadNumericChaptersInRange(int(mangaID), 1, 4); err != nil || cnt != 1 {
		t.Fatalf("CountReadNumericChaptersInRange(after unread)=(%d,%v), want (1,nil)", cnt, err)
	}
}

func TestGetUnreadAndReadChaptersRows(t *testing.T) {
	database := setupDBCoverageTest(t)

	userID := int64(1)
	ensureTestUser(t, database, userID)
	mangaID, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "Dragon Ball", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range []string{"1", "2", "3"} {
		if err := database.AddChapter(mangaID, n, "t"+n, ts, ts, ts, ts); err != nil {
			t.Fatalf("AddChapter(%s): %v", n, err)
		}
	}
	if err := database.MarkChapterAsRead(int(mangaID), "2"); err != nil {
		t.Fatalf("MarkChapterAsRead(2): %v", err)
	}

	unreadRows, err := database.GetUnreadChapters(int(mangaID))
	if err != nil {
		t.Fatalf("GetUnreadChapters(): %v", err)
	}
	defer func() { _ = unreadRows.Close() }()

	var unread []string
	for unreadRows.Next() {
		var num, title string
		if err := unreadRows.Scan(&num, &title); err != nil {
			t.Fatalf("scan unread row: %v", err)
		}
		unread = append(unread, num)
	}
	if err := unreadRows.Err(); err != nil {
		t.Fatalf("unreadRows.Err(): %v", err)
	}
	if len(unread) != 1 || unread[0] != "3" {
		t.Fatalf("unread rows=%v, want [3]", unread)
	}

	readRows, err := database.GetReadChapters(int(mangaID))
	if err != nil {
		t.Fatalf("GetReadChapters(): %v", err)
	}
	defer func() { _ = readRows.Close() }()

	var read []string
	for readRows.Next() {
		var num, title string
		if err := readRows.Scan(&num, &title); err != nil {
			t.Fatalf("scan read row: %v", err)
		}
		read = append(read, num)
	}
	if err := readRows.Err(); err != nil {
		t.Fatalf("readRows.Err(): %v", err)
	}
	if len(read) != 2 || read[0] != "2" || read[1] != "1" {
		t.Fatalf("read rows=%v, want [2 1]", read)
	}
}

func TestReadAndUnreadRangeCountsAndRecalculate(t *testing.T) {
	database := setupDBCoverageTest(t)

	userID := int64(1)
	ensureTestUser(t, database, userID)
	mangaID, err := database.AddManga("95f5f24f-f6a4-4f08-a4ca-5a16552f6b73", "Count Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range []string{"1", "2", "3", "10.5"} {
		if err := database.AddChapter(mangaID, n, "t"+n, ts, ts, ts, ts); err != nil {
			t.Fatalf("AddChapter(%s): %v", n, err)
		}
	}
	if err := database.MarkChapterAsRead(int(mangaID), "2"); err != nil {
		t.Fatalf("MarkChapterAsRead(2): %v", err)
	}

	readCount, err := database.CountReadChapters(int(mangaID))
	if err != nil {
		t.Fatalf("CountReadChapters(): %v", err)
	}
	if readCount != 2 {
		t.Fatalf("readCount=%d, want 2", readCount)
	}

	unreadInRange, err := database.CountUnreadNumericChaptersInRange(int(mangaID), 1, 4)
	if err != nil {
		t.Fatalf("CountUnreadNumericChaptersInRange(): %v", err)
	}
	if unreadInRange != 1 {
		t.Fatalf("unreadInRange=%d, want 1", unreadInRange)
	}

	if err := database.RecalculateUnreadCount(int(mangaID)); err != nil {
		t.Fatalf("RecalculateUnreadCount(): %v", err)
	}

	var unreadTotal int
	if err := database.QueryRow("SELECT unread_count FROM manga WHERE id = ?", mangaID).Scan(&unreadTotal); err != nil {
		t.Fatalf("select unread_count: %v", err)
	}
	if unreadTotal != 2 {
		t.Fatalf("unread_count=%d, want 2", unreadTotal)
	}
}

func TestMangaMethods_CRUDAndDetails(t *testing.T) {
	database := setupDBCoverageTest(t)

	user1 := int64(1)
	user2 := int64(2)
	ensureTestUser(t, database, user1)
	ensureTestUser(t, database, user2)

	mangaID, err := database.AddMangaWithMangaPlus("40bc649f-7b49-4645-859e-6cd94136e722", "Dragon Ball", true, user1)
	if err != nil {
		t.Fatalf("AddMangaWithMangaPlus(): %v", err)
	}
	if _, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Other User Manga", user2); err != nil {
		t.Fatalf("AddManga(user2): %v", err)
	}

	isPlus, err := database.IsMangaPlus(int(mangaID))
	if err != nil {
		t.Fatalf("IsMangaPlus(initial): %v", err)
	}
	if !isPlus {
		t.Fatal("expected manga plus enabled")
	}
	if err := database.SetMangaPlus(int(mangaID), false); err != nil {
		t.Fatalf("SetMangaPlus(false): %v", err)
	}
	isPlus, err = database.IsMangaPlus(int(mangaID))
	if err != nil {
		t.Fatalf("IsMangaPlus(after false): %v", err)
	}
	if isPlus {
		t.Fatal("expected manga plus disabled")
	}

	seenAt := time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC)
	if err := database.UpdateMangaLastSeenAt(int(mangaID), seenAt); err != nil {
		t.Fatalf("UpdateMangaLastSeenAt(): %v", err)
	}
	mdID, title, _, gotSeenAt, err := database.GetManga(int(mangaID))
	if err != nil {
		t.Fatalf("GetManga(): %v", err)
	}
	if mdID != "40bc649f-7b49-4645-859e-6cd94136e722" || title != "Dragon Ball" {
		t.Fatalf("GetManga id/title mismatch: (%q,%q)", mdID, title)
	}
	if !gotSeenAt.Equal(seenAt) {
		t.Fatalf("GetManga last_seen_at=%v, want %v", gotSeenAt, seenAt)
	}

	rows, err := database.GetAllMangaByUser(user1)
	if err != nil {
		t.Fatalf("GetAllMangaByUser(): %v", err)
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(): %v", err)
	}
	if count != 1 {
		t.Fatalf("GetAllMangaByUser count=%d, want 1", count)
	}

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range []string{"1", "2"} {
		if err := database.AddChapter(mangaID, n, "t"+n, ts, ts, ts, ts); err != nil {
			t.Fatalf("AddChapter(%s): %v", n, err)
		}
	}
	if err := database.MarkChapterAsRead(int(mangaID), "1"); err != nil {
		t.Fatalf("MarkChapterAsRead(1): %v", err)
	}

	details, err := database.GetMangaDetails(int(mangaID), user1)
	if err != nil {
		t.Fatalf("GetMangaDetails(): %v", err)
	}
	if details.Title != "Dragon Ball" || details.MangaDexID != "40bc649f-7b49-4645-859e-6cd94136e722" {
		t.Fatalf("details mismatch: %+v", details)
	}
	if details.ChaptersTotal != 2 || details.NumericChaptersTotal != 2 {
		t.Fatalf("unexpected details counts: %+v", details)
	}

	gotTitle, err := database.GetMangaTitle(int(mangaID), user1)
	if err != nil {
		t.Fatalf("GetMangaTitle(): %v", err)
	}
	if gotTitle != "Dragon Ball" {
		t.Fatalf("GetMangaTitle=%q, want Dragon Ball", gotTitle)
	}

	belongs, err := database.MangaBelongsToUser(int(mangaID), user1)
	if err != nil {
		t.Fatalf("MangaBelongsToUser(user1): %v", err)
	}
	if !belongs {
		t.Fatal("expected manga to belong to user1")
	}
	belongs, err = database.MangaBelongsToUser(int(mangaID), user2)
	if err != nil {
		t.Fatalf("MangaBelongsToUser(user2): %v", err)
	}
	if belongs {
		t.Fatal("did not expect manga to belong to user2")
	}

	if err := database.DeleteManga(int(mangaID), user1); err != nil {
		t.Fatalf("DeleteManga(): %v", err)
	}
	belongs, err = database.MangaBelongsToUser(int(mangaID), user1)
	if err != nil {
		t.Fatalf("MangaBelongsToUser(after delete): %v", err)
	}
	if belongs {
		t.Fatal("expected manga to be deleted")
	}

	var chapterCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM chapters WHERE manga_id = ?", mangaID).Scan(&chapterCount); err != nil {
		t.Fatalf("count chapters after delete: %v", err)
	}
	if chapterCount != 0 {
		t.Fatalf("chapterCount=%d, want 0 after deletion", chapterCount)
	}
}
