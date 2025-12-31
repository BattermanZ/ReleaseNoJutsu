package bot

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/updater"
)

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	b.logAction(message.From.ID, "Received message", message.Text)

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			b.sendMainMenu(message.Chat.ID)
		case "help":
			b.sendHelpMessage(message.Chat.ID)
		case "status":
			b.sendStatusMessage(message.Chat.ID)
		default:
			msg := tgbotapi.NewMessage(message.Chat.ID, "❓ Unknown command. Please use /start or /help.")
			if _, err := b.api.Send(msg); err != nil {
				logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", message.Chat.ID, err)
			}
		}
	} else if message.ReplyToMessage != nil && message.ReplyToMessage.Text != "" {
		b.handleReply(message)
	} else {
		text := strings.TrimSpace(message.Text)
		if b.looksLikeMangaDexID(text) {
			b.handleAddManga(message.Chat.ID, text)
			return
		}

		// Check if the message is a MangaDex URL.
		mangaID, err := b.mdClient.ExtractMangaIDFromURL(text)
		if err == nil {
			b.handleAddManga(message.Chat.ID, mangaID)
			return
		}

		msg := tgbotapi.NewMessage(message.Chat.ID, "I’m not sure what you mean. Use /start to see available options.")
		if _, err := b.api.Send(msg); err != nil {
			logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", message.Chat.ID, err)
		}
	}
}

func (b *Bot) handleReply(message *tgbotapi.Message) {
	b.logAction(message.From.ID, "Received reply", message.Text)

	replyTo := message.ReplyToMessage.Text
	replyText := strings.TrimSpace(message.Text)

	// Add manga flow (supports URL or raw UUID).
	if strings.Contains(replyTo, "Add a New Manga") || strings.Contains(replyTo, "MangaDex URL or ID") || strings.Contains(replyTo, "MangaDex ID") {
		if mangaID, err := b.mdClient.ExtractMangaIDFromURL(replyText); err == nil {
			b.handleAddManga(message.Chat.ID, mangaID)
			return
		}
		b.handleAddManga(message.Chat.ID, replyText)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "I didn’t understand that reply. Please use /start for options.")
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", message.Chat.ID, err)
	}
}

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	b.logAction(query.From.ID, "Received callback query", query.Data)

	parts := strings.Split(query.Data, ":")
	action := parts[0]

	switch action {
	case "add_manga":
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, `📚 *Add a New Manga*
Please send the MangaDex URL or ID of the manga you want to track.`)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: "MangaDex ID"}
		b.sendMessageWithMainMenuButton(msg)
	case "list_manga":
		b.handleListManga(query.Message.Chat.ID)
	case "check_new":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "check_new")
	case "mark_read":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "mark_read")
	case "list_read":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "list_read")
	case "sync_all":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "sync_all")
	case "select_manga": // This case is for manga selection menus (check_new, mark_read, list_read, remove_manga)
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for select_manga: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		nextAction := parts[2]
		b.handleMangaSelection(query.Message.Chat.ID, mangaID, nextAction)
	case "manga_action": // This case is for actions directly from the manga list
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for manga_action: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		action := parts[2]
		b.handleMangaSelection(query.Message.Chat.ID, mangaID, action)
	case "mark_chapter":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mark_chapter: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		chapterNumber := parts[2]
		b.handleMarkChapterAsRead(query.Message.Chat.ID, mangaID, chapterNumber)
	case "mr_pick":
		if len(parts) < 4 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_pick: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		scale, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting scale: %v", err)
			return
		}
		start, err := strconv.Atoi(parts[3])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting start: %v", err)
			return
		}
		switch scale {
		case 1000:
			b.sendMarkReadHundredsMenu(query.Message.Chat.ID, mangaID, start, false)
		case 100:
			b.sendMarkReadTensMenu(query.Message.Chat.ID, mangaID, start, false)
		case 10:
			b.sendMarkReadChaptersMenu(query.Message.Chat.ID, mangaID, start, false)
		default:
			logger.LogMsg(logger.LogError, "Unknown mr_pick scale: %d", scale)
		}
	case "mr_back_root":
		if len(parts) < 2 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_back_root: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		b.sendMarkReadStartMenu(query.Message.Chat.ID, mangaID)
	case "mr_back_hundreds":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_back_hundreds: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		hundredStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting hundredStart: %v", err)
			return
		}
		b.sendMarkReadHundredsMenu(query.Message.Chat.ID, mangaID, thousandBucketStart(hundredStart), false)
	case "mr_back_tens":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_back_tens: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		tenStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting tenStart: %v", err)
			return
		}
		b.sendMarkReadTensMenu(query.Message.Chat.ID, mangaID, hundredBucketStart(tenStart), false)
	case "unread_chapter":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for unread_chapter: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		chapterNumber := parts[2]
		b.handleMarkChapterAsUnread(query.Message.Chat.ID, mangaID, chapterNumber)
	case "remove_manga":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "remove_manga")
	case "main_menu":
		b.sendMainMenu(query.Message.Chat.ID)
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		logger.LogMsg(logger.LogError, "Error answering callback query: %v", err)
	}
}

func (b *Bot) sendMainMenu(chatID int64) {
	b.logAction(chatID, "Sent main menu", "")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📚 Add manga", "add_manga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 List followed manga", "list_manga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 Check for new chapters", "check_new"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Mark chapter as read", "mark_read"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Sync all chapters", "sync_all"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 List read chapters", "list_read"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Remove manga", "remove_manga"),
		),
	)

	welcomeMessage := `👋 *Welcome to ReleaseNoJutsu!*`
	msg := tgbotapi.NewMessage(chatID, welcomeMessage)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", chatID, err)
	}
}

func (b *Bot) sendMessageWithMainMenuButton(msg tgbotapi.MessageConfig) {
	mainMenuButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "main_menu"),
		),
	)

	if msg.ReplyMarkup != nil {
		if keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, mainMenuButton.InlineKeyboard...)
			msg.ReplyMarkup = keyboard
		} else {
			msg.ReplyMarkup = mainMenuButton
		}
	} else {
		msg.ReplyMarkup = mainMenuButton
	}

	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", msg.ChatID, err)
	}
}

func (b *Bot) sendHelpMessage(chatID int64) {
	b.logAction(chatID, "Sent help message", "")

	helpText := `ℹ️ *Help Information* 
Welcome to ReleaseNoJutsu!

*How it works:*
This bot helps you track your favorite manga series. It automatically checks for new chapters every 6 hours and notifies you when new releases are available. You can also manually check for updates, mark chapters as read, and view your reading progress.

*Commands:*
• /start - Return to the main menu
• /help - Show this help message
• /status - Show bot status/health

*Main Features:*
- *Add manga:* Start tracking a new manga by sending its MangaDex URL or ID.
- *List followed manga:* See which series you're currently tracking.
- *Check for new chapters:* Quickly see if any of your followed manga have fresh releases.
- *Mark chapter as read:* Update your progress so you know which chapters you've finished.
- *Sync all chapters:* Import the full chapter history from MangaDex for a manga (useful when starting from scratch).
- *List read chapters:* Review what you've read recently.
- *Remove manga:* Stop tracking a manga you no longer wish to follow.

*How to add a manga:*
Simply send the MangaDex URL (e.g., https://mangadex.org/title/123e4567-e89b-12d3-a456-426614174000) or the MangaDex ID (e.g., 123e4567-e89b-12d3-a456-426614174000) directly to the bot. The bot will automatically detect and add the manga.

If you need further assistance, feel free to /start and explore the menu options!`
	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendStatusMessage(chatID int64) {
	b.logAction(chatID, "Sent status", "")

	status, err := b.db.GetStatus()
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting status: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not retrieve status right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var bld strings.Builder
	bld.WriteString("<b>ReleaseNoJutsu Status</b>\n\n")
	bld.WriteString(fmt.Sprintf("Tracked manga: <b>%d</b>\n", status.MangaCount))
	bld.WriteString(fmt.Sprintf("Chapters stored: <b>%d</b>\n", status.ChapterCount))
	bld.WriteString(fmt.Sprintf("Registered chats: <b>%d</b>\n", status.UserCount))
	bld.WriteString(fmt.Sprintf("Total unread: <b>%d</b>\n", status.UnreadTotal))
	if status.HasCronLastRun {
		bld.WriteString(fmt.Sprintf("Scheduler last run: <b>%s</b>\n", status.CronLastRun.Local().Format(time.RFC1123)))
	} else {
		bld.WriteString("Scheduler last run: <b>never</b>\n")
	}
	bld.WriteString("\nUpdate interval: every 6 hours\n")

	msg := tgbotapi.NewMessage(chatID, bld.String())
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}

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
	messageBuilder.WriteString(`📚 *Your Followed Manga:*

`) // Changed to backticks
	count := 0
	for rows.Next() {
		var id int
		var mangadexID, title string
		var lastChecked string
		var lastSeenAt sql.NullTime
		var lastReadAt sql.NullTime
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked, &lastSeenAt, &lastReadAt)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error scanning manga row: %v", err)
			continue
		}
		count++

		// Manga title row
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d. %s", count, title), "ignore"), // "ignore" as a placeholder, no action on title click
		))

		// Action buttons row
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 Check New", fmt.Sprintf("manga_action:%d:check_new", id)),
			tgbotapi.NewInlineKeyboardButtonData("✅ Mark Read", fmt.Sprintf("manga_action:%d:mark_read", id)),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Remove", fmt.Sprintf("manga_action:%d:remove_manga", id)),
		))
	}

	if count == 0 {
		messageBuilder.WriteString("You’re not following any manga yet. Choose 'Add manga' to start tracking a series!")
	} else {
		messageBuilder.WriteString(fmt.Sprintf("Total: %d manga", count))
	}

	msg := tgbotapi.NewMessage(chatID, messageBuilder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMarkChapterAsRead(chatID int64, mangaID int, chapterNumber string) {
	b.logAction(chatID, "Mark chapter as read", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))

	err := b.db.MarkChapterAsRead(mangaID, chapterNumber)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error marking chapters as read: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not update the chapter status. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	result := fmt.Sprintf("✅ Updated!\nAll chapters up to Chapter %s of *%s* have been marked as read.", chapterNumber, mangaTitle)
	msg := tgbotapi.NewMessage(chatID, result)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMarkChapterAsUnread(chatID int64, mangaID int, chapterNumber string) {
	b.logAction(chatID, "Mark chapter as unread", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))

	err := b.db.MarkChapterAsUnread(mangaID, chapterNumber)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error marking chapter as unread: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not update the chapter status. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	result := fmt.Sprintf("✅ Chapter %s of *%s* is now marked as unread.", chapterNumber, mangaTitle)
	msg := tgbotapi.NewMessage(chatID, result)
	msg.ParseMode = "Markdown"
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
		var lastChecked string
		var lastSeenAt sql.NullTime
		var lastReadAt sql.NullTime
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked, &lastSeenAt, &lastReadAt)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error scanning manga row: %v", err)
			continue
		}
		callbackData := fmt.Sprintf("select_manga:%d:%s", id, nextAction)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(title, callbackData),
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
		messageText = `📖 *List Read Chapters*

Select a manga to see the chapters you’ve marked as read:`
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

func (b *Bot) sendMarkReadStartMenu(chatID int64, mangaID int) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)

	if unreadCount == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: 0\n\n✅ You're up to date.", mangaTitle, lastReadLine))
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	if unreadCount <= 10 {
		b.sendMarkReadDirectChaptersMenu(chatID, mangaID, unreadCount, mangaTitle, lastReadLine)
		return
	}

	thousands, err := b.db.ListUnreadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(thousands) > 1 {
		b.sendMarkReadThousandsMenu(chatID, mangaID, thousands, unreadCount, mangaTitle, lastReadLine)
		return
	}

	thousandStart := 1
	if len(thousands) == 1 {
		thousandStart = thousands[0]
	}
	rs, re := bucketRange(thousandStart, 1000)

	hundreds, err := b.db.ListUnreadBucketStarts(mangaID, 100, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(hundreds) > 1 {
		b.sendMarkReadHundredsMenu(chatID, mangaID, thousandStart, true)
		return
	}

	hundredStart := thousandStart
	if len(hundreds) == 1 {
		hundredStart = hundreds[0]
	}
	rs, re = bucketRange(hundredStart, 100)

	tens, err := b.db.ListUnreadBucketStarts(mangaID, 10, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(tens) > 1 {
		b.sendMarkReadTensMenu(chatID, mangaID, hundredStart, true)
		return
	}

	tenStart := hundredStart
	if len(tens) == 1 {
		tenStart = tens[0]
	}
	b.sendMarkReadChaptersMenu(chatID, mangaID, tenStart, true)
}

func (b *Bot) sendMarkReadDirectChaptersMenu(chatID int64, mangaID int, unreadCount int, mangaTitle, lastReadLine string) {
	chapters, err := b.db.ListUnreadChapters(mangaID, 10, 0)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, ch := range chapters {
		label := fmt.Sprintf("Ch. %s", ch.Number)
		if strings.TrimSpace(ch.Title) != "" {
			label = fmt.Sprintf("Ch. %s: %s", ch.Number, ch.Title)
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mark_chapter:%d:%s", mangaID, ch.Number)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\n\nSelect a chapter to mark it (and all previous ones) as read:", mangaTitle, lastReadLine, unreadCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadThousandsMenu(chatID int64, mangaID int, starts []int, unreadCount int, mangaTitle, lastReadLine string) {
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		label := bucketLabel(start, 1000)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 1000, start)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\n\nPick a range:", mangaTitle, lastReadLine, unreadCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadHundredsMenu(chatID int64, mangaID int, thousandStart int, root bool) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(thousandStart, 1000)

	starts, err := b.db.ListUnreadBucketStarts(mangaID, 100, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkReadTensMenu(chatID, mangaID, starts[0], root)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 100), fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 100, start)),
		})
	}
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mr_back_root:%d", mangaID)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, unreadCount, bucketLabel(thousandStart, 1000)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadTensMenu(chatID int64, mangaID int, hundredStart int, root bool) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(hundredStart, 100)

	starts, err := b.db.ListUnreadBucketStarts(mangaID, 10, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkReadChaptersMenu(chatID, mangaID, starts[0], root)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 10), fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 10, start)),
		})
	}
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mr_back_hundreds:%d:%d", mangaID, hundredStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, unreadCount, bucketLabel(hundredStart, 100)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadChaptersMenu(chatID int64, mangaID int, tenStart int, root bool) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(tenStart, 10)

	chapters, err := b.db.ListUnreadNumericChaptersInRange(mangaID, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, ch := range chapters {
		label := fmt.Sprintf("Ch. %s", ch.Number)
		if strings.TrimSpace(ch.Title) != "" {
			label = fmt.Sprintf("Ch. %s: %s", ch.Number, ch.Title)
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mark_chapter:%d:%s", mangaID, ch.Number)),
		})
	}
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mr_back_tens:%d:%d", mangaID, tenStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nSelect a chapter to mark it (and all previous ones) as read:", mangaTitle, lastReadLine, unreadCount, bucketLabel(tenStart, 10)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendReadChaptersMenu(chatID int64, mangaID int) {
	rows, err := b.db.GetReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying chapters: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for rows.Next() {
		var chapterNumber, ctitle string
		err := rows.Scan(&chapterNumber, &ctitle)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error scanning chapter row: %v", err)
			continue
		}
		callbackData := fmt.Sprintf("unread_chapter:%d:%s", mangaID, chapterNumber)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Ch. %s: %s", chapterNumber, ctitle), callbackData),
		})
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	messageText := fmt.Sprintf(`📚 *%s*

Below are some chapters you've read. Select one to mark as unread:`, mangaTitle)
	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleMangaSelection(chatID int64, mangaID int, nextAction string) {
	switch nextAction {
	case "check_new":
		b.handleCheckNewChapters(chatID, mangaID)
	case "mark_read":
		b.sendMarkReadStartMenu(chatID, mangaID)
	case "sync_all":
		b.handleSyncAllChapters(chatID, mangaID)
	case "list_read":
		b.sendReadChaptersMenu(chatID, mangaID)
	case "remove_manga":
		b.handleRemoveManga(chatID, mangaID)
	default:
		logger.LogMsg(logger.LogError, "Unknown next action: %s", nextAction)
	}
}

func (b *Bot) lastReadLine(mangaID int) string {
	lastReadNum, lastReadTitle, hasLastRead, err := b.db.GetLastReadChapter(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting last read chapter: %v", err)
	}
	if !hasLastRead {
		return "Last read: (none)"
	}
	if strings.TrimSpace(lastReadTitle) == "" {
		return fmt.Sprintf("Last read: Ch. %s", lastReadNum)
	}
	return fmt.Sprintf("Last read: Ch. %s — %s", lastReadNum, lastReadTitle)
}

func bucketLabel(start, bucketSize int) string {
	if start == 1 {
		return fmt.Sprintf("1-%d", bucketSize-1)
	}
	return fmt.Sprintf("%d-%d", start, start+bucketSize-1)
}

func bucketRange(start, bucketSize int) (float64, float64) {
	if start == 1 {
		return 1, float64(bucketSize)
	}
	return float64(start), float64(start + bucketSize)
}

func thousandBucketStart(n int) int {
	if n < 1000 {
		return 1
	}
	return (n / 1000) * 1000
}

func hundredBucketStart(n int) int {
	if n < 100 {
		return 1
	}
	return (n / 100) * 100
}

func (b *Bot) handleSyncAllChapters(chatID int64, mangaID int) {
	b.logAction(chatID, "Sync all chapters", fmt.Sprintf("Manga ID: %d", mangaID))

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	start := tgbotapi.NewMessage(chatID, fmt.Sprintf("🔄 Syncing all chapters for *%s* (this can take a bit)...", mangaTitle))
	start.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(start)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		synced, _, err := b.updater.SyncAll(ctx, mangaID)
		if err != nil {
			logger.LogMsg(logger.LogError, "SyncAll failed for manga %d: %v", mangaID, err)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Sync failed for *%s*.", mangaTitle))
			msg.ParseMode = "Markdown"
			b.sendMessageWithMainMenuButton(msg)
			return
		}

		unread, _ := b.db.CountUnreadChapters(mangaID)
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Sync complete for *%s*.\nImported/updated %d chapter entries.\nUnread chapters: %d.", mangaTitle, synced, unread))
		msg.ParseMode = "Markdown"
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
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ No new chapters for *%s*.", res.Title))
		msg.ParseMode = "Markdown"
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	message := updater.FormatNewChaptersMessageHTML(res.Title, res.NewChapters, res.UnreadCount)
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) looksLikeMangaDexID(text string) bool {
	// MangaDex IDs are UUIDs: 36 chars with 4 hyphens.
	text = strings.TrimSpace(text)
	return len(text) == 36 && strings.Count(text, "-") == 4
}

func (b *Bot) logAction(userID int64, action, details string) {
	logger.LogMsg(logger.LogInfo, "[User: %d] [%s] %s", userID, action, details)
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

	result := fmt.Sprintf("✅ *%s* has been successfully removed.", mangaTitle)
	msg := tgbotapi.NewMessage(chatID, result)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}
