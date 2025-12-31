package bot

import (
	"context"
	"fmt"
	"html"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/updater"
)

func (b *Bot) handleSyncAllChapters(chatID int64, mangaID int) {
	b.logAction(chatID, "Sync all chapters", fmt.Sprintf("Manga ID: %d", mangaID))

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	start := tgbotapi.NewMessage(chatID, fmt.Sprintf("🔄 Syncing all chapters for <b>%s</b> (this can take a bit)...", html.EscapeString(mangaTitle)))
	start.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(start)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		synced, _, err := b.updater.SyncAll(ctx, mangaID)
		if err != nil {
			logger.LogMsg(logger.LogError, "SyncAll failed for manga %d: %v", mangaID, err)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Sync failed for <b>%s</b>.", html.EscapeString(mangaTitle)))
			msg.ParseMode = "HTML"
			b.sendMessageWithMainMenuButton(msg)
			return
		}

		unread, _ := b.db.CountUnreadChapters(mangaID)
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Sync complete for <b>%s</b>.\nImported/updated %d chapter entries.\nUnread chapters: %d.", html.EscapeString(mangaTitle), synced, unread))
		msg.ParseMode = "HTML"
		b.sendMessageWithMainMenuButton(msg)
	}()
}

func (b *Bot) handleCheckNewChapters(chatID int64, mangaID int) {
	b.logAction(chatID, "Check new chapters", fmt.Sprintf("Manga ID: %d", mangaID))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := b.updater.UpdateOne(ctx, mangaID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Could not check MangaDex for updates right now. Please try again later.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	if len(res.NewChapters) == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ No new chapters for <b>%s</b>.", html.EscapeString(res.Title)))
		msg.ParseMode = "HTML"
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	isMangaPlus, err := b.db.IsMangaPlus(mangaID)
	if err != nil {
		isMangaPlus = false
	}
	message := updater.FormatNewChaptersMessageHTML(res.Title, res.NewChapters, res.UnreadCount, isMangaPlus)
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}
