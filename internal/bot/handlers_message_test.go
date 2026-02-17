package bot

import (
	"path/filepath"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/config"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
)

type fakeTelegramAPI struct {
	sent []tgbotapi.Chattable
}

func (f *fakeTelegramAPI) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return nil
}

func (f *fakeTelegramAPI) Request(_ tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (f *fakeTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.sent = append(f.sent, c)
	return tgbotapi.Message{}, nil
}

func (f *fakeTelegramAPI) StopReceivingUpdates() {}

func (f *fakeTelegramAPI) lastMessageText(t *testing.T) string {
	t.Helper()
	if len(f.sent) == 0 {
		t.Fatal("no sent messages")
	}
	msg, ok := f.sent[len(f.sent)-1].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("last sent message type = %T, want tgbotapi.MessageConfig", f.sent[len(f.sent)-1])
	}
	return msg.Text
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

	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.AddMangaTitlePlain {
		t.Fatalf("last sent text = %q, want %q", got, appcopy.Copy.Prompts.AddMangaTitlePlain)
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
