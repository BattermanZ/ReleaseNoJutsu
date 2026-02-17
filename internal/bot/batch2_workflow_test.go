package bot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func addChapterSet(t *testing.T, b *Bot, mangaID int64, nums ...string) {
	t.Helper()
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range nums {
		if err := b.db.AddChapter(mangaID, n, "T"+n, ts, ts, ts, ts); err != nil {
			t.Fatalf("AddChapter(%s): %v", n, err)
		}
	}
}

func messageCallbacks(t *testing.T, msg tgbotapi.MessageConfig) []string {
	t.Helper()
	keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("ReplyMarkup type = %T, want InlineKeyboardMarkup", msg.ReplyMarkup)
	}
	var out []string
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil {
				out = append(out, *btn.CallbackData)
			}
		}
	}
	return out
}

func hasCallback(callbacks []string, want string) bool {
	for _, cb := range callbacks {
		if cb == want {
			return true
		}
	}
	return false
}

func TestMarkReadStartMenu_DirectFlowWhenUnreadAtMostTen(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "Dragon Ball", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "2", "3")

	b.sendMarkReadStartMenu(userID, userID, int(mangaID))

	msg := api.lastMessageConfig(t)
	if !strings.Contains(msg.Text, "Dragon Ball") {
		t.Fatalf("expected manga title in message, got %q", msg.Text)
	}
	callbacks := messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbMarkChapterRead(int(mangaID), "1")) {
		t.Fatalf("expected direct chapter callback, got %v", callbacks)
	}
}

func TestMarkReadStartMenu_UsesThousandsMenuForLargeUnread(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Dragon Ball Super", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "1001")

	b.sendMarkReadStartMenu(userID, userID, int(mangaID))

	msg := api.lastMessageConfig(t)
	callbacks := messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 1000, 1)) {
		t.Fatalf("missing 1-999 bucket callback in %v", callbacks)
	}
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 1000, 1000)) {
		t.Fatalf("missing 1000-1999 bucket callback in %v", callbacks)
	}
}

func TestMarkReadChaptersMenuPage_ClampsPageBounds(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("f6f1f63b-4dd4-4a0a-8db8-166b7ccf090b", "Many Chapters", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	// 40 chapters in [10,20) => 2 pages with pageSize=30.
	nums := make([]string, 0, 40)
	for i := 1; i <= 40; i++ {
		nums = append(nums, fmt.Sprintf("10.%02d", i))
	}
	addChapterSet(t, b, mangaID, nums...)

	b.sendMarkReadChaptersMenuPage(userID, userID, int(mangaID), 10, false, -2)
	msg := api.lastMessageConfig(t)
	callbacks := messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbMarkReadChapterPage(int(mangaID), 10, false, 1)) {
		t.Fatalf("expected next page callback on first page, got %v", callbacks)
	}
	if hasCallback(callbacks, cbMarkReadChapterPage(int(mangaID), 10, false, -1)) {
		t.Fatalf("unexpected negative page callback in %v", callbacks)
	}

	b.sendMarkReadChaptersMenuPage(userID, userID, int(mangaID), 10, false, 999)
	msg = api.lastMessageConfig(t)
	callbacks = messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbMarkReadChapterPage(int(mangaID), 10, false, 0)) {
		t.Fatalf("expected prev page callback on last page, got %v", callbacks)
	}
	if hasCallback(callbacks, cbMarkReadChapterPage(int(mangaID), 10, false, 2)) {
		t.Fatalf("unexpected next callback beyond max page in %v", callbacks)
	}
}

func TestMarkUnreadStartMenu_UsesThousandsMenuForLargeReadSet(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("95f5f24f-f6a4-4f08-a4ca-5a16552f6b73", "Unread Flow", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "1001")
	if err := database.MarkAllChaptersAsRead(int(mangaID)); err != nil {
		t.Fatalf("MarkAllChaptersAsRead(): %v", err)
	}

	b.sendMarkUnreadStartMenu(userID, userID, int(mangaID))

	msg := api.lastMessageConfig(t)
	callbacks := messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 1000, 1)) {
		t.Fatalf("missing read bucket callback for 1-999 in %v", callbacks)
	}
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 1000, 1000)) {
		t.Fatalf("missing read bucket callback for 1000-1999 in %v", callbacks)
	}
}

func TestMenuHelpers_BucketsAndBreadcrumbs(t *testing.T) {
	if got := bucketLabel(1, 100); got != "1-99" {
		t.Fatalf("bucketLabel(1,100)=%q, want 1-99", got)
	}
	if got := bucketLabel(300, 100); got != "300-399" {
		t.Fatalf("bucketLabel(300,100)=%q, want 300-399", got)
	}

	s, e := bucketRange(1, 100)
	if s != 1 || e != 100 {
		t.Fatalf("bucketRange(1,100)=(%v,%v), want (1,100)", s, e)
	}
	s, e = bucketRange(300, 100)
	if s != 300 || e != 400 {
		t.Fatalf("bucketRange(300,100)=(%v,%v), want (300,400)", s, e)
	}

	if thousandBucketStart(999) != 1 {
		t.Fatalf("thousandBucketStart(999)=%d, want 1", thousandBucketStart(999))
	}
	if thousandBucketStart(1450) != 1000 {
		t.Fatalf("thousandBucketStart(1450)=%d, want 1000", thousandBucketStart(1450))
	}
	if hundredBucketStart(99) != 1 {
		t.Fatalf("hundredBucketStart(99)=%d, want 1", hundredBucketStart(99))
	}
	if hundredBucketStart(245) != 200 {
		t.Fatalf("hundredBucketStart(245)=%d, want 200", hundredBucketStart(245))
	}

	if boolToInt(true) != 1 || boolToInt(false) != 0 {
		t.Fatal("boolToInt conversion mismatch")
	}
	if !intToBool(1) || intToBool(0) {
		t.Fatal("intToBool conversion mismatch")
	}
}
