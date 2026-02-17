package bot

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/config"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
)

type runTelegramAPI struct {
	updatesCh chan tgbotapi.Update
	sent      []tgbotapi.Chattable
	requested []tgbotapi.Chattable
}

func (f *runTelegramAPI) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return f.updatesCh
}

func (f *runTelegramAPI) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	f.requested = append(f.requested, c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (f *runTelegramAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.sent = append(f.sent, c)
	return tgbotapi.Message{}, nil
}

func (f *runTelegramAPI) StopReceivingUpdates() {}

func (f *runTelegramAPI) lastMessageText(t *testing.T) string {
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

func setupBotForRunTests(t *testing.T, api TelegramAPI) (*Bot, *db.DB) {
	t.Helper()
	database, err := db.New(t.TempDir() + "/test.db")
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

	cfg := &config.Config{AdminUserID: 1}
	return New(api, database, mangadex.NewClient(), cfg, nil), database
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for condition")
}

func TestRun_NonPrivateMessageGetsPrivateOnlyWarning(t *testing.T) {
	api := &runTelegramAPI{updatesCh: make(chan tgbotapi.Update, 4)}
	b, _ := setupBotForRunTests(t, api)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- b.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("bot did not stop in time")
		}
	})

	api.updatesCh <- tgbotapi.Update{
		UpdateID: 1,
		Message: &tgbotapi.Message{
			Text: "hello",
			From: &tgbotapi.User{ID: 42},
			Chat: &tgbotapi.Chat{ID: -100, Type: "group"},
		},
	}

	waitUntil(t, time.Second, func() bool { return len(api.sent) > 0 })
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.PrivateChatOnly {
		t.Fatalf("last message=%q, want %q", got, appcopy.Copy.Prompts.PrivateChatOnly)
	}
}

func TestRun_NonPrivateCallbackGetsPrivateOnlyWarning(t *testing.T) {
	api := &runTelegramAPI{updatesCh: make(chan tgbotapi.Update, 4)}
	b, _ := setupBotForRunTests(t, api)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- b.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("bot did not stop in time")
		}
	})

	api.updatesCh <- tgbotapi.Update{
		UpdateID: 2,
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "cb1",
			Data: "list_manga",
			From: &tgbotapi.User{ID: 42},
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: -100, Type: "group"},
			},
		},
	}

	waitUntil(t, time.Second, func() bool { return len(api.sent) > 0 })
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.PrivateChatOnly {
		t.Fatalf("last message=%q, want %q", got, appcopy.Copy.Prompts.PrivateChatOnly)
	}
}

func TestIsPrivateChat(t *testing.T) {
	if !isPrivateChat(&tgbotapi.Chat{ID: 42}, &tgbotapi.User{ID: 42}) {
		t.Fatal("expected private chat to be true")
	}
	if isPrivateChat(&tgbotapi.Chat{ID: -100}, &tgbotapi.User{ID: 42}) {
		t.Fatal("expected group chat to be false")
	}
	if isPrivateChat(nil, &tgbotapi.User{ID: 42}) {
		t.Fatal("expected nil chat to be false")
	}
	if isPrivateChat(&tgbotapi.Chat{ID: 42}, nil) {
		t.Fatal("expected nil user to be false")
	}
}

func TestIsAuthorized_CacheAndDBPaths(t *testing.T) {
	b, database, _ := setupBotForMessageTests(t)

	const userID = int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	if !b.isAuthorized(userID) {
		t.Fatal("expected authorized user from DB lookup")
	}
	if _, ok := b.authorizedCache[userID]; !ok {
		t.Fatal("expected user to be cached after DB authorization")
	}

	if _, err := database.Exec("DELETE FROM users WHERE chat_id = ?", userID); err != nil {
		t.Fatalf("DELETE user: %v", err)
	}
	if !b.isAuthorized(userID) {
		t.Fatal("expected cached user to remain authorized")
	}

	// Cache hit must still work when DB is unavailable.
	cachedOnly := int64(777)
	b.authorizedCache[cachedOnly] = struct{}{}
	if err := database.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if !b.isAuthorized(cachedOnly) {
		t.Fatal("expected cached-only user to be authorized with closed DB")
	}

	// Admin bypass should also work with DB unavailable.
	if !b.isAuthorized(b.config.AdminUserID) {
		t.Fatal("expected admin to be authorized")
	}
	if b.isAuthorized(99999) {
		t.Fatal("expected unknown uncached user to be unauthorized on DB error")
	}
}

func TestTryHandlePairingCode_InvalidFormatIgnored(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	ok := b.tryHandlePairingCode(&tgbotapi.Message{
		Text: "not a pairing code",
		From: &tgbotapi.User{ID: 42},
		Chat: &tgbotapi.Chat{ID: 42},
	})
	if ok {
		t.Fatal("expected invalid text not to be treated as pairing")
	}
	if len(api.sent) != 0 {
		t.Fatalf("unexpected sent messages: %d", len(api.sent))
	}
}

func TestTryHandlePairingCode_PrivateOnlyGate(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)

	ok := b.tryHandlePairingCode(&tgbotapi.Message{
		Text: "ABCD-1234",
		From: &tgbotapi.User{ID: 42},
		Chat: &tgbotapi.Chat{ID: -100, Type: "group"},
	})
	if !ok {
		t.Fatal("expected group pairing text to be consumed")
	}
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.PairingPrivateOnly {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.PairingPrivateOnly)
	}
}

func TestTryHandlePairingCode_AlreadyAuthorized(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	const userID = int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	ok := b.tryHandlePairingCode(&tgbotapi.Message{
		Text: "ABCD-1234",
		From: &tgbotapi.User{ID: userID},
		Chat: &tgbotapi.Chat{ID: userID},
	})
	if !ok {
		t.Fatal("expected already-authorized code message to be consumed")
	}
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.PairingAlreadyAuth {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.PairingAlreadyAuth)
	}
}

func TestTryHandlePairingCode_InvalidExpiredOrUsed(t *testing.T) {
	tests := []struct {
		name string
		seed func(t *testing.T, d *db.DB, code string)
	}{
		{
			name: "unknown",
			seed: func(t *testing.T, d *db.DB, code string) {},
		},
		{
			name: "expired",
			seed: func(t *testing.T, d *db.DB, code string) {
				t.Helper()
				if err := d.CreatePairingCode(code, 1, time.Now().UTC().Add(-time.Hour)); err != nil {
					t.Fatalf("CreatePairingCode(expired): %v", err)
				}
			},
		},
		{
			name: "used",
			seed: func(t *testing.T, d *db.DB, code string) {
				t.Helper()
				if err := d.CreatePairingCode(code, 1, time.Now().UTC().Add(time.Hour)); err != nil {
					t.Fatalf("CreatePairingCode(used): %v", err)
				}
				ok, err := d.RedeemPairingCode(code, 999)
				if err != nil || !ok {
					t.Fatalf("RedeemPairingCode(seed) = (%v,%v), want (true,nil)", ok, err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, database, api := setupBotForMessageTests(t)
			code := "ABCD-1234"
			tc.seed(t, database, code)

			ok := b.tryHandlePairingCode(&tgbotapi.Message{
				Text: code,
				From: &tgbotapi.User{ID: 42},
				Chat: &tgbotapi.Chat{ID: 42},
			})
			if !ok {
				t.Fatal("expected pairing message to be consumed")
			}
			if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.PairingInvalid {
				t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.PairingInvalid)
			}
		})
	}
}

func TestTryHandlePairingCode_Success(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	const (
		userID = int64(42)
		code   = "ABCD-1234"
	)
	if err := database.CreatePairingCode(code, 1, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("CreatePairingCode(): %v", err)
	}

	ok := b.tryHandlePairingCode(&tgbotapi.Message{
		Text: code,
		From: &tgbotapi.User{ID: userID},
		Chat: &tgbotapi.Chat{ID: userID},
	})
	if !ok {
		t.Fatal("expected pairing message to be consumed")
	}
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.PairingSuccess {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.PairingSuccess)
	}

	authorized, _, err := database.IsUserAuthorized(userID)
	if err != nil {
		t.Fatalf("IsUserAuthorized(): %v", err)
	}
	if !authorized {
		t.Fatal("expected successfully paired user to be authorized")
	}
	if _, ok := b.authorizedCache[userID]; !ok {
		t.Fatal("expected successful pairing to cache authorization")
	}
}

func TestHandleGeneratePairingCode_NonAdminBlocked(t *testing.T) {
	b, _, api := setupBotForMessageTests(t)
	b.handleGeneratePairingCode(42, 42)
	if got := api.lastMessageText(t); got != appcopy.Copy.Prompts.AdminOnly {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Prompts.AdminOnly)
	}
}

func TestHandleGeneratePairingCode_AdminSuccess(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	b.handleGeneratePairingCode(1, 1)

	got := api.lastMessageText(t)
	if !strings.Contains(got, "Pairing code:") {
		t.Fatalf("message=%q, want to contain Pairing code", got)
	}
	if !strings.Contains(got, "https://t.me/ReleaseNoJutsuBot") {
		t.Fatalf("message=%q, want to contain bot link", got)
	}
	if !strings.Contains(got, "How to join:") {
		t.Fatalf("message=%q, want to contain onboarding steps", got)
	}

	rx := regexp.MustCompile(`[A-F0-9]{4}-[A-F0-9]{4}`)
	code := rx.FindString(got)
	if code == "" {
		t.Fatalf("generated message missing pairing code: %q", got)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM pairing_codes WHERE code = ?", code).Scan(&count); err != nil {
		t.Fatalf("pairing code lookup: %v", err)
	}
	if count != 1 {
		t.Fatalf("pairing code row count=%d, want 1", count)
	}
}

func TestHandleGeneratePairingCode_StoreFailure(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	if err := database.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	b.handleGeneratePairingCode(1, 1)
	if got := api.lastMessageText(t); got != appcopy.Copy.Errors.CannotStorePair {
		t.Fatalf("message=%q, want %q", got, appcopy.Copy.Errors.CannotStorePair)
	}
}

func TestParsePairingCode(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{in: "ABCD-1234", want: "ABCD-1234", wantOK: true},
		{in: " abcd-1234 ", want: "ABCD-1234", wantOK: true},
		{in: "abcd 1234", want: "", wantOK: false},
		{in: "ZZZZ-1234", want: "", wantOK: false},
		{in: "ABC-1234", want: "", wantOK: false},
	}

	for _, tc := range tests {
		got, ok := parsePairingCode(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Fatalf("parsePairingCode(%q) = (%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestGeneratePairingCode_Format(t *testing.T) {
	code, err := generatePairingCode()
	if err != nil {
		t.Fatalf("generatePairingCode(): %v", err)
	}
	if len(code) != 9 || code[4] != '-' {
		t.Fatalf("invalid format: %q", code)
	}
	if _, ok := parsePairingCode(strings.ToLower(code)); !ok {
		t.Fatalf("generated code should be parseable, got %q", code)
	}
}
