package bot

import (
	"context"
	"fmt"
	"html"
	"strings"
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

	title = strings.TrimSpace(title)
	if title == "" {
		title = "Title not available"
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Yes (MANGA Plus)", fmt.Sprintf("add_confirm:%s:1", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ No", fmt.Sprintf("add_confirm:%s:0", mangaID)),
		),
	)
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📚 <b>%s</b>\n\nIs this a <b>MANGA Plus</b> manga?\n\nThis controls whether you get the “3+ unread chapters” warning.", html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) confirmAddManga(chatID int64, mangaDexID string, isMangaPlus bool) {
	b.logAction(chatID, "Confirm add manga", fmt.Sprintf("%s (MANGA Plus=%t)", mangaDexID, isMangaPlus))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mangaData, err := b.mdClient.GetManga(ctx, mangaDexID)
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

	mangaDBID, err := b.db.AddMangaWithMangaPlus(mangaDexID, title, isMangaPlus)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error inserting manga into database: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Error adding the manga to the database. It may already exist or the ID is invalid.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	// Full backfill so you can start from scratch (have the complete chapter list locally).
	mangaPlusLabel := "no"
	if isMangaPlus {
		mangaPlusLabel = "yes"
	}
	startMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Added <b>%s</b>.\nMANGA Plus: <b>%s</b>\n\n🔄 Now syncing all chapters from MangaDex (this can take a bit)...", html.EscapeString(title), html.EscapeString(mangaPlusLabel)))
	startMsg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(startMsg)

	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		synced, _, err := b.updater.SyncAll(syncCtx, int(mangaDBID))
		if err != nil {
			logger.LogMsg(logger.LogError, "Error syncing chapters for %s: %v", title, err)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Sync failed for <b>%s</b>.\n\nYou can try again from the main menu: “Sync all chapters”.", html.EscapeString(title)))
			msg.ParseMode = "HTML"
			b.sendMessageWithMainMenuButton(msg)
			return
		}

		unread, _ := b.db.CountUnreadChapters(int(mangaDBID))
		done := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Sync complete for <b>%s</b>.\nImported/updated %d chapter entries.\nUnread chapters: %d.\n\nUse “Mark chapter as read” to set your progress.", html.EscapeString(title), synced, unread))
		done.ParseMode = "HTML"
		b.sendMessageWithMainMenuButton(done)
	}()
}
