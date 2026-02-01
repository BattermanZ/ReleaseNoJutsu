package bot

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/logger"
)

func (b *Bot) sendMainMenu(chatID int64) {
	b.logAction(chatID, "Sent main menu", "")

	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.AddManga, "add_manga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.ListManga, "list_manga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.CheckNew, "check_new"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkRead, "mark_read"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.SyncAll, "sync_all"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MarkUnread, "list_read"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.RemoveManga, "remove_manga"),
		),
	}

	if b.isAdmin(chatID) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.GeneratePairingCode, "gen_pair"),
		))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	welcomeMessage := appcopy.Copy.Info.WelcomeTitle
	msg := tgbotapi.NewMessage(chatID, welcomeMessage)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", chatID, err)
	}
}

func (b *Bot) sendMessageWithMainMenuButton(msg tgbotapi.MessageConfig) {
	mainMenuButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(appcopy.Copy.Buttons.MainMenu, "main_menu"),
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

	if _, err := b.api.Send(msg); err != nil {
		logger.LogMsg(logger.LogWarning, "Failed sending message to %d: %v", msg.ChatID, err)
	}
}

func (b *Bot) sendHelpMessage(chatID int64) {
	b.logAction(chatID, "Sent help message", "")

	msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Info.HelpText)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendStatusMessage(chatID int64) {
	b.logAction(chatID, "Sent status", "")

	status, err := b.db.GetStatus()
	if err != nil {
		logger.LogMsg(logger.LogError, "Error getting status: %v", err)
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotRetrieveStatus)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var bld strings.Builder
	bld.WriteString("<b>" + appcopy.Copy.Info.StatusTitle + "</b>\n\n")
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusTracked, status.MangaCount))
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusChaptersStored, status.ChapterCount))
	bld.WriteString(fmt.Sprintf(appcopy.Copy.Info.StatusRegisteredChats, status.UserCount))
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
