package bot

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/logger"
)

func (b *Bot) handleAddManga(chatID int64, userID int64, mangaID string) {
	b.logAction(chatID, "Add manga", mangaID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mangaData, err := b.mdClient.GetManga(ctx, mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error fetching manga data: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CouldNotRetrieveManga)
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
			title = appcopy.Copy.Prompts.TitleNotAvailable
		}
	}

	title = strings.TrimSpace(title)
	if title == "" {
		title = appcopy.Copy.Prompts.TitleNotAvailable
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.YesMangaPlus, fmt.Sprintf("add_confirm:%s:1", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.NoMangaPlus, fmt.Sprintf("add_confirm:%s:0", mangaID)),
		),
	)
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Prompts.MangaPlusQuestion, html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) confirmAddManga(chatID int64, userID int64, mangaDexID string, isMangaPlus bool) {
	b.logAction(chatID, "Confirm add manga", fmt.Sprintf("%s (MANGA Plus=%t)", mangaDexID, isMangaPlus))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mangaData, err := b.mdClient.GetManga(ctx, mangaDexID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error fetching manga data: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CouldNotRetrieveManga)
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
			title = appcopy.Copy.Prompts.TitleNotAvailable
		}
	}

	mangaDBID, err := b.db.AddMangaWithMangaPlus(mangaDexID, title, isMangaPlus, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error inserting manga into database: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CouldNotAddManga)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	// Full backfill so you can start from scratch (have the complete chapter list locally).
	mangaPlusLabel := appcopy.Copy.Info.MangaPlusNoLabel
	if isMangaPlus {
		mangaPlusLabel = appcopy.Copy.Info.MangaPlusYesLabel
	}
	startMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.SyncStartWithPlus, html.EscapeString(title), html.EscapeString(mangaPlusLabel)))
	startMsg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(startMsg)

	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		synced, _, err := b.updater.SyncAll(syncCtx, int(mangaDBID))
		if err != nil {
			logger.LogMsg(logger.LogError, "Error syncing chapters for %s: %v", title, err)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Errors.SyncFailed, html.EscapeString(title)))
			msg.ParseMode = "HTML"
			b.sendMessageWithMainMenuButton(msg)
			return
		}

		unread, _ := b.db.CountUnreadChapters(int(mangaDBID))
		done := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.SyncCompleteWithHint, html.EscapeString(title), synced, unread))
		done.ParseMode = "HTML"
		b.sendMessageWithMainMenuButton(done)
	}()
}
