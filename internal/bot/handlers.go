package bot

import (
	"context"
	"database/sql"
	"fmt"
	"html"
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
			b.sendMarkReadChaptersMenuPage(query.Message.Chat.ID, mangaID, start, false, 0)
		default:
			logger.LogMsg(logger.LogError, "Unknown mr_pick scale: %d", scale)
		}
	case "mr_page":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_page: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkReadThousandsMenuPage(query.Message.Chat.ID, mangaID, page)
	case "mr_chpage":
		if len(parts) < 5 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_chpage: %s", query.Data)
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
		rootInt, err := strconv.Atoi(parts[3])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting root flag: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[4])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkReadChaptersMenuPage(query.Message.Chat.ID, mangaID, tenStart, intToBool(rootInt), page)
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
	case "mu_pick":
		if len(parts) < 4 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_pick: %s", query.Data)
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
			b.sendMarkUnreadHundredsMenu(query.Message.Chat.ID, mangaID, start, false)
		case 100:
			b.sendMarkUnreadTensMenu(query.Message.Chat.ID, mangaID, start, false)
		case 10:
			b.sendMarkUnreadChaptersMenuPage(query.Message.Chat.ID, mangaID, start, false, 0)
		default:
			logger.LogMsg(logger.LogError, "Unknown mu_pick scale: %d", scale)
		}
	case "mu_page":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_page: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkUnreadThousandsMenuPage(query.Message.Chat.ID, mangaID, page)
	case "mu_chpage":
		if len(parts) < 5 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_chpage: %s", query.Data)
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
		rootInt, err := strconv.Atoi(parts[3])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting root flag: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[4])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkUnreadChaptersMenuPage(query.Message.Chat.ID, mangaID, tenStart, intToBool(rootInt), page)
	case "mu_back_root":
		if len(parts) < 2 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_back_root: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		b.sendMarkUnreadStartMenu(query.Message.Chat.ID, mangaID)
	case "mu_back_hundreds":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_back_hundreds: %s", query.Data)
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
		b.sendMarkUnreadHundredsMenu(query.Message.Chat.ID, mangaID, thousandBucketStart(hundredStart), false)
	case "mu_back_tens":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_back_tens: %s", query.Data)
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
		b.sendMarkUnreadTensMenu(query.Message.Chat.ID, mangaID, hundredBucketStart(tenStart), false)
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
			tgbotapi.NewInlineKeyboardButtonData("↩️ Mark chapter as unread", "list_read"),
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
	- *Mark chapter as unread:* Move your progress back to a selected chapter.
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
		var lastReadNumber sql.NullFloat64
		var unreadCount int
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked, &lastSeenAt, &lastReadNumber, &unreadCount)
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
			tgbotapi.NewInlineKeyboardButtonData("↩️ Mark Unread", fmt.Sprintf("manga_action:%d:list_read", id)),
		))

		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Details", fmt.Sprintf("manga_action:%d:details", id)),
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
		var lastReadNumber sql.NullFloat64
		var unreadCount int
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked, &lastSeenAt, &lastReadNumber, &unreadCount)
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
		b.sendMarkReadThousandsMenu(chatID, mangaID, thousands, unreadCount, mangaTitle, lastReadLine, 0)
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
	b.sendMarkReadChaptersMenuPage(chatID, mangaID, tenStart, true, 0)
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

func (b *Bot) sendMarkReadThousandsMenuPage(chatID int64, mangaID int, page int) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	starts, err := b.db.ListUnreadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	b.sendMarkReadThousandsMenu(chatID, mangaID, starts, unreadCount, mangaTitle, lastReadLine, page)
}

func (b *Bot) sendMarkReadThousandsMenu(chatID int64, mangaID int, starts []int, unreadCount int, mangaTitle, lastReadLine string, page int) {
	const pageSize = 24
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if len(starts) > 0 {
		maxPage = (len(starts) - 1) / pageSize
	}
	if page > maxPage {
		page = maxPage
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(starts) {
		end = len(starts)
	}

	buttons := make([]tgbotapi.InlineKeyboardButton, 0, end-start)
	for _, bucketStart := range starts[start:end] {
		label := bucketLabel(bucketStart, 1000)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 1000, bucketStart)))
	}
	keyboard := appendButtonsInRows(nil, buttons, 2)

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mr_page:%d:%d", mangaID, page-1)))
	}
	if page < maxPage {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mr_page:%d:%d", mangaID, page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
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
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 100), fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 100, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
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
		b.sendMarkReadChaptersMenuPage(chatID, mangaID, starts[0], root, 0)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 10), fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 10, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mr_back_hundreds:%d:%d", mangaID, hundredStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, unreadCount, bucketLabel(hundredStart, 100)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadChaptersMenuPage(chatID int64, mangaID int, tenStart int, root bool, page int) {
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

	totalInRange, err := b.db.CountUnreadNumericChaptersInRange(mangaID, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	const pageSize = 30
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if totalInRange > 0 {
		maxPage = (totalInRange - 1) / pageSize
	}
	if page > maxPage {
		page = maxPage
	}
	chapters, err := b.db.ListUnreadNumericChaptersInRange(mangaID, rangeStart, rangeEnd, pageSize, page*pageSize)
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

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mr_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page-1)))
	}
	if (page+1)*pageSize < totalInRange {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mr_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
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

func (b *Bot) sendMarkUnreadStartMenu(chatID int64, mangaID int) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)

	if readCount == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: 0\n\nNothing to mark unread yet.", mangaTitle, lastReadLine))
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	if readCount <= 10 {
		b.sendMarkUnreadDirectChaptersMenu(chatID, mangaID, readCount, mangaTitle, lastReadLine)
		return
	}

	thousands, err := b.db.ListReadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(thousands) > 1 {
		b.sendMarkUnreadThousandsMenu(chatID, mangaID, thousands, readCount, mangaTitle, lastReadLine, 0)
		return
	}

	thousandStart := 1
	if len(thousands) == 1 {
		thousandStart = thousands[0]
	}
	rs, re := bucketRange(thousandStart, 1000)

	hundreds, err := b.db.ListReadBucketStarts(mangaID, 100, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(hundreds) > 1 {
		b.sendMarkUnreadHundredsMenu(chatID, mangaID, thousandStart, true)
		return
	}

	hundredStart := thousandStart
	if len(hundreds) == 1 {
		hundredStart = hundreds[0]
	}
	rs, re = bucketRange(hundredStart, 100)

	tens, err := b.db.ListReadBucketStarts(mangaID, 10, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(tens) > 1 {
		b.sendMarkUnreadTensMenu(chatID, mangaID, hundredStart, true)
		return
	}

	tenStart := hundredStart
	if len(tens) == 1 {
		tenStart = tens[0]
	}
	b.sendMarkUnreadChaptersMenuPage(chatID, mangaID, tenStart, true, 0)
}

func (b *Bot) sendMarkUnreadDirectChaptersMenu(chatID int64, mangaID int, readCount int, mangaTitle, lastReadLine string) {
	chapters, err := b.db.ListReadNumericChaptersInRange(mangaID, 1, 1.0e18, 10, 0)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
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
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("unread_chapter:%d:%s", mangaID, ch.Number)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\n\nSelect a chapter to mark it (and all following ones) as unread:", mangaTitle, lastReadLine, readCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadThousandsMenuPage(chatID int64, mangaID int, page int) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	starts, err := b.db.ListReadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	b.sendMarkUnreadThousandsMenu(chatID, mangaID, starts, readCount, mangaTitle, lastReadLine, page)
}

func (b *Bot) sendMarkUnreadThousandsMenu(chatID int64, mangaID int, starts []int, readCount int, mangaTitle, lastReadLine string, page int) {
	const pageSize = 24
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if len(starts) > 0 {
		maxPage = (len(starts) - 1) / pageSize
	}
	if page > maxPage {
		page = maxPage
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(starts) {
		end = len(starts)
	}

	buttons := make([]tgbotapi.InlineKeyboardButton, 0, end-start)
	for _, bucketStart := range starts[start:end] {
		label := bucketLabel(bucketStart, 1000)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mu_pick:%d:%d:%d", mangaID, 1000, bucketStart)))
	}
	keyboard := appendButtonsInRows(nil, buttons, 2)

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mu_page:%d:%d", mangaID, page-1)))
	}
	if page < maxPage {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mu_page:%d:%d", mangaID, page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\n\nPick a range:", mangaTitle, lastReadLine, readCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadHundredsMenu(chatID int64, mangaID int, thousandStart int, root bool) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(thousandStart, 1000)

	starts, err := b.db.ListReadBucketStarts(mangaID, 100, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkUnreadTensMenu(chatID, mangaID, starts[0], root)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 100), fmt.Sprintf("mu_pick:%d:%d:%d", mangaID, 100, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mu_back_root:%d", mangaID)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, readCount, bucketLabel(thousandStart, 1000)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadTensMenu(chatID int64, mangaID int, hundredStart int, root bool) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(hundredStart, 100)

	starts, err := b.db.ListReadBucketStarts(mangaID, 10, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkUnreadChaptersMenuPage(chatID, mangaID, starts[0], root, 0)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 10), fmt.Sprintf("mu_pick:%d:%d:%d", mangaID, 10, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mu_back_hundreds:%d:%d", mangaID, hundredStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, readCount, bucketLabel(hundredStart, 100)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadChaptersMenuPage(chatID int64, mangaID int, tenStart int, root bool, page int) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(tenStart, 10)

	totalInRange, err := b.db.CountReadNumericChaptersInRange(mangaID, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	const pageSize = 30
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if totalInRange > 0 {
		maxPage = (totalInRange - 1) / pageSize
	}
	if page > maxPage {
		page = maxPage
	}
	chapters, err := b.db.ListReadNumericChaptersInRange(mangaID, rangeStart, rangeEnd, pageSize, page*pageSize)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
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
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("unread_chapter:%d:%s", mangaID, ch.Number)),
		})
	}

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mu_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page-1)))
	}
	if (page+1)*pageSize < totalInRange {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mu_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
	}
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mu_back_tens:%d:%d", mangaID, tenStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\nRange: %s\n\nSelect a chapter to mark it (and all following ones) as unread:", mangaTitle, lastReadLine, readCount, bucketLabel(tenStart, 10)))
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
		b.sendMarkUnreadStartMenu(chatID, mangaID)
	case "details":
		b.handleMangaDetails(chatID, mangaID)
	case "remove_manga":
		b.handleRemoveManga(chatID, mangaID)
	default:
		logger.LogMsg(logger.LogError, "Unknown next action: %s", nextAction)
	}
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
	b.sendMessageWithMainMenuButton(msg)
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

func appendButtonsInRows(keyboard [][]tgbotapi.InlineKeyboardButton, buttons []tgbotapi.InlineKeyboardButton, perRow int) [][]tgbotapi.InlineKeyboardButton {
	if perRow <= 1 {
		for _, btn := range buttons {
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{btn})
		}
		return keyboard
	}

	for i := 0; i < len(buttons); i += perRow {
		end := i + perRow
		if end > len(buttons) {
			end = len(buttons)
		}
		keyboard = append(keyboard, buttons[i:end])
	}
	return keyboard
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(n int) bool {
	return n != 0
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
