package bot

import (
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
)

func TestSendMainMenu_NonAdminButtons(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	b.sendMainMenu(userID)

	msg := api.lastMessageConfig(t)
	if msg.ParseMode != "Markdown" {
		t.Fatalf("ParseMode=%q, want Markdown", msg.ParseMode)
	}
	callbacks := messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbAddManga()) {
		t.Fatalf("main menu missing add callback: %v", callbacks)
	}
	if !hasCallback(callbacks, cbListManga()) {
		t.Fatalf("main menu missing list callback: %v", callbacks)
	}
	if hasCallback(callbacks, cbGenPair()) {
		t.Fatalf("non-admin main menu should not contain gen pair callback: %v", callbacks)
	}
}

func TestSendMainMenu_AdminIncludesGeneratePairButton(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	adminID := int64(1)
	b.sendMainMenu(adminID)

	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbGenPair()) {
		t.Fatalf("admin main menu missing gen pair callback: %v", callbacks)
	}
}

func TestSendMessageWithMainMenuButton_AppendsToExistingKeyboard(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	msg := tgbotapi.NewMessage(42, "hello")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("X", "x")),
	)
	b.sendMessageWithMainMenuButton(msg)

	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, "x") {
		t.Fatalf("existing callback not preserved: %v", callbacks)
	}
	if !hasCallback(callbacks, cbMainMenu()) {
		t.Fatalf("main menu callback not appended: %v", callbacks)
	}
}

func TestSendMessageWithMainMenuButton_NonInlineReplyMarkupFallsBackToMainMenu(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	msg := tgbotapi.NewMessage(42, "hello")
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
	b.sendMessageWithMainMenuButton(msg)

	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if len(callbacks) != 1 || callbacks[0] != cbMainMenu() {
		t.Fatalf("expected only main menu callback, got %v", callbacks)
	}
}

func TestSendMangaScopedMessage_AddsContextNavigation(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	msg := tgbotapi.NewMessage(42, "context")
	b.sendMangaScopedMessage(msg, 123)

	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbMangaAction(123, "menu")) {
		t.Fatalf("missing back-to-manga callback: %v", callbacks)
	}
	if !hasCallback(callbacks, cbListManga()) {
		t.Fatalf("missing back-to-list callback: %v", callbacks)
	}
	if !hasCallback(callbacks, cbMainMenu()) {
		t.Fatalf("missing main menu callback: %v", callbacks)
	}
}

func TestSendMangaScopedMessage_PreservesExistingInlineKeyboard(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	msg := tgbotapi.NewMessage(42, "context")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Existing", "existing_cb")),
	)
	b.sendMangaScopedMessage(msg, 321)

	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, "existing_cb") {
		t.Fatalf("missing preserved existing callback: %v", callbacks)
	}
	if !hasCallback(callbacks, cbMangaAction(321, "menu")) {
		t.Fatalf("missing back-to-manga callback: %v", callbacks)
	}
	if !hasCallback(callbacks, cbListManga()) {
		t.Fatalf("missing back-to-list callback: %v", callbacks)
	}
	if !hasCallback(callbacks, cbMainMenu()) {
		t.Fatalf("missing main menu callback: %v", callbacks)
	}
}

func TestSendListScopedMessage_AddsListAndMainMenuNavigation(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	msg := tgbotapi.NewMessage(42, "list-context")
	b.sendListScopedMessage(msg)

	callbacks := messageCallbacks(t, api.lastMessageConfig(t))
	if !hasCallback(callbacks, cbListManga()) {
		t.Fatalf("missing back-to-list callback: %v", callbacks)
	}
	if !hasCallback(callbacks, cbMainMenu()) {
		t.Fatalf("missing main menu callback: %v", callbacks)
	}
}

func TestHandleCallbackQuery_TopLevelCheckNewShowsMangaList(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "Dragon Ball", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	query := &tgbotapi.CallbackQuery{
		ID:   "cb-top-level",
		Data: "check_new",
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: userID},
		},
	}
	b.handleCallbackQuery(query)

	msg := api.lastMessageConfig(t)
	if !strings.Contains(msg.Text, appcopy.Copy.Info.ListHeader) {
		t.Fatalf("expected list header in message, got %q", msg.Text)
	}
	callbacks := messageCallbacks(t, msg)
	if !hasCallback(callbacks, cbMangaAction(int(mangaID), "menu")) {
		t.Fatalf("expected manga selection callback, got %v", callbacks)
	}
	if hasCallback(callbacks, cbMangaAction(int(mangaID), "check_new")) {
		t.Fatalf("list should not include direct action callbacks, got %v", callbacks)
	}
}

func TestHandleCallbackQuery_TopLevelActionsRouteToMangaList(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("ecf8f75c-84ce-4d08-a0ea-42f359dd146a", "Boruto", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	actions := []string{"mark_read", "mark_unread", "sync_all", "remove_manga"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			query := &tgbotapi.CallbackQuery{
				ID:   "cb-top-level-" + action,
				Data: action,
				From: &tgbotapi.User{ID: userID},
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{ID: userID},
				},
			}
			b.handleCallbackQuery(query)

			msg := api.lastMessageConfig(t)
			if !strings.Contains(msg.Text, appcopy.Copy.Info.ListHeader) {
				t.Fatalf("expected list header in %q for action %q", msg.Text, action)
			}
			callbacks := messageCallbacks(t, msg)
			if !hasCallback(callbacks, cbMangaAction(int(mangaID), "menu")) {
				t.Fatalf("expected manga selection callback for action %q, got %v", action, callbacks)
			}
		})
	}
}

func TestAppendButtonsInRows_PerRowOneAndMany(t *testing.T) {
	btns := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("A", "a"),
		tgbotapi.NewInlineKeyboardButtonData("B", "b"),
		tgbotapi.NewInlineKeyboardButtonData("C", "c"),
	}

	rows := appendButtonsInRows(nil, btns, 1)
	if len(rows) != 3 || len(rows[0]) != 1 || len(rows[1]) != 1 || len(rows[2]) != 1 {
		t.Fatalf("perRow=1 rows shape mismatch: %#v", rows)
	}

	rows = appendButtonsInRows(nil, btns, 2)
	if len(rows) != 2 || len(rows[0]) != 2 || len(rows[1]) != 1 {
		t.Fatalf("perRow=2 rows shape mismatch: %#v", rows)
	}
}

func TestBreadcrumbLine_TrimsAndSkipsEmptyBuckets(t *testing.T) {
	got := breadcrumbLine(" Root ", " 1000-1999 ", " ", "", "1200-1299")
	if !strings.Contains(got, "Root > 1000-1999 > 1200-1299") {
		t.Fatalf("breadcrumbLine unexpected output: %q", got)
	}

	if !strings.Contains(unreadBreadcrumbLine("100-199"), "Unread") {
		t.Fatalf("unread breadcrumb root missing: %q", unreadBreadcrumbLine("100-199"))
	}
	if !strings.Contains(readBreadcrumbLine("100-199"), "Read") {
		t.Fatalf("read breadcrumb root missing: %q", readBreadcrumbLine("100-199"))
	}
}
