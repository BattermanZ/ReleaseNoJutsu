package bot

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) tryHandlePairingCode(message *tgbotapi.Message) bool {
	if message == nil || message.Text == "" {
		return false
	}
	if message.Chat == nil || message.From == nil {
		return false
	}
	if message.Chat.ID != message.From.ID || message.Chat.ID <= 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⚠️ Pairing codes can only be used in a private chat with the bot.")
		_, _ = b.api.Send(msg)
		return true
	}

	code, ok := parsePairingCode(message.Text)
	if !ok {
		return false
	}

	if b.isAuthorized(message.From.ID) {
		msg := tgbotapi.NewMessage(message.Chat.ID, "✅ You’re already authorized.")
		_, _ = b.api.Send(msg)
		return true
	}

	used, err := b.db.RedeemPairingCode(code, message.From.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ That pairing code is invalid or expired.")
		_, _ = b.api.Send(msg)
		return true
	}
	if !used {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ That pairing code is invalid or expired.")
		_, _ = b.api.Send(msg)
		return true
	}

	_ = b.db.EnsureUser(message.From.ID, false)
	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ You’re now authorized! Use /start to open the menu.")
	_, _ = b.api.Send(msg)
	return true
}

func (b *Bot) handleGeneratePairingCode(chatID int64, userID int64) {
	if !b.isAdmin(userID) {
		msg := tgbotapi.NewMessage(chatID, "🚫 Only the admin can generate pairing codes.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	code, err := generatePairingCode()
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Could not generate a pairing code right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	expiresAt := time.Now().UTC().Add(48 * time.Hour)
	if err := b.db.CreatePairingCode(code, userID, expiresAt); err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Could not store the pairing code right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🔑 Pairing code: <b>%s</b>\nValid until: <b>%s</b> (UTC)\n\nTell your friend to send this code to the bot in a private chat.", html.EscapeString(code), html.EscapeString(expiresAt.Format(time.RFC1123))))
	msg.ParseMode = "HTML"
	b.sendMessageWithMainMenuButton(msg)
}

func parsePairingCode(text string) (string, bool) {
	raw := strings.TrimSpace(strings.ToUpper(text))
	raw = strings.ReplaceAll(raw, " ", "")
	if len(raw) != 9 {
		return "", false
	}
	if raw[4] != '-' {
		return "", false
	}
	left := raw[:4]
	right := raw[5:]
	if !isHex4(left) || !isHex4(right) {
		return "", false
	}
	return left + "-" + right, true
}

func isHex4(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

func generatePairingCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	hexStr := strings.ToUpper(hex.EncodeToString(buf[:]))
	if len(hexStr) != 8 {
		return "", errors.New("invalid random hex length")
	}
	return hexStr[:4] + "-" + hexStr[4:], nil
}
