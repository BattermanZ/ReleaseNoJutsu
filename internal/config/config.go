package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the application configuration

type Config struct {
	TelegramBotToken  string
	AllowedUsers      []int64
	DatabasePath      string
}

// Load loads the configuration from environment variables

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	allowedUsersStr := os.Getenv("TELEGRAM_ALLOWED_USERS")
	allowedUserIDs := strings.Split(allowedUsersStr, ",")
	allowedUsers := make([]int64, 0, len(allowedUserIDs))
	for _, userID := range allowedUserIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 64)
		if err == nil {
			allowedUsers = append(allowedUsers, id)
		}
	}

	return &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		AllowedUsers:      allowedUsers,
		DatabasePath:      "database/ReleaseNoJutsu.db",
	}, nil
}
