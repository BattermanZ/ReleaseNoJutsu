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
}

// New creates a new Bot.

func New(api TelegramAPI, db *db.DB, mdClient *mangadex.Client, config *config.Config, upd *updater.Updater) *Bot {
	return &Bot{
		api:      api,
		db:       db,
		mdClient: mdClient,
		config:   config,
		updater:  upd,
	}
}

// Run starts the bot and listens for updates until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	logger.LogMsg(logger.LogInfo, "Bot started")

	// Set bot commands
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Show the main menu"},
		{Command: "help", Description: "Show help information"},
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
				if !b.isAuthorized(update.Message.From.ID) {
					b.sendUnauthorizedMessage(update.Message.Chat.ID)
					continue
				}
				b.ensureUser(update.Message.Chat.ID)
				b.handleMessage(update.Message)
			} else if update.CallbackQuery != nil {
				if !b.isAuthorized(update.CallbackQuery.From.ID) {
					if update.CallbackQuery.Message != nil {
						b.sendUnauthorizedMessage(update.CallbackQuery.Message.Chat.ID)
					}
					continue
				}
				if update.CallbackQuery.Message != nil {
					b.ensureUser(update.CallbackQuery.Message.Chat.ID)
				}
				b.handleCallbackQuery(update.CallbackQuery)
			}
		}
	}
}

func (b *Bot) ensureUser(chatID int64) {
	if err := b.db.EnsureUser(chatID); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed to ensure chat ID %d in users table: %v", chatID, err)
	}
}

func (b *Bot) isAuthorized(userID int64) bool {
	for _, allowedID := range b.config.AllowedUsers {
		if userID == allowedID {
			return true
		}
	}
	return false
}

func (b *Bot) sendUnauthorizedMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🚫 Sorry, you are not authorised to use this bot.")
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending unauthorized message to %d: %v", chatID, err)
	}
}
