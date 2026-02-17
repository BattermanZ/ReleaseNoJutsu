package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempCWD(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(tmp): %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	return tmp
}

func TestLoad_ParsesAndSetsAdmin(t *testing.T) {
	withTempCWD(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_ALLOWED_USERS", "123, 456")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.TelegramBotToken != "token" {
		t.Fatalf("token=%q, want token", cfg.TelegramBotToken)
	}
	if len(cfg.AllowedUsers) != 2 || cfg.AllowedUsers[0] != 123 || cfg.AllowedUsers[1] != 456 {
		t.Fatalf("allowed users=%v, want [123 456]", cfg.AllowedUsers)
	}
	if cfg.AdminUserID != 123 {
		t.Fatalf("AdminUserID=%d, want 123", cfg.AdminUserID)
	}
	if cfg.DatabasePath != filepath.Join("database", "ReleaseNoJutsu.db") {
		t.Fatalf("DatabasePath=%q, want database/ReleaseNoJutsu.db", cfg.DatabasePath)
	}
}

func TestLoad_EmptyAllowedUsersReturnsError(t *testing.T) {
	withTempCWD(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_ALLOWED_USERS", "   ")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for empty TELEGRAM_ALLOWED_USERS")
	}
}

func TestLoad_InvalidAllowedUserEntryReturnsError(t *testing.T) {
	withTempCWD(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_ALLOWED_USERS", "123,abc")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for invalid TELEGRAM_ALLOWED_USERS")
	}
}

func TestValidate(t *testing.T) {
	cfg := &Config{
		TelegramBotToken: "token",
		AllowedUsers:     []int64{1},
		AdminUserID:      1,
		DatabasePath:     "database/ReleaseNoJutsu.db",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(valid): %v", err)
	}

	cases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing token",
			cfg: Config{
				AllowedUsers: []int64{1},
				AdminUserID:  1,
				DatabasePath: "database/ReleaseNoJutsu.db",
			},
		},
		{
			name: "missing users",
			cfg: Config{
				TelegramBotToken: "token",
				AdminUserID:      1,
				DatabasePath:     "database/ReleaseNoJutsu.db",
			},
		},
		{
			name: "invalid admin",
			cfg: Config{
				TelegramBotToken: "token",
				AllowedUsers:     []int64{1},
				AdminUserID:      0,
				DatabasePath:     "database/ReleaseNoJutsu.db",
			},
		},
		{
			name: "missing db path",
			cfg: Config{
				TelegramBotToken: "token",
				AllowedUsers:     []int64{1},
				AdminUserID:      1,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatalf("Validate(%s) expected error", tc.name)
			}
		})
	}
}
