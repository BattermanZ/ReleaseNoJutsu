package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	details := fmt.Sprintf("chat_id=%d is_command=%t len=%d", message.Chat.ID, message.IsCommand(), len(message.Text))
	if message.IsCommand() {
		details += fmt.Sprintf(" cmd=%s", message.Command())
	}
	b.logAction(message.From.ID, "Received message", details)

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
	b.logAction(message.From.ID, "Received reply", fmt.Sprintf("chat_id=%d len=%d", message.Chat.ID, len(message.Text)))

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

func (b *Bot) looksLikeMangaDexID(text string) bool {
	// MangaDex IDs are UUIDs: 36 chars with 4 hyphens.
	text = strings.TrimSpace(text)
	return len(text) == 36 && strings.Count(text, "-") == 4
}
