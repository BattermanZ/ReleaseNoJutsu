package db

import (
	"path/filepath"
	"testing"
	"time"
)

func ensureTestUser(t *testing.T, database *DB, chatID int64) {
	t.Helper()
	if err := database.EnsureUser(chatID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
}

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
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	chatID := int64(12345)
	ensureTestUser(t, database, chatID)
	ensureTestUser(t, database, chatID)

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
	if count != 2 {
		t.Fatalf("users count=%d, want 2 (admin + user)", count)
	}
}

func TestUserPendingStateLifecycle(t *testing.T) {
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

	chatID := int64(12345)
	ensureTestUser(t, database, chatID)

	state, payload, hasState, err := database.GetUserPendingState(chatID)
	if err != nil {
		t.Fatalf("GetUserPendingState(empty): %v", err)
	}
	if hasState || state != "" || payload != "" {
		t.Fatalf("unexpected initial pending state: state=%q payload=%q has=%v", state, payload, hasState)
	}

	if err := database.SetUserPendingState(chatID, "add_manga", "foo"); err != nil {
		t.Fatalf("SetUserPendingState(): %v", err)
	}
	state, payload, hasState, err = database.GetUserPendingState(chatID)
	if err != nil {
		t.Fatalf("GetUserPendingState(after set): %v", err)
	}
	if !hasState || state != "add_manga" || payload != "foo" {
		t.Fatalf("unexpected pending state after set: state=%q payload=%q has=%v", state, payload, hasState)
	}

	if err := database.ClearUserPendingState(chatID); err != nil {
		t.Fatalf("ClearUserPendingState(): %v", err)
	}
	state, payload, hasState, err = database.GetUserPendingState(chatID)
	if err != nil {
		t.Fatalf("GetUserPendingState(after clear): %v", err)
	}
	if hasState || state != "" || payload != "" {
		t.Fatalf("unexpected pending state after clear: state=%q payload=%q has=%v", state, payload, hasState)
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
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	ensureTestUser(t, database, 1)
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", 1)
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
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	ensureTestUser(t, database, 1)
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", 1)
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

func TestMarkAllChaptersAsRead(t *testing.T) {
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

	ensureTestUser(t, database, 1)
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", 1)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := database.AddChapter(mangaID, "1", "One", ts, ts, ts, ts); err != nil {
		t.Fatalf("AddChapter(1): %v", err)
	}
	if err := database.AddChapter(mangaID, "2.5", "TwoPointFive", ts, ts, ts, ts); err != nil {
		t.Fatalf("AddChapter(2.5): %v", err)
	}
	if err := database.AddChapter(mangaID, "3", "Three", ts, ts, ts, ts); err != nil {
		t.Fatalf("AddChapter(3): %v", err)
	}

	if err := database.MarkAllChaptersAsRead(int(mangaID)); err != nil {
		t.Fatalf("MarkAllChaptersAsRead(): %v", err)
	}

	unread, err := database.CountUnreadChapters(int(mangaID))
	if err != nil {
		t.Fatalf("CountUnreadChapters(): %v", err)
	}
	if unread != 0 {
		t.Fatalf("unread=%d, want 0", unread)
	}

	num, title, ok, err := database.GetLastReadChapter(int(mangaID))
	if err != nil {
		t.Fatalf("GetLastReadChapter(): %v", err)
	}
	if !ok || num != "3" || title != "Three" {
		t.Fatalf("GetLastReadChapter()=(%q,%q,%v), want (3,Three,true)", num, title, ok)
	}
}

func TestPairingCodes_OneTimeAndExpiry(t *testing.T) {
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

	code := "ABCD-1234"
	expires := time.Now().UTC().Add(1 * time.Hour)
	if err := database.CreatePairingCode(code, 1, expires); err != nil {
		t.Fatalf("CreatePairingCode(): %v", err)
	}

	ok, err := database.RedeemPairingCode(code, 42)
	if err != nil || !ok {
		t.Fatalf("RedeemPairingCode(ok) = (%v,%v), want (true,nil)", ok, err)
	}

	ok, err = database.RedeemPairingCode(code, 43)
	if err != nil || ok {
		t.Fatalf("RedeemPairingCode(second) = (%v,%v), want (false,nil)", ok, err)
	}

	expired := "FFFF-0000"
	if err := database.CreatePairingCode(expired, 1, time.Now().UTC().Add(-1*time.Hour)); err != nil {
		t.Fatalf("CreatePairingCode(expired): %v", err)
	}
	ok, err = database.RedeemPairingCode(expired, 44)
	if err != nil || ok {
		t.Fatalf("RedeemPairingCode(expired) = (%v,%v), want (false,nil)", ok, err)
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
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	ensureTestUser(t, database, 1)
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", 1)
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
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	ensureTestUser(t, database, 1)
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", 1)
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

func TestMigrate_RepairsFuturePublishAtAndLastSeenAt(t *testing.T) {
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
		t.Fatalf("Migrate(initial): %v", err)
	}

	ensureTestUser(t, database, 1)
	mangaID, err := database.AddManga("0b094aab-0cfb-4837-a49b-7267bdb86bec", "Boruto - Two Blue Vortex", 1)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	created := time.Date(2023, 8, 21, 1, 0, 40, 0, time.UTC)
	readable := time.Date(2023, 8, 21, 1, 0, 41, 0, time.UTC)
	futurePublish := time.Date(2037, 12, 31, 15, 0, 0, 0, time.UTC)

	if _, err := database.Exec(`
		INSERT INTO chapters (manga_id, chapter_number, title, published_at, readable_at, created_at, updated_at)
		VALUES (?, '1', 'Boruto', ?, ?, ?, ?)
	`, mangaID, futurePublish, readable, created, created); err != nil {
		t.Fatalf("insert chapter: %v", err)
	}

	if _, err := database.Exec("UPDATE manga SET last_seen_at = ? WHERE id = ?", futurePublish, mangaID); err != nil {
		t.Fatalf("poison last_seen_at: %v", err)
	}

	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(repair): %v", err)
	}

	var (
		publishedStr string
		lastSeenStr  string
	)
	if err := database.QueryRow("SELECT CAST(published_at AS TEXT) FROM chapters WHERE manga_id = ? AND chapter_number = '1'", mangaID).Scan(&publishedStr); err != nil {
		t.Fatalf("select published_at: %v", err)
	}
	if err := database.QueryRow("SELECT CAST(last_seen_at AS TEXT) FROM manga WHERE id = ?", mangaID).Scan(&lastSeenStr); err != nil {
		t.Fatalf("select last_seen_at: %v", err)
	}

	publishedAt, err := parseSQLiteTime(publishedStr)
	if err != nil {
		t.Fatalf("parse published_at: %v", err)
	}
	lastSeenAt, err := parseSQLiteTime(lastSeenStr)
	if err != nil {
		t.Fatalf("parse last_seen_at: %v", err)
	}

	if publishedAt.After(time.Now().UTC().Add(24 * time.Hour)) {
		t.Fatalf("published_at still in the future after migration repair: %v", publishedAt)
	}
	if lastSeenAt.After(time.Now().UTC().Add(24 * time.Hour)) {
		t.Fatalf("last_seen_at still in the future after migration repair: %v", lastSeenAt)
	}
	if !lastSeenAt.Equal(created) {
		t.Fatalf("last_seen_at=%v, want %v", lastSeenAt, created)
	}
}
