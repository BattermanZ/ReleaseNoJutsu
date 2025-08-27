package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

const (
	AppName = "ReleaseNoJutsu"

	LogError   = "ERROR"
	LogInfo    = "INFO"
	LogWarning = "WARN"
)

var (
	logger *log.Logger
)

func InitLogger() {
	err := os.MkdirAll("logs", os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create logs folder: %v", err)
	}

	logFile, err := os.OpenFile(filepath.Join("logs", AppName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger = log.New(multiWriter, "", log.Ldate|log.Ltime|log.Lshortfile)
	LogMsg(LogInfo, "Application started")
}

func LogMsg(level string, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	logger.Printf("[%s] %s", level, msg)
}
