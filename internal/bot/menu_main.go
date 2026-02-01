package bot

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) sendMainMenu(chatID int64) {
	b.logAction(chatID, "Sent main menu", "")

	rows := [][]tgbotapi.InlineKeyboardButton{
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
	}

	if b.isAdmin(chatID) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔑 Generate pairing code", "gen_pair"),
		))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

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

If you need access, ask the admin for a pairing code and send it to the bot in a private chat.

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
