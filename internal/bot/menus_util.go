package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) lastReadLine(mangaID int) string {
	lastReadNum, lastReadTitle, hasLastRead, err := b.db.GetLastReadChapter(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting last read chapter: %v", err)
	}
	if !hasLastRead {
		return "Last read: (none)"
	}
	if strings.TrimSpace(lastReadTitle) == "" {
		return fmt.Sprintf("Last read: Ch. %s", lastReadNum)
	}
	return fmt.Sprintf("Last read: Ch. %s — %s", lastReadNum, lastReadTitle)
}

func bucketLabel(start, bucketSize int) string {
	if start == 1 {
		return fmt.Sprintf("1-%d", bucketSize-1)
	}
	return fmt.Sprintf("%d-%d", start, start+bucketSize-1)
}

func appendButtonsInRows(keyboard [][]tgbotapi.InlineKeyboardButton, buttons []tgbotapi.InlineKeyboardButton, perRow int) [][]tgbotapi.InlineKeyboardButton {
	if perRow <= 1 {
		for _, btn := range buttons {
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{btn})
		}
		return keyboard
	}

	for i := 0; i < len(buttons); i += perRow {
		end := i + perRow
		if end > len(buttons) {
			end = len(buttons)
		}
		keyboard = append(keyboard, buttons[i:end])
	}
	return keyboard
}

func bucketRange(start, bucketSize int) (float64, float64) {
	if start == 1 {
		return 1, float64(bucketSize)
	}
	return float64(start), float64(start + bucketSize)
}

func thousandBucketStart(n int) int {
	if n < 1000 {
		return 1
	}
	return (n / 1000) * 1000
}

func hundredBucketStart(n int) int {
	if n < 100 {
		return 1
	}
	return (n / 100) * 100
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(n int) bool {
	return n != 0
}
