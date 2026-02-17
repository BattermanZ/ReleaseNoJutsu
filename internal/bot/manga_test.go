package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
)

func TestSendMangaActionMenu_UsesRemoveMangaLabel(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	mangaID, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "Dragon Ball", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	b.sendMangaActionMenu(userID, userID, int(mangaID))

	msg := api.lastMessageConfig(t)
	keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("ReplyMarkup type = %T, want InlineKeyboardMarkup", msg.ReplyMarkup)
	}

	hasRemoveManga := false
	hasLegacyConfirmLabel := false
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == appcopy.Copy.Buttons.RemoveManga {
				hasRemoveManga = true
			}
			if btn.Text == "✅ Yes, Remove" {
				hasLegacyConfirmLabel = true
			}
		}
	}

	if !hasRemoveManga {
		t.Fatalf("expected action menu to include %q button", appcopy.Copy.Buttons.RemoveManga)
	}
	if hasLegacyConfirmLabel {
		t.Fatalf("action menu should not include legacy confirmation label")
	}
}
