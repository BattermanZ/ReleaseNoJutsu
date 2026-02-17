package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitLoggerAndLogMsg(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(tmp): %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	InitLogger()
	LogMsg(LogInfo, "test log message %d", 123)

	logPath := filepath.Join("logs", AppName+".log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", logPath, err)
	}
	text := string(content)
	if !strings.Contains(text, "Application started") {
		t.Fatalf("log missing startup line: %q", text)
	}
	if !strings.Contains(text, "test log message 123") {
		t.Fatalf("log missing message line: %q", text)
	}
}
