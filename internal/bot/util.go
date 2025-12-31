package bot

import "releasenojutsu/internal/logger"

func (b *Bot) logAction(userID int64, action, details string) {
	logger.LogMsg(logger.LogInfo, "[User: %d] [%s] %s", userID, action, details)
}
