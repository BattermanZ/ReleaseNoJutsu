package main

import (
	"log"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/bot"
	"releasenojutsu/internal/config"
	"releasenojutsu/internal/cron"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/mangadex"
)

func main() {
	logger.InitLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

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

	mdClient := mangadex.NewClient()

	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to initialize Telegram bot: %v", err)
		return
	}

	appBot := bot.New(api, database, mdClient, cfg)

	scheduler := cron.NewScheduler(database, api, mdClient)
	scheduler.Start()

	appBot.Start()
}
