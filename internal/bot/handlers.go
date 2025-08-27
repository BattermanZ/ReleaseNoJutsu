package bot

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	b.logAction(message.From.ID, "Received message", message.Text)

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			b.sendMainMenu(message.Chat.ID)
		case "help":
			b.sendHelpMessage(message.Chat.ID)
		default:
			msg := tgbotapi.NewMessage(message.Chat.ID, "❓ Unknown command. Please use /start or /help.")
			_, _ = b.api.Send(msg)
		}
	} else if message.ReplyToMessage != nil && message.ReplyToMessage.Text != "" {
		b.handleReply(message)
	} else {
		// Check if the message is a MangaDex URL
		mangaID, err := b.mdClient.ExtractMangaIDFromURL(message.Text)
		if err == nil {
			b.handleAddManga(message.Chat.ID, mangaID)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, "I’m not sure what you mean. Use /start to see available options.")
			_, _ = b.api.Send(msg)
		}
	}
}

func (b *Bot) handleReply(message *tgbotapi.Message) {
	b.logAction(message.From.ID, "Received reply", message.Text)

	replyTo := message.ReplyToMessage.Text
	switch {
	case strings.Contains(replyTo, "enter the MangaDex ID"):
		b.handleAddManga(message.Chat.ID, message.Text)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "I didn’t understand that reply. Please use /start for options.")
		_, _ = b.api.Send(msg)
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
	_, _ = b.api.Send(msg)
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

	_, _ = b.api.Send(msg)
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

*Main Features:*
- *Add manga:* Start tracking a new manga by sending its MangaDex URL or ID.
- *List followed manga:* See which series you're currently tracking.
- *Check for new chapters:* Quickly see if any of your followed manga have fresh releases.
- *Mark chapter as read:* Update your progress so you know which chapters you've finished.
- *List read chapters:* Review what you've read recently.

*How to add a manga:*
Simply send the MangaDex URL (e.g., https://mangadex.org/title/123e4567-e89b-12d3-a456-426614174000) or the MangaDex ID (e.g., 123e4567-e89b-12d3-a456-426614174000) directly to the bot. The bot will automatically detect and add the manga.

If you need further assistance, feel free to /start and explore the menu options!`
	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) handleAddManga(chatID int64, mangaID string) {
	b.logAction(chatID, "Add manga", mangaID)

	mangaData, err := b.mdClient.GetManga(mangaID)
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

	b.fetchLastChapters(mangaDBID, mangaID)

	result := fmt.Sprintf("✅ *%s* has been added successfully!\nThe last few chapters have also been fetched.", title)
	msg := tgbotapi.NewMessage(chatID, result)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
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
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked)
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
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked)
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

func (b *Bot) sendChapterSelectionMenu(chatID int64, mangaID int) {
	rows, err := b.db.GetUnreadChapters(mangaID)
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
		callbackData := fmt.Sprintf("mark_chapter:%d:%s", mangaID, chapterNumber)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Ch. %s: %s", chapterNumber, ctitle), callbackData),
		})
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	messageText := fmt.Sprintf(`📖 *%s*

Select a chapter below to mark it (and all previous ones) as read:`, mangaTitle)
	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "Markdown"
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
		// This is now handled by the cron job, we can provide a message to the user
		msg := tgbotapi.NewMessage(chatID, "The bot automatically checks for new chapters every 6 hours.")
		b.sendMessageWithMainMenuButton(msg)
	case "mark_read":
		b.sendChapterSelectionMenu(chatID, mangaID)
	case "list_read":
		b.sendReadChaptersMenu(chatID, mangaID)
	case "remove_manga":
		b.handleRemoveManga(chatID, mangaID)
	default:
		logger.LogMsg(logger.LogError, "Unknown next action: %s", nextAction)
	}
}

func (b *Bot) fetchLastChapters(mangaDBID int64, mangadexID string) {
	chapterFeed, err := b.mdClient.GetChapterFeed(mangadexID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error fetching chapter data: %v", err)
		return
	}

	chaptersToStore := chapterFeed.Data
	if len(chaptersToStore) > 3 {
		chaptersToStore = chaptersToStore[:3]
	}

	for _, chapter := range chaptersToStore {
		err := b.db.AddChapter(mangaDBID, chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error inserting chapter into database: %v", err)
		}
	}
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
