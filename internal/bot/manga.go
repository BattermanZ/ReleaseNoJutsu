package bot

import (
	"database/sql"
	"fmt"
	"html"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/logger"
)

func (b *Bot) handleListManga(chatID int64, userID int64, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	b.logAction(chatID, "List manga", "")

	rows, err := b.db.GetAllMangaByUser(userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying manga: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadManga)
		b.sendMessageWithMainMenuButton(msg, cbTarget)
		return
	}
	defer func() { _ = rows.Close() }()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var messageBuilder strings.Builder
	messageBuilder.WriteString(appcopy.Copy.Info.ListHeader)
	count := 0
	for rows.Next() {
		var id int
		var rowUserID int64
		var mangadexID, title string
		var isMangaPlus int
		var lastChecked string
		var lastSeenAt sql.NullTime
		var lastReadNumber sql.NullFloat64
		var unreadCount int
		err := rows.Scan(&id, &rowUserID, &mangadexID, &title, &isMangaPlus, &lastChecked, &lastSeenAt, &lastReadNumber, &unreadCount)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error scanning manga row: %v", err)
			continue
		}
		count++

		displayTitle := title
		if isMangaPlus != 0 {
			displayTitle = appcopy.Copy.Labels.MangaPlusPrefix + displayTitle
		}

		label := fmt.Sprintf(appcopy.Copy.Labels.ListItemFormat, count, displayTitle)
		if unreadCount > 0 {
			label = label + fmt.Sprintf(appcopy.Copy.Labels.ListUnreadSuffix, unreadCount)
		}
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, cbMangaAction(id, "menu")),
		))
	}

	if count == 0 {
		messageBuilder.WriteString(appcopy.Copy.Info.ListEmpty)
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.AddManga, cbAddManga()),
		))
	} else {
		messageBuilder.WriteString(fmt.Sprintf(appcopy.Copy.Info.ListTotal, count))
	}

	msg := tgbotapi.NewMessage(chatID, messageBuilder.String())
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg, cbTarget)
}

func (b *Bot) handleMangaSelection(chatID int64, userID int64, mangaID int, nextAction string, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	allowed, err := b.db.MangaBelongsToUser(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error checking manga ownership: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotAccessManga)
		b.sendListScopedMessage(msg, cbTarget)
		return
	}
	if !allowed {
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.NoAccessToManga)
		b.sendListScopedMessage(msg, cbTarget)
		return
	}
	switch nextAction {
	case "menu":
		b.sendMangaActionMenu(chatID, userID, mangaID, cbTarget)
	case "check_new":
		b.handleCheckNewChapters(chatID, mangaID, cbTarget)
	case "mark_read":
		b.sendMarkReadStartMenu(chatID, userID, mangaID, cbTarget)
	case "mark_all_read":
		b.sendMarkAllReadConfirm(chatID, userID, mangaID, cbTarget)
	case "mark_all_read_yes":
		b.handleMarkAllRead(chatID, userID, mangaID, cbTarget)
	case "sync_all":
		b.handleSyncAllChapters(chatID, userID, mangaID, cbTarget)
	case "mark_unread", "list_read": // Keep legacy callback token for backward compatibility.
		b.sendMarkUnreadStartMenu(chatID, userID, mangaID, cbTarget)
	case "details":
		b.handleMangaDetails(chatID, userID, mangaID, cbTarget)
	case "toggle_plus":
		b.toggleMangaPlus(chatID, userID, mangaID, cbTarget)
	case "remove_manga":
		b.sendRemoveMangaConfirm(chatID, userID, mangaID, cbTarget)
	case "remove_manga_yes":
		b.handleRemoveManga(chatID, userID, mangaID, cbTarget)
	default:
		logger.LogMsg(logger.LogError, "Unknown next action: %s", nextAction)
	}
}

func (b *Bot) sendMangaActionMenu(chatID int64, userID int64, mangaID int, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	title, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadManga)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}
	unread, _ := b.db.CountUnreadChapters(mangaID)

	detailsLine := b.lastReadLineHTML(mangaID)

	var bld strings.Builder
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.ActionMenuHeader, html.EscapeString(title)))
	bld.WriteString(detailsLine)
	bld.WriteString("\n")
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.ActionMenuUnread, unread))
	bld.WriteString(appcopy.Copy.Info.ActionMenuPrompt)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.CheckNewShort, cbMangaAction(mangaID, "check_new")),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.SyncAllShort, cbMangaAction(mangaID, "sync_all")),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkReadShort, cbMangaAction(mangaID, "mark_read")),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkAllRead, cbMangaAction(mangaID, "mark_all_read")),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkUnreadShort, cbMangaAction(mangaID, "mark_unread")),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.Details, cbMangaAction(mangaID, "details")),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.ToggleMangaPlus, cbMangaAction(mangaID, "toggle_plus")),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.RemoveManga, cbMangaAction(mangaID, "remove_manga")),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.BackToList, cbListManga()),
		),
	)

	msg := tgbotapi.NewMessage(chatID, bld.String())
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.sendMessageWithMainMenuButton(msg, cbTarget)
}

func (b *Bot) sendRemoveMangaConfirm(chatID int64, userID int64, mangaID int, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	title, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title for removal: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRetrieveManga)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Prompts.ConfirmDelete, html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.YesDelete, cbMangaAction(mangaID, "remove_manga_yes")),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.Cancel, cbMangaAction(mangaID, "menu")),
		),
	)
	b.sendMessageWithMainMenuButton(msg, cbTarget)
}

func (b *Bot) sendMarkAllReadConfirm(chatID int64, userID int64, mangaID int, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	title, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadManga)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Prompts.ConfirmMarkAllRead, html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.YesConfirm, cbMangaAction(mangaID, "mark_all_read_yes")),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.Cancel, cbMangaAction(mangaID, "menu")),
		),
	)
	b.sendMessageWithMainMenuButton(msg, cbTarget)
}

func (b *Bot) handleMarkAllRead(chatID int64, userID int64, mangaID int, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	b.logAction(chatID, "Mark all chapters as read", fmt.Sprintf("Manga ID: %d", mangaID))

	if err := b.db.MarkAllChaptersAsRead(mangaID); err != nil {
		logger.LogMsg(logger.LogError, "Error marking all chapters as read: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotUpdateProgress)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}

	title, _ := b.db.GetMangaTitle(mangaID, userID)
	lastReadLine := b.lastReadLineHTML(mangaID)
	unread, _ := b.db.CountUnreadChapters(mangaID)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.MarkAllReadDone, html.EscapeString(title), lastReadLine, unread))
	msg.ParseMode = "HTML"
	b.sendMangaScopedMessage(msg, mangaID, cbTarget)
}

func (b *Bot) handleMangaDetails(chatID int64, userID int64, mangaID int, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	b.logAction(chatID, "Manga details", fmt.Sprintf("Manga ID: %d", mangaID))

	d, err := b.db.GetMangaDetails(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga details: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadMangaDetails)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}

	var bld strings.Builder
	bld.WriteString(appcopy.Copy.Info.MangaDetails)
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsTitleLine, html.EscapeString(d.Title)))
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsMangaDexLine, html.EscapeString(d.MangaDexID)))
	if d.IsMangaPlus {
		bld.WriteString(appcopy.Copy.Info.MangaPlusYes)
	} else {
		bld.WriteString(appcopy.Copy.Info.MangaPlusNo)
	}
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsChaptersLine, d.ChaptersTotal, d.NumericChaptersTotal))
	if d.HasMinNumber && d.HasMaxNumber {
		bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsRangeLine, d.MinNumber, d.MaxNumber))
	}
	if d.HasLastReadNumber {
		bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsLastReadLine, d.LastReadNumber))
	} else {
		bld.WriteString(appcopy.Copy.Info.DetailsLastReadNoneLine)
	}
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsUnreadLine, d.UnreadCount))
	if d.HasLastSeenAt {
		bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsLastSeenLine, html.EscapeString(d.LastSeenAt.Local().Format(time.RFC1123))))
	}
	if d.HasLastChecked {
		bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.DetailsLastCheckedLine, html.EscapeString(d.LastChecked.Local().Format(time.RFC1123))))
	}
	bld.WriteString(appcopy.Copy.Info.DetailsNote)

	msg := tgbotapi.NewMessage(chatID, bld.String())
	msg.ParseMode = "HTML"
	toggleLabel := appcopy.Copy.Buttons.ToggleMangaPlus
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, cbMangaAction(mangaID, "toggle_plus")),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkAllRead, cbMangaAction(mangaID, "mark_all_read")),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.BackToManga, cbMangaAction(mangaID, "menu")),
		),
	)
	b.sendMessageWithMainMenuButton(msg, cbTarget)
}

func (b *Bot) toggleMangaPlus(chatID int64, userID int64, mangaID int, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	cur, err := b.db.IsMangaPlus(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga plus flag: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotUpdateMangaPlus)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}
	next := !cur
	if err := b.db.SetMangaPlus(mangaID, next); err != nil {
		logger.LogMsg(logger.LogError, "Error setting manga plus flag: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotUpdateMangaPlus)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}

	title, _ := b.db.GetMangaTitle(mangaID, userID)
	state := appcopy.Copy.Info.MangaPlusDisabled
	if next {
		state = appcopy.Copy.Info.MangaPlusEnabled
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.MangaPlusStatus, html.EscapeString(state), html.EscapeString(title)))
	msg.ParseMode = "HTML"
	b.sendMangaScopedMessage(msg, mangaID, cbTarget)
}

func (b *Bot) handleRemoveManga(chatID int64, userID int64, mangaID int, target ...*callbackEditTarget) {
	cbTarget := firstCallbackTarget(target...)
	b.logAction(chatID, "Remove manga", fmt.Sprintf("Manga ID: %d", mangaID))

	mangaTitle, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title for removal: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRetrieveManga)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}

	err = b.db.DeleteManga(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error deleting manga: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRemoveManga)
		b.sendMangaScopedMessage(msg, mangaID, cbTarget)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.MangaRemoved, html.EscapeString(mangaTitle)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.BackToList, cbListManga()),
		),
	)
	b.sendMessageWithMainMenuButton(msg, cbTarget)
}
