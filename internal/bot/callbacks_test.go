package bot

import "testing"

func TestParseCallbackData_MarkUnreadAlias(t *testing.T) {
	tests := []string{"mark_unread", "list_read"}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			payload, err := parseCallbackData(raw)
			if err != nil {
				t.Fatalf("parseCallbackData(%q): %v", raw, err)
			}
			if payload.Kind != callbackMarkUnread {
				t.Fatalf("kind=%v, want %v", payload.Kind, callbackMarkUnread)
			}
		})
	}
}

func TestParseCallbackData_CancelPending(t *testing.T) {
	payload, err := parseCallbackData("cancel_pending")
	if err != nil {
		t.Fatalf("parseCallbackData(cancel_pending): %v", err)
	}
	if payload.Kind != callbackCancelPending {
		t.Fatalf("kind=%v, want %v", payload.Kind, callbackCancelPending)
	}
}

func TestParseCallbackData_SelectMangaBackCompat(t *testing.T) {
	payload, err := parseCallbackData("select_manga:12:menu")
	if err != nil {
		t.Fatalf("parseCallbackData(select_manga): %v", err)
	}
	if payload.Kind != callbackMangaAction {
		t.Fatalf("kind=%v, want %v", payload.Kind, callbackMangaAction)
	}
	if payload.MangaID != 12 || payload.NextAction != "menu" {
		t.Fatalf("payload mismatch: %+v", payload)
	}
}
