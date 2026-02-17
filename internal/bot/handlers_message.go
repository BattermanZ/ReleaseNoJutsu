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
		if mangaID, ok := b.mangaInputToID(message.Text); ok {
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

	switch state {
	case pendingStateAddManga:
		mangaID, ok := b.mangaInputToID(message.Text)
		if !ok {
			// Keep pending state until the user sends a valid MangaDex URL/ID.
			b.sendAddMangaPrompt(message.Chat.ID)
			return true
		}
		if err := b.db.ClearUserPendingState(message.From.ID); err != nil {
			logger.LogMsg(logger.LogWarning, "Failed clearing pending state for %d: %v", message.From.ID, err)
		}
		b.handleAddManga(message.Chat.ID, message.From.ID, mangaID)
		return true
	default:
		logger.LogMsg(logger.LogWarning, "Unknown pending state %q for user %d", state, message.From.ID)
		return false
	}
}

func (b *Bot) handleReply(message *tgbotapi.Message) {
	b.logAction(message.From.ID, "Received reply", fmt.Sprintf("chat_id=%d len=%d", message.Chat.ID, len(message.Text)))

	msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.UnknownReply)
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", message.Chat.ID, err)
	}
}

func (b *Bot) mangaInputToID(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	if b.mdClient != nil {
		if mangaID, err := b.mdClient.ExtractMangaIDFromURL(text); err == nil {
			return mangaID, true
		}
	}
	if b.looksLikeMangaDexID(text) {
		return text, true
	}
	return "", false
}

func (b *Bot) sendAddMangaPrompt(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.AddMangaTitle)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.CancelAdd, cbCancelPending()),
		),
	)
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) looksLikeMangaDexID(text string) bool {
	// MangaDex IDs are UUIDs: 36 chars with 4 hyphens.
	text = strings.TrimSpace(text)
	return len(text) == 36 && strings.Count(text, "-") == 4
}
