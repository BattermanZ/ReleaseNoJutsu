package bot

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) handleAddManga(chatID int64, mangaID string) {
	b.logAction(chatID, "Add manga", mangaID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mangaData, err := b.mdClient.GetManga(ctx, mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error fetching manga data: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not retrieve manga data. Please check the ID and try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var title string
	if enTitle, ok := mangaData.Data.Attributes.Title["en"]; ok && enTitle != "" {
		title = enTitle
	} else {
		for _, val := range mangaData.Data.Attributes.Title {
			if val != "" {
				title = val
				break
			}
		}
		if title == "" {
			title = "Title not available"
		}
	}

	mangaDBID, err := b.db.AddManga(mangaID, title)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error inserting manga into database: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Error adding the manga to the database. It may already exist or the ID is invalid.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	// Full backfill so you can start from scratch (have the complete chapter list locally).
	startMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Added *%s*.\n\n🔄 Now syncing all chapters from MangaDex (this can take a bit)...", title))
	startMsg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(startMsg)

	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		synced, _, err := b.updater.SyncAll(syncCtx, int(mangaDBID))
		if err != nil {
			logger.LogMsg(logger.LogError, "Error syncing chapters for %s: %v", title, err)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Sync failed for *%s*.\n\nYou can try again from the main menu: “Sync all chapters”.", title))
			msg.ParseMode = "Markdown"
			b.sendMessageWithMainMenuButton(msg)
			return
		}

		unread, _ := b.db.CountUnreadChapters(int(mangaDBID))
		done := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Sync complete for *%s*.\nImported/updated %d chapter entries.\nUnread chapters: %d.\n\nUse “Mark chapter as read” to set your progress.", title, synced, unread))
		done.ParseMode = "Markdown"
		b.sendMessageWithMainMenuButton(done)
	}()
}
