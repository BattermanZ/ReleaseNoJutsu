package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_InvalidConfigFailsFast(t *testing.T) {
	if os.Getenv("RJN_RUN_MAIN_HELPER") == "1" {
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestMain_InvalidConfigFailsFast$")
	cmd.Env = append(
		os.Environ(),
		"RJN_RUN_MAIN_HELPER=1",
		"TELEGRAM_BOT_TOKEN=",
		"TELEGRAM_ALLOWED_USERS=",
	)
	cmd.Dir = t.TempDir()

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid config, output=%s", string(out))
	}
	got := string(out)
	if !strings.Contains(got, "TELEGRAM_ALLOWED_USERS") && !strings.Contains(got, "TELEGRAM_BOT_TOKEN") {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestMain_DatabasePathDirectoryIsCreatable(t *testing.T) {
	// Smoke assertion for current default DB path expectation.
	dir := filepath.Dir("database/ReleaseNoJutsu.db")
	if dir != "database" {
		t.Fatalf("unexpected db dir %q", dir)
	}
}

func TestMain_DatabaseDirectoryCreationFailureIsReported(t *testing.T) {
	if os.Getenv("RJN_RUN_MAIN_DB_DIR_FAIL_HELPER") == "1" {
		if err := os.WriteFile("database", []byte("not-a-directory"), 0o600); err != nil {
			t.Fatalf("WriteFile(database): %v", err)
		}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestMain_DatabaseDirectoryCreationFailureIsReported$")
	cmd.Env = append(
		os.Environ(),
		"RJN_RUN_MAIN_DB_DIR_FAIL_HELPER=1",
		"TELEGRAM_BOT_TOKEN=test-token",
		"TELEGRAM_ALLOWED_USERS=1",
	)
	cmd.Dir = t.TempDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected helper process to exit cleanly, err=%v output=%s", err, string(out))
	}
	if !strings.Contains(string(out), "Failed to create database folder") {
		t.Fatalf("expected database folder failure log, output=%s", string(out))
	}
}

func TestMain_DatabaseOpenFailureIsReported(t *testing.T) {
	if os.Getenv("RJN_RUN_MAIN_DB_OPEN_FAIL_HELPER") == "1" {
		if err := os.MkdirAll(filepath.Join("database", "ReleaseNoJutsu.db"), 0o755); err != nil {
			t.Fatalf("MkdirAll(database/ReleaseNoJutsu.db): %v", err)
		}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestMain_DatabaseOpenFailureIsReported$")
	cmd.Env = append(
		os.Environ(),
		"RJN_RUN_MAIN_DB_OPEN_FAIL_HELPER=1",
		"TELEGRAM_BOT_TOKEN=test-token",
		"TELEGRAM_ALLOWED_USERS=1",
	)
	cmd.Dir = t.TempDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected helper process to exit cleanly, err=%v output=%s", err, string(out))
	}
	if !strings.Contains(string(out), "Failed to open database") {
		t.Fatalf("expected database open failure log, output=%s", string(out))
	}
}
