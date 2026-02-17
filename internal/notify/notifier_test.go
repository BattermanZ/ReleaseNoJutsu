package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestTelegramNotifier_SendHTML(t *testing.T) {
	const token = "TEST_TOKEN"

	var gotSendMessage bool
	var gotChatID string
	var gotParseMode string
	var gotText string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": map[string]any{
					"id":         1,
					"is_bot":     true,
					"first_name": "bot",
					"username":   "bot",
				},
			})
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm(): %v", err)
			}
			gotSendMessage = true
			gotChatID = r.PostForm.Get("chat_id")
			gotParseMode = r.PostForm.Get("parse_mode")
			gotText = r.PostForm.Get("text")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": map[string]any{
					"message_id": 1,
					"date":       0,
					"chat": map[string]any{
						"id":   42,
						"type": "private",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	api, err := tgbotapi.NewBotAPIWithAPIEndpoint(token, srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatalf("NewBotAPIWithAPIEndpoint(): %v", err)
	}

	n := NewTelegramNotifier(api)
	wantText := "<b>Hello</b>"
	if err := n.SendHTML(42, wantText); err != nil {
		t.Fatalf("SendHTML(): %v", err)
	}

	if !gotSendMessage {
		t.Fatal("expected /sendMessage call")
	}
	if gotChatID != "42" {
		t.Fatalf("chat_id=%q, want 42", gotChatID)
	}
	if !strings.EqualFold(gotParseMode, "HTML") {
		t.Fatalf("parse_mode=%q, want HTML", gotParseMode)
	}
	if decoded, _ := url.QueryUnescape(gotText); decoded != wantText && gotText != wantText {
		t.Fatalf("text=%q, want %q", gotText, wantText)
	}
}
