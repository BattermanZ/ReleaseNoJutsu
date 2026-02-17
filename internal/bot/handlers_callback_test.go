package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
