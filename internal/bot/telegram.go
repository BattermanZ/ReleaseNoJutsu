package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/config"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/mangadex"
	"releasenojutsu/internal/updater"
)

// Bot represents the Telegram bot.

type TelegramAPI interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	StopReceivingUpdates()
}

type Bot struct {
	api      TelegramAPI
	db       *db.DB
	mdClient *mangadex.Client
	config   *config.Config
	updater  *updater.Updater

	authorizedCache map[int64]struct{}
}

// New creates a new Bot.

func New(api TelegramAPI, db *db.DB, mdClient *mangadex.Client, config *config.Config, upd *updater.Updater) *Bot {
	return &Bot{
		api:      api,
		db:       db,
		mdClient: mdClient,
		config:   config,
		updater:  upd,
		authorizedCache: map[int64]struct{}{
			config.AdminUserID: {},
		},
	}
}

// Run starts the bot and listens for updates until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	logger.LogMsg(logger.LogInfo, "Bot started")

	// Set bot commands
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Show the main menu"},
		{Command: "help", Description: "Show help information"},
		{Command: "status", Description: "Show status/health information"},
		{Command: "genpair", Description: "Generate a pairing code (admin only)"},
	}
	if _, err := b.api.Request(tgbotapi.NewSetMyCommands(commands...)); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed to set bot commands: %v", err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message != nil {
				if !isPrivateChat(update.Message.Chat, update.Message.From) {
					b.sendPrivateOnlyMessage(update.Message.Chat.ID)
					continue
				}
				if !b.isAuthorized(update.Message.From.ID) {
					if b.tryHandlePairingCode(update.Message) {
						continue
					}
					b.sendUnauthorizedMessage(update.Message.Chat.ID)
					continue
				}
				b.ensureUser(update.Message.Chat.ID, update.Message.From.ID, b.isAdmin(update.Message.From.ID))
				b.handleMessage(update.Message)
			} else if update.CallbackQuery != nil {
				if update.CallbackQuery.Message != nil && !isPrivateChat(update.CallbackQuery.Message.Chat, update.CallbackQuery.From) {
					b.sendPrivateOnlyMessage(update.CallbackQuery.Message.Chat.ID)
					continue
				}
				if !b.isAuthorized(update.CallbackQuery.From.ID) {
					if update.CallbackQuery.Message != nil {
						b.sendUnauthorizedMessage(update.CallbackQuery.Message.Chat.ID)
					}
					continue
				}
				if update.CallbackQuery.Message != nil {
					b.ensureUser(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.From.ID, b.isAdmin(update.CallbackQuery.From.ID))
				}
				b.handleCallbackQuery(update.CallbackQuery)
			}
		}
	}
}

func (b *Bot) ensureUser(chatID int64, fromUserID int64, isAdmin bool) {
	// Hardening: only store private chat IDs for notifications.
	// In private chats, Chat.ID equals the user ID.
	if chatID <= 0 || chatID != fromUserID {
		return
	}
	if err := b.db.EnsureUser(chatID, isAdmin); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed to ensure chat ID %d in users table: %v", chatID, err)
	}
}

func (b *Bot) isAuthorized(userID int64) bool {
	if _, ok := b.authorizedCache[userID]; ok {
		return true
	}
	if b.isAdmin(userID) {
		return true
	}
	ok, _, err := b.db.IsUserAuthorized(userID)
	if err != nil {
		logger.LogMsg(logger.LogWarning, "Auth lookup failed for %d: %v", userID, err)
		return false
	}
	if ok {
		b.authorizedCache[userID] = struct{}{}
	}
	return ok
}

func (b *Bot) isAdmin(userID int64) bool {
	return userID == b.config.AdminUserID
}

func (b *Bot) sendUnauthorizedMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🚫 You’re not authorized yet.\nAsk the admin for a pairing code and send it here (format: XXXX-XXXX).")
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending unauthorized message to %d: %v", chatID, err)
	}
}

func (b *Bot) sendPrivateOnlyMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🚫 This bot can only be used in a private chat.")
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending private-only message to %d: %v", chatID, err)
	}
}

func isPrivateChat(chat *tgbotapi.Chat, from *tgbotapi.User) bool {
	if chat == nil || from == nil {
		return false
	}
	return chat.ID == from.ID && chat.ID > 0
}
