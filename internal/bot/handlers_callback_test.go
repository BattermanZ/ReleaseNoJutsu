package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
)

func TestHandleCallbackQuery_NavigationClearsPendingState(t *testing.T) {
	b, database, _ := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	tests := []struct {
		name           string
		callbackData   string
		wantStateAfter string
		wantHasState   bool
	}{
		{name: "top-level", callbackData: "mark_read", wantStateAfter: "", wantHasState: false},
		{name: "manga-action", callbackData: "manga_action:1:menu", wantStateAfter: "", wantHasState: false},
		{name: "legacy-select", callbackData: "select_manga:1:menu", wantStateAfter: "", wantHasState: false},
		{name: "add-mode", callbackData: "add_manga", wantStateAfter: pendingStateAddManga, wantHasState: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := database.SetUserPendingState(userID, pendingStateAddManga, ""); err != nil {
				t.Fatalf("SetUserPendingState(): %v", err)
			}

			query := &tgbotapi.CallbackQuery{
				ID:   "cb1",
				Data: tc.callbackData,
				From: &tgbotapi.User{ID: userID},
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{ID: userID},
				},
			}
			b.handleCallbackQuery(query)

			state, payload, hasState, err := database.GetUserPendingState(userID)
			if err != nil {
				t.Fatalf("GetUserPendingState(): %v", err)
			}
			if hasState != tc.wantHasState || state != tc.wantStateAfter {
				t.Fatalf("state mismatch: got state=%q payload=%q has=%v, want state=%q has=%v", state, payload, hasState, tc.wantStateAfter, tc.wantHasState)
			}
		})
	}
}

func TestHandleCallbackQuery_CancelPendingSendsMessageAndClearsState(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	if err := database.SetUserPendingState(userID, pendingStateAddManga, ""); err != nil {
		t.Fatalf("SetUserPendingState(): %v", err)
	}

	b.handleCallbackQuery(&tgbotapi.CallbackQuery{
		ID:   "cb-cancel",
		Data: cbCancelPending(),
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: userID},
		},
	})

	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.AddMangaCancelled {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.AddMangaCancelled)
	}
	state, payload, hasState, err := database.GetUserPendingState(userID)
	if err != nil {
		t.Fatalf("GetUserPendingState(): %v", err)
	}
	if hasState || state != "" || payload != "" {
		t.Fatalf("pending state not cleared: state=%q payload=%q has=%v", state, payload, hasState)
	}
}

func TestHandleCallbackQuery_MainMenu(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	b.handleCallbackQuery(&tgbotapi.CallbackQuery{
		ID:   "cb-main",
		Data: cbMainMenu(),
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			MessageID: 100,
			Chat: &tgbotapi.Chat{ID: userID},
		},
	})

	if got := api.lastMessageText(t); got != appcopy.Copy.Info.WelcomeTitle {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Info.WelcomeTitle)
	}
	if got := len(api.sent); got != 0 {
		t.Fatalf("expected callback flow to edit existing message, sends=%d", got)
	}
}

func TestHandleCallbackQuery_EditFailureFallsBackToSend(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	api.failEditRequests = true
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	b.handleCallbackQuery(&tgbotapi.CallbackQuery{
		ID:   "cb-main-fallback",
		Data: cbMainMenu(),
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			MessageID: 101,
			Chat:      &tgbotapi.Chat{ID: userID},
		},
	})

	if got := api.lastMessageText(t); got != appcopy.Copy.Info.WelcomeTitle {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Info.WelcomeTitle)
	}
	if got := len(api.sent); got == 0 {
		t.Fatal("expected send fallback after edit failure")
	}
}

func TestHandleCallbackQuery_InvalidPayloadReturnsWithoutSend(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)
	userID := int64(42)

	b.handleCallbackQuery(&tgbotapi.CallbackQuery{
		ID:   "cb-invalid",
		Data: "not_a_real_callback_payload",
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: userID},
		},
	})

	if got := len(api.sentMessageTexts(t)); got != 0 {
		t.Fatalf("expected no messages for invalid callback parse, got %d", got)
	}
}
