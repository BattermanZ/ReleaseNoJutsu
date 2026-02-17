package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/logger"
)

const pendingStateAddManga = "add_manga"

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	details := fmt.Sprintf("chat_id=%d is_command=%t len=%d", message.Chat.ID, message.IsCommand(), len(message.Text))
	if message.IsCommand() {
		details += fmt.Sprintf(" cmd=%s", message.Command())
	}
	b.logAction(message.From.ID, "Received message", details)

	if message.IsCommand() {
		if err := b.db.ClearUserPendingState(message.From.ID); err != nil {
			logger.LogMsg(logger.LogWarning, "Failed clearing pending state for %d: %v", message.From.ID, err)
		}

		switch message.Command() {
		case appcopy.Copy.Commands.Start:
			b.sendMainMenu(message.Chat.ID)
		case appcopy.Copy.Commands.Help:
			b.sendHelpMessage(message.Chat.ID)
		case appcopy.Copy.Commands.Status:
			b.sendStatusMessage(message.Chat.ID, message.From.ID)
		case appcopy.Copy.Commands.GenPair:
			b.handleGeneratePairingCode(message.Chat.ID, message.From.ID)
		default:
			msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.UnknownCommand)
			if _, err := b.api.Send(msg); err != nil {
				logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", message.Chat.ID, err)
			}
		}
	} else if b.consumePendingInput(message) {
		return
	} else if message.ReplyToMessage != nil && message.ReplyToMessage.Text != "" {
		b.handleReply(message)
	} else {
		text := strings.TrimSpace(message.Text)
		if b.looksLikeMangaDexID(text) {
			b.handleAddManga(message.Chat.ID, message.From.ID, text)
			return
		}

		// Check if the message is a MangaDex URL.
		mangaID, err := b.mdClient.ExtractMangaIDFromURL(text)
		if err == nil {
			b.handleAddManga(message.Chat.ID, message.From.ID, mangaID)
			return
		}

		msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.UnknownMessage)
		if _, err := b.api.Send(msg); err != nil {
			logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", message.Chat.ID, err)
		}
	}
}

func (b *Bot) consumePendingInput(message *tgbotapi.Message) bool {
	state, _, hasState, err := b.db.GetUserPendingState(message.From.ID)
	if err != nil {
		logger.LogMsg(logger.LogWarning, "Failed loading pending state for %d: %v", message.From.ID, err)
		return false
	}
	if !hasState {
		return false
	}

	if err := b.db.ClearUserPendingState(message.From.ID); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed clearing pending state for %d: %v", message.From.ID, err)
	}

	switch state {
	case pendingStateAddManga:
		replyText := strings.TrimSpace(message.Text)
		if mangaID, err := b.mdClient.ExtractMangaIDFromURL(replyText); err == nil {
			b.handleAddManga(message.Chat.ID, message.From.ID, mangaID)
			return true
		}
		b.handleAddManga(message.Chat.ID, message.From.ID, replyText)
		return true
	default:
		logger.LogMsg(logger.LogWarning, "Unknown pending state %q for user %d", state, message.From.ID)
		return false
	}
}

func (b *Bot) handleReply(message *tgbotapi.Message) {
	b.logAction(message.From.ID, "Received reply", fmt.Sprintf("chat_id=%d len=%d", message.Chat.ID, len(message.Text)))

	replyText := strings.TrimSpace(message.Text)

	if mangaID, err := b.mdClient.ExtractMangaIDFromURL(replyText); err == nil {
		b.handleAddManga(message.Chat.ID, message.From.ID, mangaID)
		return
	}
	if b.looksLikeMangaDexID(replyText) {
		b.handleAddManga(message.Chat.ID, message.From.ID, replyText)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.UnknownReply)
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", message.Chat.ID, err)
	}
}

func (b *Bot) looksLikeMangaDexID(text string) bool {
	// MangaDex IDs are UUIDs: 36 chars with 4 hyphens.
	text = strings.TrimSpace(text)
	return len(text) == 36 && strings.Count(text, "-") == 4
}
