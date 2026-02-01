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

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/logger"
)

func (b *Bot) tryHandlePairingCode(message *tgbotapi.Message) bool {
	if message == nil || message.Text == "" {
		return false
	}
	if message.Chat == nil || message.From == nil {
		return false
	}
	if message.Chat.ID != message.From.ID || message.Chat.ID <= 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.PairingPrivateOnly)
		_, _ = b.api.Send(msg)
		return true
	}

	code, ok := parsePairingCode(message.Text)
	if !ok {
		return false
	}

	if b.isAuthorized(message.From.ID) {
		msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.PairingAlreadyAuth)
		_, _ = b.api.Send(msg)
		return true
	}

	used, err := b.db.RedeemPairingCode(code, message.From.ID)
	if err != nil {
		logger.LogMsg(logger.LogWarning, "Pairing code redeem failed: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.PairingInvalid)
		_, _ = b.api.Send(msg)
		return true
	}
	if !used {
		msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.PairingInvalid)
		_, _ = b.api.Send(msg)
		return true
	}

	_ = b.db.EnsureUser(message.From.ID, false)
	b.authorizedCache[message.From.ID] = struct{}{}
	msg := tgbotapi.NewMessage(message.Chat.ID, appcopy.Copy.Prompts.PairingSuccess)
	_, _ = b.api.Send(msg)
	return true
}

func (b *Bot) handleGeneratePairingCode(chatID int64, userID int64) {
	if !b.isAdmin(userID) {
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Prompts.AdminOnly)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	code, err := generatePairingCode()
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotGeneratePair)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	expiresAt := time.Now().UTC().Add(48 * time.Hour)
	if err := b.db.CreatePairingCode(code, userID, expiresAt); err != nil {
		msg := tgbotapi.NewMessage(chatID, appcopy.Copy.Errors.CannotStorePair)
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(appcopy.Copy.Prompts.PairingCodeGenerated, html.EscapeString(code), html.EscapeString(expiresAt.Format(time.RFC1123))))
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
