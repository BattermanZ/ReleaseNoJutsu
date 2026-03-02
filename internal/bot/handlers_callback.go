package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/logger"
)

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	b.logAction(query.From.ID, "Received callback query", query.Data)
	b.activeCallback = query.Message
	defer func() {
		b.activeCallback = nil
	}()

	payload, err := parseCallbackData(query.Data)
	if err != nil {
		logger.LogMsg(logger.LogError, "Invalid callback data: %s (%v)", query.Data, err)
		callback := tgbotapi.NewCallback(query.ID, "")
		if _, reqErr := b.api.Request(callback); reqErr != nil {
			logger.LogMsg(logger.LogError, "Error answering callback query: %v", reqErr)
		}
		return
	}

	// Leaving add mode via any callback should reset pending text-input flow.
	if payload.Kind != callbackAddManga {
		b.clearPendingState(query.From.ID)
	}

	switch payload.Kind {
	case callbackAddConfirm:
		b.confirmAddManga(query.Message.Chat.ID, query.From.ID, payload.MangaDexID, payload.IsMangaPlus)
	case callbackAddManga:
		if err := b.db.SetUserPendingState(query.From.ID, pendingStateAddManga, ""); err != nil {
			logger.LogMsg(logger.LogWarning, "Failed to set pending state for user %d: %v", query.From.ID, err)
		}
		b.sendAddMangaPrompt(query.Message.Chat.ID)
	case callbackListManga:
		b.handleListManga(query.Message.Chat.ID, query.From.ID)
	case callbackCheckNew, callbackMarkRead, callbackMarkUnread, callbackSyncAll, callbackRemoveManga:
		// UX: normalize to "manga first, action second" via the manga list.
		b.handleListManga(query.Message.Chat.ID, query.From.ID)
	case callbackGenPair:
		b.handleGeneratePairingCode(query.Message.Chat.ID, query.From.ID)
	case callbackMangaAction:
		b.handleMangaSelection(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.NextAction)
	case callbackMarkChapterRead:
		b.handleMarkChapterAsRead(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.ChapterNumber)
	case callbackMarkReadPick:
		switch payload.Scale {
		case 1000:
			b.sendMarkReadHundredsMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, false)
		case 100:
			b.sendMarkReadTensMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, false)
		case 10:
			b.sendMarkReadChaptersMenuPage(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, false, 0)
		default:
			logger.LogMsg(logger.LogError, "Unknown mr_pick scale: %d", payload.Scale)
		}
	case callbackMarkReadPage:
		b.sendMarkReadThousandsMenuPage(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Page)
	case callbackMarkReadChapterPage:
		b.sendMarkReadChaptersMenuPage(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, payload.Root, payload.Page)
	case callbackMarkReadBackRoot:
		b.sendMarkReadStartMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID)
	case callbackMarkReadBackHundreds:
		b.sendMarkReadHundredsMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, thousandBucketStart(payload.Start), false)
	case callbackMarkReadBackTens:
		b.sendMarkReadTensMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, hundredBucketStart(payload.Start), false)
	case callbackMarkUnreadPick:
		switch payload.Scale {
		case 1000:
			b.sendMarkUnreadHundredsMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, false)
		case 100:
			b.sendMarkUnreadTensMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, false)
		case 10:
			b.sendMarkUnreadChaptersMenuPage(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, false, 0)
		default:
			logger.LogMsg(logger.LogError, "Unknown mu_pick scale: %d", payload.Scale)
		}
	case callbackMarkUnreadPage:
		b.sendMarkUnreadThousandsMenuPage(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Page)
	case callbackMarkUnreadChapterPage:
		b.sendMarkUnreadChaptersMenuPage(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.Start, payload.Root, payload.Page)
	case callbackMarkUnreadBackRoot:
		b.sendMarkUnreadStartMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID)
	case callbackMarkUnreadBackHundreds:
		b.sendMarkUnreadHundredsMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, thousandBucketStart(payload.Start), false)
	case callbackMarkUnreadBackTens:
		b.sendMarkUnreadTensMenu(query.Message.Chat.ID, query.From.ID, payload.MangaID, hundredBucketStart(payload.Start), false)
	case callbackMarkChapterUnread:
		b.handleMarkChapterAsUnread(query.Message.Chat.ID, query.From.ID, payload.MangaID, payload.ChapterNumber)
	case callbackMainMenu:
		b.sendMainMenu(query.Message.Chat.ID)
	case callbackCancelPending:
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, appcopy.Copy.Prompts.AddMangaCancelled)
		b.sendMessageWithMainMenuButton(msg)
	default:
		logger.LogMsg(logger.LogError, "Unhandled callback kind: %d", payload.Kind)
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		logger.LogMsg(logger.LogError, "Error answering callback query: %v", err)
	}
}

func (b *Bot) clearPendingState(userID int64) {
	if err := b.db.ClearUserPendingState(userID); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed clearing pending state for %d: %v", userID, err)
	}
}
