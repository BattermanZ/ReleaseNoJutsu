package notify

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Notifier interface {
	SendHTML(chatID int64, html string) error
}

type TelegramNotifier struct {
	api *tgbotapi.BotAPI
}

func NewTelegramNotifier(api *tgbotapi.BotAPI) *TelegramNotifier {
	return &TelegramNotifier{api: api}
}

func (n *TelegramNotifier) SendHTML(chatID int64, html string) error {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	_, err := n.api.Send(msg)
	return err
}
