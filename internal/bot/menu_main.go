package bot

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/logger"
)

type callbackEditTarget struct {
	chatID    int64
	messageID int
}

func firstCallbackTarget(targets ...*callbackEditTarget) *callbackEditTarget {
	if len(targets) == 0 {
		return nil
	}
	return targets[0]
}

func (b *Bot) sendMainMenu(chatID int64, target ...*callbackEditTarget) {
	b.logAction(chatID, "Sent main menu", "")

	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.AddManga, cbAddManga()),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.ListManga, cbListManga()),
		),
	}

	if b.isAdmin(chatID) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.GeneratePairingCode, cbGenPair()),
		))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	welcomeMessage := appcopy.Copy.Info.WelcomeTitle
	msg := tgbotapi.NewMessage(chatID, welcomeMessage)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	b.sendOrEditMessage(msg, firstCallbackTarget(target...))
}

func (b *Bot) sendMessageWithMainMenuButton(msg tgbotapi.MessageConfig, target ...*callbackEditTarget) {
	mainMenuButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MainMenu, cbMainMenu()),
		),
	)

	if msg.ReplyMarkup != nil {
		if keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, mainMenuButton.InlineKeyboard...)
			msg.ReplyMarkup = keyboard
		} else {
			msg.ReplyMarkup = mainMenuButton
		}
	} else {
		msg.ReplyMarkup = mainMenuButton
	}

	b.sendOrEditMessage(msg, firstCallbackTarget(target...))
}

func (b *Bot) sendOrEditMessage(msg tgbotapi.MessageConfig, target *callbackEditTarget) {
	if b.tryEditTarget(msg, target) {
		return
	}

	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", msg.ChatID, err)
	}
}

func (b *Bot) tryEditTarget(msg tgbotapi.MessageConfig, target *callbackEditTarget) bool {
	if target == nil || target.chatID != msg.ChatID {
		return false
	}

	var req tgbotapi.Chattable
	if keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
		edit := tgbotapi.NewEditMessageTextAndMarkup(msg.ChatID, target.messageID, msg.Text, keyboard)
		edit.ParseMode = msg.ParseMode
		edit.DisableWebPagePreview = msg.DisableWebPagePreview
		req = edit
	} else {
		edit := tgbotapi.NewEditMessageText(msg.ChatID, target.messageID, msg.Text)
		edit.ParseMode = msg.ParseMode
		edit.DisableWebPagePreview = msg.DisableWebPagePreview
		req = edit
	}

	if _, err := b.api.Request(req); err != nil {
		// Telegram returns this when user taps the same option and content is unchanged.
		if strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
			return true
		}
		logger.LogMsg(logger.LogWarning, "Failed editing message %d in chat %d: %v", target.messageID, msg.ChatID, err)
		return false
	}
	return true
}

func (b *Bot) sendHelpMessage(chatID int64) {
	b.logAction(chatID, "Sent help message", "")

	msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Info.HelpText)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendStatusMessage(chatID int64, userID int64) {
	b.logAction(chatID, "Sent status", "")

	status, err := b.db.GetStatusByUser(userID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting status: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRetrieveStatus)
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	isAdmin := b.isAdmin(userID)
	if isAdmin {
		globalStatus, err := b.db.GetStatus()
		if err != nil {
			logger.LogMsg(logger.LogError, "Error getting global status: %v", err)
			msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRetrieveStatus)
			b.sendMessageWithMainMenuButton(msg)
			return
		}
		status.UserCount = globalStatus.UserCount
	}

	var bld strings.Builder
	bld.WriteString("<b>" + appcopy.Copy.Info.StatusTitle + "</b>\n\n")
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusTracked, status.MangaCount))
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusChaptersStored, status.ChapterCount))
	if isAdmin {
		bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusRegisteredChats, status.UserCount))
	}
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusTotalUnread, status.UnreadTotal))
	if status.HasCronLastRun {
		bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusLastRun, status.CronLastRun.Local().Format(time.RFC1123)))
	} else {
		bld.WriteString(appcopy.Copy.Info.StatusCronNever)
	}
	bld.WriteString(appcopy.Copy.Info.StatusInterval)

	msg := tgbotapi.NewMessage(chatID, bld.String())
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}
