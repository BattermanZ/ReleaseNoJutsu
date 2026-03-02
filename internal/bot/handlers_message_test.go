package bot

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/config"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
)

type fakeTelegramAPI struct {
	mu               sync.Mutex
	sent             []tgbotapi.Chattable
	outboundMessages []tgbotapi.MessageConfig
	failEditRequests bool
}

func (f *fakeTelegramAPI) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return nil
}

func (f *fakeTelegramAPI) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch cfg := c.(type) {
	case tgbotapi.EditMessageTextConfig:
		if f.failEditRequests {
			return nil, errors.New("forced edit failure")
		}
		msg := tgbotapi.NewMessage(cfg.ChatID, cfg.Text)
		msg.ParseMode = cfg.ParseMode
		if cfg.ReplyMarkup != nil {
			msg.ReplyMarkup = *cfg.ReplyMarkup
		}
		f.outboundMessages = append(f.outboundMessages, msg)
	}
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (f *fakeTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, c)
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		f.outboundMessages = append(f.outboundMessages, msg)
	}
	return tgbotapi.Message{}, nil
}

func (f *fakeTelegramAPI) StopReceivingUpdates() {}

func (f *fakeTelegramAPI) lastMessageText(t *testing.T) string {
	t.Helper()
	return f.lastMessageConfig(t).Text
}

func (f *fakeTelegramAPI) lastMessageConfig(t *testing.T) tgbotapi.MessageConfig {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.outboundMessages) == 0 {
		t.Fatal("no sent messages")
	}
	return f.outboundMessages[len(f.outboundMessages)-1]
}

func (f *fakeTelegramAPI) sentMessageTexts(t *testing.T) []string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.outboundMessages))
	for _, msg := range f.outboundMessages {
		out = append(out, msg.Text)
	}
	return out
}

func setupBotForMessageTests(t *testing.T) (*Bot, *db.DB, *fakeTelegramAPI) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateTables(); err != nil {
		t.Fatalf("CreateTables(): %v", err)
	}
	if err := database.Migrate(1); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	api := &fakeTelegramAPI{}
	cfg := &config.Config{AdminUserID: 1}
	b := New(api, database, mangadex.NewClient(), cfg, nil)
	return b, database, api
}

func TestMangaInputToID(t *testing.T) {
	b := &Bot{mdClient: mangadex.NewClient()}

	const uuid = "40bc649f-7b49-4645-859e-6cd94136e722"
	tests := []struct {
		name   string
		input  string
		wantID string
		wantOK bool
	}{
		{
			name:   "url",
			input:  "https://mangadex.org/title/40bc649f-7b49-4645-859e-6cd94136e722/dragon-ball",
			wantID: uuid,
			wantOK: true,
		},
		{
			name:   "uuid",
			input:  uuid,
			wantID: uuid,
			wantOK: true,
		},
		{
			name:   "invalid",
			input:  "not-a-manga-id",
			wantID: "",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := b.mangaInputToID(tc.input)
			if ok != tc.wantOK || got != tc.wantID {
				t.Fatalf("mangaInputToID(%q) = (%q,%v), want (%q,%v)", tc.input, got, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}

func TestConsumePendingInput_InvalidInputKeepsState(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	if err := database.SetUserPendingState(userID, pendingStateAddManga, ""); err != nil {
		t.Fatalf("SetUserPendingState(): %v", err)
	}

	msg := &tgbotapi.Message{
		Text: "hello there",
		From: &tgbotapi.User{ID: userID},
		Chat: &tgbotapi.Chat{ID: userID},
	}

	if !b.consumePendingInput(msg) {
		t.Fatal("consumePendingInput() = false, want true")
	}

	state, _, hasState, err := database.GetUserPendingState(userID)
	if err != nil {
		t.Fatalf("GetUserPendingState(): %v", err)
	}
	if !hasState || state != pendingStateAddManga {
		t.Fatalf("pending state = (%q,%v), want (%q,true)", state, hasState, pendingStateAddManga)
	}

	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.AddMangaTitle {
		t.Fatalf("last sent text = %q, want %q", got, appcopy.Copy.Prompts.AddMangaTitle)
	}
}

func TestHandleMessage_ReplyDoesNotAutoAddManga(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	b.mdClient = nil // This test should pass without trying to fetch/parse MangaDex data.

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	msg := &tgbotapi.Message{
		Text: "https://mangadex.org/title/40bc649f-7b49-4645-859e-6cd94136e722/dragon-ball",
		From: &tgbotapi.User{ID: userID},
		Chat: &tgbotapi.Chat{ID: userID},
		ReplyToMessage: &tgbotapi.Message{
			Text: "previous prompt",
		},
	}

	b.handleMessage(msg)

	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.UnknownReply {
		t.Fatalf("last sent text = %q, want %q", got, appcopy.Copy.Prompts.UnknownReply)
	}
}

func TestSendAddMangaPrompt_HasCancelAction(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	chatID := int64(42)
	b.sendAddMangaPrompt(chatID)

	msg := api.lastMessageConfig(t)
	if msg.Text != appcopy.Copy.Prompts.AddMangaTitle {
		t.Fatalf("prompt text = %q, want %q", msg.Text, appcopy.Copy.Prompts.AddMangaTitle)
	}

	keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("ReplyMarkup type = %T, want InlineKeyboardMarkup", msg.ReplyMarkup)
	}
	if len(keyboard.InlineKeyboard) == 0 || len(keyboard.InlineKeyboard[0]) == 0 {
		t.Fatalf("keyboard is empty")
	}
	found := false
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == appcopy.Copy.Buttons.CancelAdd && btn.CallbackData != nil && *btn.CallbackData == cbCancelPending() {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("cancel add button not found")
	}
}

func TestHandleMessage_HelpCommand(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	msg := &tgbotapi.Message{
		Text: "/help",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
		From: &tgbotapi.User{ID: userID},
		Chat: &tgbotapi.Chat{ID: userID},
	}
	b.handleMessage(msg)

	got := api.lastMessageConfig(t)
	if got.Text != appcopy.Copy.Info.HelpText {
		t.Fatalf("help text mismatch")
	}
}

func TestHandleMessage_UnknownCommand(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	msg := &tgbotapi.Message{
		Text: "/doesnotexist",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 12},
		},
		From: &tgbotapi.User{ID: userID},
		Chat: &tgbotapi.Chat{ID: userID},
	}
	b.handleMessage(msg)

	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.UnknownCommand {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.UnknownCommand)
	}
}

func TestHandleMessage_UnknownPlainText(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	msg := &tgbotapi.Message{
		Text: "hello there",
		From: &tgbotapi.User{ID: userID},
		Chat: &tgbotapi.Chat{ID: userID},
	}
	b.handleMessage(msg)

	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.UnknownMessage {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.UnknownMessage)
	}
}
