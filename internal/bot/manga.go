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

func (b *Bot) handleListManga(chatID int64, userID int64) {
	b.logAction(chatID, "List manga", "")

	rows, err := b.db.GetAllMangaByUser(userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying manga: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadManga)
		b.sendMessageWithMainMenuButton(msg)
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
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("manga_action:%d:menu", id)),
		))
	}

	if count == 0 {
		messageBuilder.WriteString(appcopy.Copy.Info.ListEmpty)
	} else {
		messageBuilder.WriteString(fmt.Sprintf(appcopy.Copy.Info.ListTotal, count))
	}

	msg := tgbotapi.NewMessage(chatID, messageBuilder.String())
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMangaSelectionMenu(chatID int64, userID int64, nextAction string) {
	rows, err := b.db.GetAllMangaByUser(userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying manga: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	var keyboard [][]tgbotapi.InlineKeyboardButton
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
		displayTitle := title
		if isMangaPlus != 0 {
			displayTitle = appcopy.Copy.Labels.MangaPlusPrefix + displayTitle
		}
		callbackData := fmt.Sprintf("select_manga:%d:%s", id, nextAction)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(displayTitle, callbackData),
		})
	}

	var messageText string
	switch nextAction {
	case "check_new":
		messageText = appcopy.Copy.Menus.CheckNewTitle
	case "mark_read":
		messageText = appcopy.Copy.Menus.MarkReadTitle
	case "sync_all":
		messageText = appcopy.Copy.Menus.SyncAllTitle
	case "list_read":
		messageText = appcopy.Copy.Menus.MarkUnreadTitle
	case "remove_manga":
		messageText = appcopy.Copy.Menus.RemoveTitle
	default:
		messageText = appcopy.Copy.Menus.SelectManga
	}

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMangaSelection(chatID int64, userID int64, mangaID int, nextAction string) {
	allowed, err := b.db.MangaBelongsToUser(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error checking manga ownership: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotAccessManga)
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if !allowed {
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.NoAccessToManga)
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	switch nextAction {
	case "menu":
		b.sendMangaActionMenu(chatID, userID, mangaID)
	case "check_new":
		b.handleCheckNewChapters(chatID, mangaID)
	case "mark_read":
		b.sendMarkReadStartMenu(chatID, userID, mangaID)
	case "mark_all_read":
		b.sendMarkAllReadConfirm(chatID, userID, mangaID)
	case "mark_all_read_yes":
		b.handleMarkAllRead(chatID, userID, mangaID)
	case "sync_all":
		b.handleSyncAllChapters(chatID, userID, mangaID)
	case "list_read":
		b.sendMarkUnreadStartMenu(chatID, userID, mangaID)
	case "details":
		b.handleMangaDetails(chatID, userID, mangaID)
	case "toggle_plus":
		b.toggleMangaPlus(chatID, userID, mangaID)
	case "remove_manga":
		b.sendRemoveMangaConfirm(chatID, userID, mangaID)
	case "remove_manga_yes":
		b.handleRemoveManga(chatID, userID, mangaID)
	default:
		logger.LogMsg(logger.LogError, "Unknown next action: %s", nextAction)
	}
}

func (b *Bot) sendMangaActionMenu(chatID int64, userID int64, mangaID int) {
	title, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadManga)
		b.sendMessageWithMainMenuButton(msg)
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
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.CheckNewShort, fmt.Sprintf("manga_action:%d:check_new", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.SyncAllShort, fmt.Sprintf("manga_action:%d:sync_all", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkReadShort, fmt.Sprintf("manga_action:%d:mark_read", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkAllRead, fmt.Sprintf("manga_action:%d:mark_all_read", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkUnreadShort, fmt.Sprintf("manga_action:%d:list_read", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.Details, fmt.Sprintf("manga_action:%d:details", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.ToggleMangaPlus, fmt.Sprintf("manga_action:%d:toggle_plus", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.RemoveConfirm, fmt.Sprintf("manga_action:%d:remove_manga", mangaID)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, bld.String())
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendRemoveMangaConfirm(chatID int64, userID int64, mangaID int) {
	title, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title for removal: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRetrieveManga)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Prompts.ConfirmDelete, html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.YesDelete, fmt.Sprintf("manga_action:%d:remove_manga_yes", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.Cancel, fmt.Sprintf("manga_action:%d:menu", mangaID)),
		),
	)
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkAllReadConfirm(chatID int64, userID int64, mangaID int) {
	title, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadManga)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Prompts.ConfirmMarkAllRead, html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.YesConfirm, fmt.Sprintf("manga_action:%d:mark_all_read_yes", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.Cancel, fmt.Sprintf("manga_action:%d:menu", mangaID)),
		),
	)
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMarkAllRead(chatID int64, userID int64, mangaID int) {
	b.logAction(chatID, "Mark all chapters as read", fmt.Sprintf("Manga ID: %d", mangaID))

	if err := b.db.MarkAllChaptersAsRead(mangaID); err != nil {
		logger.LogMsg(logger.LogError, "Error marking all chapters as read: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotUpdateProgress)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	title, _ := b.db.GetMangaTitle(mangaID, userID)
	lastReadLine := b.lastReadLineHTML(mangaID)
	unread, _ := b.db.CountUnreadChapters(mangaID)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.MarkAllReadDone, html.EscapeString(title), lastReadLine, unread))
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMangaDetails(chatID int64, userID int64, mangaID int) {
	b.logAction(chatID, "Manga details", fmt.Sprintf("Manga ID: %d", mangaID))

	d, err := b.db.GetMangaDetails(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga details: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.CannotLoadMangaDetails)
		b.sendMessageWithMainMenuButton(msg)
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
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, fmt.Sprintf("manga_action:%d:toggle_plus", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkAllRead, fmt.Sprintf("manga_action:%d:mark_all_read", mangaID)),
		),
	)
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) toggleMangaPlus(chatID int64, userID int64, mangaID int) {
	cur, err := b.db.IsMangaPlus(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga plus flag: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotUpdateMangaPlus)
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	next := !cur
	if err := b.db.SetMangaPlus(mangaID, next); err != nil {
		logger.LogMsg(logger.LogError, "Error setting manga plus flag: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotUpdateMangaPlus)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	title, _ := b.db.GetMangaTitle(mangaID, userID)
	state := appcopy.Copy.Info.MangaPlusDisabled
	if next {
		state = appcopy.Copy.Info.MangaPlusEnabled
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.MangaPlusStatus, html.EscapeString(state), html.EscapeString(title)))
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleRemoveManga(chatID int64, userID int64, mangaID int) {
	b.logAction(chatID, "Remove manga", fmt.Sprintf("Manga ID: %d", mangaID))

	mangaTitle, err := b.db.GetMangaTitle(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title for removal: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRetrieveManga)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	err = b.db.DeleteManga(mangaID, userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error deleting manga: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRemoveManga)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Info.MangaRemoved, html.EscapeString(mangaTitle)))
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}
