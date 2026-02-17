package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestHandleCallbackQuery_TopLevelNavigationClearsPendingState(t *testing.T) {
	b, database, _ := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	if err := database.SetUserPendingState(userID, pendingStateAddManga, ""); err != nil {
		t.Fatalf("SetUserPendingState(): %v", err)
	}

	query := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		Data: "mark_read",
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
	if hasState || state != "" || payload != "" {
		t.Fatalf("pending state not cleared: state=%q payload=%q has=%v", state, payload, hasState)
	}
}
