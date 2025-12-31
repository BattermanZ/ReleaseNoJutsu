package bot

import (
	"database/sql"
	"fmt"
	"html"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) handleListManga(chatID int64) {
	b.logAction(chatID, "List manga", "")

	rows, err := b.db.GetAllManga()
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying manga: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Unable to retrieve your manga list. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	defer func() { _ = rows.Close() }()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var messageBuilder strings.Builder
	messageBuilder.WriteString("📚 <b>Your Followed Manga</b>\n\n")
	count := 0
	for rows.Next() {
		var id int
		var mangadexID, title string
		var isMangaPlus int
		var lastChecked string
		var lastSeenAt sql.NullTime
		var lastReadNumber sql.NullFloat64
		var unreadCount int
		err := rows.Scan(&id, &mangadexID, &title, &isMangaPlus, &lastChecked, &lastSeenAt, &lastReadNumber, &unreadCount)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error scanning manga row: %v", err)
			continue
		}
		count++

		displayTitle := title
		if isMangaPlus != 0 {
			displayTitle = "⭐ " + displayTitle
		}

		label := fmt.Sprintf("%d. %s", count, displayTitle)
		if unreadCount > 0 {
			label = fmt.Sprintf("%s (%d unread)", label, unreadCount)
		}
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("manga_action:%d:menu", id)),
		))
	}

	if count == 0 {
		messageBuilder.WriteString("You’re not following any manga yet. Choose “Add manga” to start tracking a series!")
	} else {
		messageBuilder.WriteString(fmt.Sprintf("Total: <b>%d</b>", count))
	}

	msg := tgbotapi.NewMessage(chatID, messageBuilder.String())
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMangaSelectionMenu(chatID int64, nextAction string) {
	rows, err := b.db.GetAllManga()
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying manga: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for rows.Next() {
		var id int
		var mangadexID, title string
		var isMangaPlus int
		var lastChecked string
		var lastSeenAt sql.NullTime
		var lastReadNumber sql.NullFloat64
		var unreadCount int
		err := rows.Scan(&id, &mangadexID, &title, &isMangaPlus, &lastChecked, &lastSeenAt, &lastReadNumber, &unreadCount)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error scanning manga row: %v", err)
			continue
		}
		displayTitle := title
		if isMangaPlus != 0 {
			displayTitle = "⭐ " + displayTitle
		}
		callbackData := fmt.Sprintf("select_manga:%d:%s", id, nextAction)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(displayTitle, callbackData),
		})
	}

	var messageText string
	switch nextAction {
	case "check_new":
		messageText = `🔍 *Check for New Chapters*

Select a manga to see if new chapters are available:`
	case "mark_read":
		messageText = `✅ *Mark Chapters as Read*

Select a manga to update your reading progress:`
	case "sync_all":
		messageText = `🔄 *Sync All Chapters*

	Select a manga to import its full chapter history from MangaDex:`
	case "list_read":
		messageText = `↩️ *Mark Chapter as Unread*

	Select a manga to move your progress back:`
	case "remove_manga":
		messageText = "🗑️ *Remove Manga*\n\nSelect a manga to stop tracking:"
	default:
		messageText = `📚 *Select a Manga*

Choose a manga to proceed.`
	}

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMangaSelection(chatID int64, mangaID int, nextAction string) {
	switch nextAction {
	case "menu":
		b.sendMangaActionMenu(chatID, mangaID)
	case "check_new":
		b.handleCheckNewChapters(chatID, mangaID)
	case "mark_read":
		b.sendMarkReadStartMenu(chatID, mangaID)
	case "mark_all_read":
		b.sendMarkAllReadConfirm(chatID, mangaID)
	case "mark_all_read_yes":
		b.handleMarkAllRead(chatID, mangaID)
	case "sync_all":
		b.handleSyncAllChapters(chatID, mangaID)
	case "list_read":
		b.sendMarkUnreadStartMenu(chatID, mangaID)
	case "details":
		b.handleMangaDetails(chatID, mangaID)
	case "toggle_plus":
		b.toggleMangaPlus(chatID, mangaID)
	case "remove_manga":
		b.sendRemoveMangaConfirm(chatID, mangaID)
	case "remove_manga_yes":
		b.handleRemoveManga(chatID, mangaID)
	default:
		logger.LogMsg(logger.LogError, "Unknown next action: %s", nextAction)
	}
}

func (b *Bot) sendMangaActionMenu(chatID int64, mangaID int) {
	title, err := b.db.GetMangaTitle(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load that manga right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	unread, _ := b.db.CountUnreadChapters(mangaID)

	detailsLine := b.lastReadLineHTML(mangaID)

	var bld strings.Builder
	bld.WriteString("📖 <b>")
	bld.WriteString(html.EscapeString(title))
	bld.WriteString("</b>\n\n")
	bld.WriteString(detailsLine)
	bld.WriteString("\n")
	bld.WriteString(fmt.Sprintf("Unread: <b>%d</b>\n\n", unread))
	bld.WriteString("Choose an action:")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 Check New", fmt.Sprintf("manga_action:%d:check_new", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Sync All", fmt.Sprintf("manga_action:%d:sync_all", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Mark Read", fmt.Sprintf("manga_action:%d:mark_read", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData("✅ Mark ALL Read", fmt.Sprintf("manga_action:%d:mark_all_read", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("↩️ Mark Unread", fmt.Sprintf("manga_action:%d:list_read", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Details", fmt.Sprintf("manga_action:%d:details", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData("⭐ Toggle MANGA Plus", fmt.Sprintf("manga_action:%d:toggle_plus", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Remove", fmt.Sprintf("manga_action:%d:remove_manga", mangaID)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, bld.String())
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendRemoveMangaConfirm(chatID int64, mangaID int) {
	title, err := b.db.GetMangaTitle(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title for removal: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not retrieve manga details for removal. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🗑️ Remove <b>%s</b>?\n\nThis will delete the manga and all stored chapters from your local database.", html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Yes, delete", fmt.Sprintf("manga_action:%d:remove_manga_yes", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", fmt.Sprintf("manga_action:%d:menu", mangaID)),
		),
	)
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkAllReadConfirm(chatID int64, mangaID int) {
	title, err := b.db.GetMangaTitle(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load that manga right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Mark <b>all chapters</b> as read for <b>%s</b>?\n\nThis will set your progress to the latest numeric chapter.", html.EscapeString(title)))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Yes", fmt.Sprintf("manga_action:%d:mark_all_read_yes", mangaID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", fmt.Sprintf("manga_action:%d:menu", mangaID)),
		),
	)
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMarkAllRead(chatID int64, mangaID int) {
	b.logAction(chatID, "Mark all chapters as read", fmt.Sprintf("Manga ID: %d", mangaID))

	if err := b.db.MarkAllChaptersAsRead(mangaID); err != nil {
		logger.LogMsg(logger.LogError, "Error marking all chapters as read: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not update your progress right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	title, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLineHTML(mangaID)
	unread, _ := b.db.CountUnreadChapters(mangaID)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Updated <b>%s</b>.\n\n%s\nUnread: <b>%d</b>", html.EscapeString(title), lastReadLine, unread))
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMangaDetails(chatID int64, mangaID int) {
	b.logAction(chatID, "Manga details", fmt.Sprintf("Manga ID: %d", mangaID))

	d, err := b.db.GetMangaDetails(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga details: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load manga details right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var bld strings.Builder
	bld.WriteString("<b>Manga Details</b>\n\n")
	bld.WriteString(fmt.Sprintf("Title: <b>%s</b>\n", html.EscapeString(d.Title)))
	bld.WriteString(fmt.Sprintf(`MangaDex: <a href="https://mangadex.org/title/%s">Open</a>`+"\n", html.EscapeString(d.MangaDexID)))
	if d.IsMangaPlus {
		bld.WriteString("MANGA Plus: <b>yes</b>\n")
	} else {
		bld.WriteString("MANGA Plus: <b>no</b>\n")
	}
	bld.WriteString(fmt.Sprintf("Chapters stored: <b>%d</b> (numeric: <b>%d</b>)\n", d.ChaptersTotal, d.NumericChaptersTotal))
	if d.HasMinNumber && d.HasMaxNumber {
		bld.WriteString(fmt.Sprintf("Numeric range: <b>%.1f</b> → <b>%.1f</b>\n", d.MinNumber, d.MaxNumber))
	}
	if d.HasLastReadNumber {
		bld.WriteString(fmt.Sprintf("Last read: <b>%.1f</b>\n", d.LastReadNumber))
	} else {
		bld.WriteString("Last read: <b>(none)</b>\n")
	}
	bld.WriteString(fmt.Sprintf("Unread: <b>%d</b>\n", d.UnreadCount))
	if d.HasLastSeenAt {
		bld.WriteString(fmt.Sprintf("Last seen at: <b>%s</b>\n", html.EscapeString(d.LastSeenAt.Local().Format(time.RFC1123))))
	}
	if d.HasLastChecked {
		bld.WriteString(fmt.Sprintf("Last checked: <b>%s</b>\n", html.EscapeString(d.LastChecked.Local().Format(time.RFC1123))))
	}
	bld.WriteString("\nNote: unread/read tracking is based on numeric chapter numbers; non-numeric extras are excluded from progress.")

	msg := tgbotapi.NewMessage(chatID, bld.String())
	msg.ParseMode = "HTML"
	toggleLabel := "⭐ Mark as MANGA Plus"
	if d.IsMangaPlus {
		toggleLabel = "⭐ Unmark MANGA Plus"
	}
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, fmt.Sprintf("manga_action:%d:toggle_plus", mangaID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Mark ALL Read", fmt.Sprintf("manga_action:%d:mark_all_read", mangaID)),
		),
	)
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) toggleMangaPlus(chatID int64, mangaID int) {
	cur, err := b.db.IsMangaPlus(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga plus flag: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not update MANGA Plus status right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	next := !cur
	if err := b.db.SetMangaPlus(mangaID, next); err != nil {
		logger.LogMsg(logger.LogError, "Error setting manga plus flag: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not update MANGA Plus status right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	title, _ := b.db.GetMangaTitle(mangaID)
	state := "disabled"
	if next {
		state = "enabled"
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ MANGA Plus is now <b>%s</b> for <b>%s</b>.", html.EscapeString(state), html.EscapeString(title)))
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleRemoveManga(chatID int64, mangaID int) {
	b.logAction(chatID, "Remove manga", fmt.Sprintf("Manga ID: %d", mangaID))

	mangaTitle, err := b.db.GetMangaTitle(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting manga title for removal: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not retrieve manga details for removal. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	err = b.db.DeleteManga(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error deleting manga: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Error removing the manga from the database. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ <b>%s</b> has been successfully removed.", html.EscapeString(mangaTitle)))
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}
