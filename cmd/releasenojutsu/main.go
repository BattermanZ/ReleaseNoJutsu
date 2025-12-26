package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/bot"
	"releasenojutsu/internal/config"
	"releasenojutsu/internal/cron"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/mangadex"
	"releasenojutsu/internal/notify"
	"releasenojutsu/internal/updater"
)

func main() {
	logger.InitLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Ensure database folder exists
	dbDir := filepath.Dir(cfg.DatabasePath)
	err = os.MkdirAll(dbDir, os.ModePerm)
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to create database folder: %v", err)
		return
	}

	database, err := db.New(cfg.DatabasePath)
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to open database: %v", err)
		return
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.LogMsg(logger.LogError, "Failed to close database: %v", err)
		}
	}()

	err = database.CreateTables()
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to create tables: %v", err)
		return
	}
	if err := database.Migrate(); err != nil {
		logger.LogMsg(logger.LogError, "Failed to migrate database: %v", err)
		return
	}

	mdClient := mangadex.NewClient()

	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to initialize Telegram bot: %v", err)
		return
	}

	upd := updater.New(database, mdClient)
	notifier := notify.NewTelegramNotifier(api)

	appBot := bot.New(api, database, mdClient, cfg, upd)

	scheduler := cron.NewScheduler(database, notifier, upd)
	go scheduler.Run(ctx)

	if err := appBot.Run(ctx); err != nil {
		logger.LogMsg(logger.LogError, "Bot exited with error: %v", err)
	}
}
