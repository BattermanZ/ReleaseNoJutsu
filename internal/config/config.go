package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the application configuration

type Config struct {
	TelegramBotToken string
	AllowedUsers     []int64
	DatabasePath     string
}

// Load loads the configuration from environment variables

func Load() (*Config, error) {
	// Attempt to load .env file. This is primarily for local development.
	// In Docker, environment variables are expected to be set directly.
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) { // Only return error if it's not a "file not found" error
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
		AllowedUsers:     allowedUsers,
		DatabasePath:     "database/ReleaseNoJutsu.db",
	}, nil
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.TelegramBotToken) == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if len(c.AllowedUsers) == 0 {
		return fmt.Errorf("TELEGRAM_ALLOWED_USERS is required (at least 1 user id)")
	}
	if strings.TrimSpace(c.DatabasePath) == "" {
		return fmt.Errorf("database path is required")
	}
	return nil
}
