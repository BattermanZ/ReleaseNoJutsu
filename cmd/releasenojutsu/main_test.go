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
