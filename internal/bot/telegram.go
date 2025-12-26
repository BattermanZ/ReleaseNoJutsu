package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/config"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/mangadex"
)

// Bot represents the Telegram bot.

type Bot struct {
	api      *tgbotapi.BotAPI
	db       *db.DB
	mdClient *mangadex.Client
	config   *config.Config
}

// New creates a new Bot.

func New(api *tgbotapi.BotAPI, db *db.DB, mdClient *mangadex.Client, config *config.Config) *Bot {
	return &Bot{
		api:      api,
		db:       db,
		mdClient: mdClient,
		config:   config,
	}
}

// Start starts the bot and listens for updates.

func (b *Bot) Start() {
	logger.LogMsg(logger.LogInfo, "Authorized on account %s", b.api.Self.UserName)

	// Set bot commands
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Show the main menu"},
		{Command: "help", Description: "Show help information"},
	}
	_, _ = b.api.Request(tgbotapi.NewSetMyCommands(commands...))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
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
	_, _ = b.api.Send(msg)
}
