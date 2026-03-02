package bot

import (
	"strings"
	"testing"

	"releasenojutsu/internal/appcopy"
)

func TestSendHelpMessage_UsesHelpCopy(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	b.sendHelpMessage(42)

	msg := api.lastMessageConfig(t)
	if msg.ParseMode != "Markdown" {
		t.Fatalf("ParseMode=%q, want Markdown", msg.ParseMode)
	}
	if msg.Text != appcopy.Copy.Info.HelpText {
		t.Fatalf("help text mismatch")
	}
	if !hasCallback(messageCallbacks(t, msg), cbMainMenu()) {
		t.Fatalf("help message missing main-menu callback")
	}
}

func TestEnsureUser_OnlyStoresPrivateChatIDs(t *testing.T) {
	b, database, _ := setupBotForMessageTests(t)

	b.ensureUser(42, 42, false)
	ok, _, err := database.IsUserAuthorized(42)
	if err != nil {
		t.Fatalf("IsUserAuthorized(42): %v", err)
	}
	if !ok {
		t.Fatal("expected private chat user to be stored")
	}

	b.ensureUser(-100, 42, false)
	ok, _, err = database.IsUserAuthorized(-100)
	if err != nil {
		t.Fatalf("IsUserAuthorized(-100): %v", err)
	}
	if ok {
		t.Fatal("did not expect group chat id to be stored")
	}

	b.ensureUser(43, 42, false)
	ok, _, err = database.IsUserAuthorized(43)
	if err != nil {
		t.Fatalf("IsUserAuthorized(43): %v", err)
	}
	if ok {
		t.Fatal("did not expect mismatched chat/from ids to be stored")
	}
}

func TestSendUnauthorizedMessage_SendsPrompt(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	b.sendUnauthorizedMessage(42)

	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.Unauthorized {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.Unauthorized)
	}
}

func TestSendMarkAllReadConfirm_HasConfirmButtons(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "Dragon Ball", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	b.sendMarkAllReadConfirm(userID, userID, int(mangaID))

	msg := api.lastMessageConfig(t)
	if !strings.Contains(msg.Text, "Dragon Ball") {
		t.Fatalf("missing manga title in mark-all-read confirm: %q", msg.Text)
	}
	callbacks := messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbMangaAction(int(mangaID), "mark_all_read_yes")) {
		t.Fatalf("missing mark-all-read confirmation callback")
	}
	if !hasCallback(callbacks, cbMangaAction(int(mangaID), "menu")) {
		t.Fatalf("missing cancel callback back to menu")
	}
}

func TestHandleMarkChapterAsReadAndUnread_UpdateProgress(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("95f5f24f-f6a4-4f08-a4ca-5a16552f6b73", "Progress Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "2", "3")

	b.handleMarkChapterAsRead(userID, userID, int(mangaID), "2")
	lastRead, ok, err := database.GetLastReadNumber(int(mangaID))
	if err != nil {
		t.Fatalf("GetLastReadNumber(after read): %v", err)
	}
	if !ok || lastRead != 2 {
		t.Fatalf("last_read_number=(%v,%v), want (2,true)", lastRead, ok)
	}
	unread, err := database.CountUnreadChapters(int(mangaID))
	if err != nil {
		t.Fatalf("CountUnreadChapters(after read): %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread=%d, want 1", unread)
	}
	if got := api.lastMessageText(t); !strings.Contains(got, "Chapter <b>2</b>") {
		t.Fatalf("unexpected read message: %q", got)
	}

	b.handleMarkChapterAsUnread(userID, userID, int(mangaID), "2")
	lastRead, ok, err = database.GetLastReadNumber(int(mangaID))
	if err != nil {
		t.Fatalf("GetLastReadNumber(after unread): %v", err)
	}
	if !ok || lastRead != 1 {
		t.Fatalf("last_read_number=(%v,%v), want (1,true)", lastRead, ok)
	}
	unread, err = database.CountUnreadChapters(int(mangaID))
	if err != nil {
		t.Fatalf("CountUnreadChapters(after unread): %v", err)
	}
	if unread != 2 {
		t.Fatalf("unread=%d, want 2", unread)
	}
	if got := api.lastMessageText(t); !strings.Contains(got, "marked as unread") {
		t.Fatalf("unexpected unread message: %q", got)
	}
}

func TestMarkReadBucketMenus_RenderExpectedCallbacks(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("f6f1f63b-4dd4-4a0a-8db8-166b7ccf090b", "Read Buckets", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "1001", "1101", "1111")

	b.sendMarkReadThousandsMenuPage(userID, userID, int(mangaID), 0)
	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 1000, 1)) {
		t.Fatalf("missing unread thousand bucket callback for 1")
	}
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 1000, 1000)) {
		t.Fatalf("missing unread thousand bucket callback for 1000")
	}

	b.sendMarkReadHundredsMenu(userID, userID, int(mangaID), 1000, false)
	callbacks = messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 100, 1000)) {
		t.Fatalf("missing unread hundred bucket callback for 1000")
	}
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 100, 1100)) {
		t.Fatalf("missing unread hundred bucket callback for 1100")
	}
	if !hasCallback(callbacks, cbMarkReadBackRoot(int(mangaID))) {
		t.Fatalf("missing back-to-root callback")
	}

	b.sendMarkReadTensMenu(userID, userID, int(mangaID), 1100, false)
	callbacks = messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 10, 1100)) {
		t.Fatalf("missing unread tens bucket callback for 1100")
	}
	if !hasCallback(callbacks, cbMarkReadPick(int(mangaID), 10, 1110)) {
		t.Fatalf("missing unread tens bucket callback for 1110")
	}
	if !hasCallback(callbacks, cbMarkReadBackHundreds(int(mangaID), 1100)) {
		t.Fatalf("missing back-to-hundreds callback")
	}
}

func TestMarkUnreadBucketAndChapterMenus_RenderExpectedCallbacks(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("95f5f24f-f6a4-4f08-a4ca-5a16552f6b73", "Unread Buckets", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "2", "3", "1001", "1101", "1111")
	if err := database.MarkAllChaptersAsRead(int(mangaID)); err != nil {
		t.Fatalf("MarkAllChaptersAsRead(): %v", err)
	}

	title, err := database.GetMangaTitle(int(mangaID), userID)
	if err != nil {
		t.Fatalf("GetMangaTitle(): %v", err)
	}
	lastReadLine := b.lastReadLine(int(mangaID))

	b.sendMarkUnreadDirectChaptersMenu(userID, userID, int(mangaID), 6, title, lastReadLine)
	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkChapterUnread(int(mangaID), "1111")) {
		t.Fatalf("missing direct unread callback for highest read chapter")
	}

	b.sendMarkUnreadThousandsMenuPage(userID, userID, int(mangaID), 0)
	callbacks = messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 1000, 1)) {
		t.Fatalf("missing read thousand bucket callback for 1")
	}
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 1000, 1000)) {
		t.Fatalf("missing read thousand bucket callback for 1000")
	}

	b.sendMarkUnreadHundredsMenu(userID, userID, int(mangaID), 1000, false)
	callbacks = messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 100, 1000)) {
		t.Fatalf("missing read hundred bucket callback for 1000")
	}
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 100, 1100)) {
		t.Fatalf("missing read hundred bucket callback for 1100")
	}
	if !hasCallback(callbacks, cbMarkUnreadBackRoot(int(mangaID))) {
		t.Fatalf("missing unread back-to-root callback")
	}

	b.sendMarkUnreadTensMenu(userID, userID, int(mangaID), 1100, false)
	callbacks = messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 10, 1100)) {
		t.Fatalf("missing read tens bucket callback for 1100")
	}
	if !hasCallback(callbacks, cbMarkUnreadPick(int(mangaID), 10, 1110)) {
		t.Fatalf("missing read tens bucket callback for 1110")
	}
	if !hasCallback(callbacks, cbMarkUnreadBackHundreds(int(mangaID), 1100)) {
		t.Fatalf("missing unread back-to-hundreds callback")
	}

	b.sendMarkUnreadChaptersMenuPage(userID, userID, int(mangaID), 1110, false, -1)
	callbacks = messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMarkChapterUnread(int(mangaID), "1111")) {
		t.Fatalf("missing unread chapter callback")
	}
	if !hasCallback(callbacks, cbMarkUnreadBackTens(int(mangaID), 1110)) {
		t.Fatalf("missing unread back-to-tens callback")
	}
}

func TestMarkReadStartMenu_NoUnreadShowsUpToDate(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("f6f1f63b-4dd4-4a0a-8db8-166b7ccf090b", "UpToDate Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	b.sendMarkReadStartMenu(userID, userID, int(mangaID))

	if got := api.lastMessageText(t); !strings.Contains(got, "all caught up") {
		t.Fatalf("unexpected up-to-date message: %q", got)
	}
}

func TestMarkUnreadStartMenu_NoReadShowsNothingToUnread(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("95f5f24f-f6a4-4f08-a4ca-5a16552f6b73", "Nothing Unread", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "2", "3")

	b.sendMarkUnreadStartMenu(userID, userID, int(mangaID))

	if got := api.lastMessageText(t); !strings.Contains(got, "Nothing to mark unread yet.") {
		t.Fatalf("unexpected nothing-to-unread message: %q", got)
	}
}

func TestStartMenus_SingleBucketFallsThroughToChapterPages(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("37b87be0-b1f4-4507-affa-06c99ebb27f8", "Single Bucket", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	// 11 chapters, all in the same 1000/100/10 buckets.
	addChapterSet(t, b, mangaID, "1000", "1000.1", "1001", "1002", "1003", "1004", "1005", "1006", "1007", "1008", "1009")

	b.sendMarkReadStartMenu(userID, userID, int(mangaID))
	readCallbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(readCallbacks, cbMarkChapterRead(int(mangaID), "1000")) {
		t.Fatalf("expected direct chapter callbacks for read flow, got %v", readCallbacks)
	}

	if err := database.MarkAllChaptersAsRead(int(mangaID)); err != nil {
		t.Fatalf("MarkAllChaptersAsRead(): %v", err)
	}
	b.sendMarkUnreadStartMenu(userID, userID, int(mangaID))
	unreadCallbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(unreadCallbacks, cbMarkChapterUnread(int(mangaID), "1009")) {
		t.Fatalf("expected direct chapter callbacks for unread flow, got %v", unreadCallbacks)
	}
}

func TestHandleMangaSelection_OwnershipAndActionRouting(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	ownerID := int64(42)
	otherUserID := int64(43)
	if err := database.EnsureUser(ownerID, false); err != nil {
		t.Fatalf("EnsureUser(owner): %v", err)
	}
	if err := database.EnsureUser(otherUserID, false); err != nil {
		t.Fatalf("EnsureUser(other): %v", err)
	}
	mangaID, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "Selection Manga", ownerID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	b.handleMangaSelection(otherUserID, otherUserID, int(mangaID), "menu")
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.NoAccessToManga {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.NoAccessToManga)
	}

	b.handleMangaSelection(ownerID, ownerID, int(mangaID), "mark_all_read")
	if got := api.lastMessageText(t); !strings.Contains(got, "Mark <b>all chapters</b> as read") {
		t.Fatalf("unexpected mark-all-read confirm text: %q", got)
	}

	if err := database.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	b.handleMangaSelection(ownerID, ownerID, int(mangaID), "menu")
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.CannotAccessManga {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.CannotAccessManga)
	}
}
