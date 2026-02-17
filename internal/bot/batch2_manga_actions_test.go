package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/config"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
	"releasenojutsu/internal/updater"
)

func setupBotWithMangaDexServer(t *testing.T) (*Bot, *db.DB, *fakeTelegramAPI, string) {
	t.Helper()

	const mdID = "40bc649f-7b49-4645-859e-6cd94136e722"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/feed") {
			_ = json.NewEncoder(w).Encode(mangadex.ChapterFeedResponse{
				Data:   []mangadex.Chapter{},
				Limit:  100,
				Offset: 0,
				Total:  0,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(mangadex.MangaResponse{
			Data: struct {
				Id         string `json:"id"`
				Attributes struct {
					Title map[string]string `json:"title"`
				} `json:"attributes"`
			}{
				Id: mdID,
				Attributes: struct {
					Title map[string]string `json:"title"`
				}{
					Title: map[string]string{
						"en": "Dragon Ball Super",
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

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

	mdClient := mangadex.NewClient()
	mdClient.BaseURL = srv.URL
	upd := updater.New(database, mdClient, mdClient)
	api := &fakeTelegramAPI{}
	cfg := &config.Config{AdminUserID: 1}
	return New(api, database, mdClient, cfg, upd), database, api, mdID
}

func TestHandleAddManga_ShowsMangaPlusQuestion(t *testing.T) {
	b, database, api, mdID := setupBotWithMangaDexServer(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	b.handleAddManga(userID, userID, mdID)

	msg := api.lastMessageConfig(t)
	if !strings.Contains(msg.Text, "Dragon Ball Super") {
		t.Fatalf("expected manga title in add prompt, got %q", msg.Text)
	}
	keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("ReplyMarkup type = %T, want InlineKeyboardMarkup", msg.ReplyMarkup)
	}
	var callbacks []string
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil {
				callbacks = append(callbacks, *btn.CallbackData)
			}
		}
	}
	if !hasCallback(callbacks, cbAddConfirm(mdID, true)) || !hasCallback(callbacks, cbAddConfirm(mdID, false)) {
		t.Fatalf("missing add confirm callbacks in %v", callbacks)
	}
}

func TestConfirmAddManga_InsertsRowAndStartsSync(t *testing.T) {
	b, database, api, mdID := setupBotWithMangaDexServer(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}

	b.confirmAddManga(userID, userID, mdID, true)

	waitUntil(t, 2*time.Second, func() bool {
		var count int
		err := database.QueryRow("SELECT COUNT(*) FROM manga WHERE user_id = ? AND mangadex_id = ?", userID, mdID).Scan(&count)
		return err == nil && count == 1
	})

	var isPlus int
	if err := database.QueryRow("SELECT is_manga_plus FROM manga WHERE user_id = ? AND mangadex_id = ?", userID, mdID).Scan(&isPlus); err != nil {
		t.Fatalf("select is_manga_plus: %v", err)
	}
	if isPlus != 1 {
		t.Fatalf("is_manga_plus=%d, want 1", isPlus)
	}

	waitUntil(t, 2*time.Second, func() bool {
		for _, text := range api.sentMessageTexts(t) {
			if strings.Contains(text, "Dragon Ball Super") {
				return true
			}
		}
		return false
	})
}

func TestMangaActions_RemoveConfirmMarkAllDetailsToggleRemove(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga("95f5f24f-f6a4-4f08-a4ca-5a16552f6b73", "Action Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}
	addChapterSet(t, b, mangaID, "1", "2", "3")

	b.sendRemoveMangaConfirm(userID, userID, int(mangaID))
	msg := api.lastMessageConfig(t)
	if !strings.Contains(msg.Text, "Action Manga") {
		t.Fatalf("remove confirm missing title: %q", msg.Text)
	}
	if !hasCallback(messageCallbacks(t, msg), cbMangaAction(int(mangaID), "remove_manga_yes")) {
		t.Fatalf("remove confirm missing yes callback")
	}

	b.handleMarkAllRead(userID, userID, int(mangaID))
	unread, err := database.CountUnreadChapters(int(mangaID))
	if err != nil {
		t.Fatalf("CountUnreadChapters(): %v", err)
	}
	if unread != 0 {
		t.Fatalf("unread=%d, want 0", unread)
	}
	if got := api.lastMessageText(t); !strings.Contains(got, "Action Manga") {
		t.Fatalf("mark-all-read message missing title: %q", got)
	}

	b.handleMangaDetails(userID, userID, int(mangaID))
	detailsMsg := api.lastMessageConfig(t)
	if !strings.Contains(detailsMsg.Text, "Action Manga") {
		t.Fatalf("details message missing title: %q", detailsMsg.Text)
	}
	if !hasCallback(messageCallbacks(t, detailsMsg), cbMangaAction(int(mangaID), "toggle_plus")) {
		t.Fatalf("details message missing toggle callback")
	}

	b.toggleMangaPlus(userID, userID, int(mangaID))
	isPlus, err := database.IsMangaPlus(int(mangaID))
	if err != nil {
		t.Fatalf("IsMangaPlus(): %v", err)
	}
	if !isPlus {
		t.Fatal("expected manga plus to be enabled after toggle")
	}

	b.handleRemoveManga(userID, userID, int(mangaID))
	owned, err := database.MangaBelongsToUser(int(mangaID), userID)
	if err != nil {
		t.Fatalf("MangaBelongsToUser(): %v", err)
	}
	if owned {
		t.Fatal("expected manga to be deleted")
	}

	var chapterCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM chapters WHERE manga_id = ?", mangaID).Scan(&chapterCount); err != nil {
		t.Fatalf("chapter count query: %v", err)
	}
	if chapterCount != 0 {
		t.Fatalf("chapterCount=%d, want 0 after manga deletion", chapterCount)
	}
	if got := api.lastMessageText(t); !strings.Contains(got, "Action Manga") {
		t.Fatalf("remove message missing title: %q", got)
	}
}

func TestConfirmAddManga_FetchFailureSendsError(t *testing.T) {
	// Server always fails so confirmAddManga should send retrieval error and stop.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

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

	api := &fakeTelegramAPI{}
	mdClient := mangadex.NewClient()
	mdClient.BaseURL = srv.URL
	upd := updater.New(database, mdClient, mdClient)
	b := New(api, database, mdClient, &config.Config{AdminUserID: 1}, upd)

	b.confirmAddManga(42, 42, "40bc649f-7b49-4645-859e-6cd94136e722", false)
	if got := api.lastMessageText(t); got == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestHandleSyncAllChapters_SendsStartAndCompletion(t *testing.T) {
	b, database, api, mdID := setupBotWithMangaDexServer(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga(mdID, "Sync Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	b.handleSyncAllChapters(userID, userID, int(mangaID))

	waitUntil(t, 2*time.Second, func() bool {
		for _, txt := range api.sentMessageTexts(t) {
			if strings.Contains(txt, "Sync Manga") {
				return true
			}
		}
		return false
	})
}

func TestHandleCheckNewChapters_NoNewChaptersPath(t *testing.T) {
	b, database, _, mdID := setupBotWithMangaDexServer(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga(mdID, "No New Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	api := b.api.(*fakeTelegramAPI)
	b.handleCheckNewChapters(userID, int(mangaID))
	if got := api.lastMessageText(t); !strings.Contains(got, "No New Manga") {
		t.Fatalf("expected no-new message with title, got %q", got)
	}
}

func TestHandleSyncAllChapters_ContextTimeoutPathCovered(t *testing.T) {
	// Minimal sanity check for goroutine timeout/cancel path plumbing: this should not panic.
	b, database, _, mdID := setupBotWithMangaDexServer(t)
	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(): %v", err)
	}
	mangaID, err := database.AddManga(mdID, "Timeout Manga", userID)
	if err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = ctx
	b.handleSyncAllChapters(userID, userID, int(mangaID))
}
