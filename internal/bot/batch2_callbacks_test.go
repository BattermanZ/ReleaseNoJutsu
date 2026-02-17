package bot

import "testing"

func TestParseCallbackData_BuilderRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want callbackPayload
	}{
		{
			name: "add confirm plus",
			raw:  cbAddConfirm("40bc649f-7b49-4645-859e-6cd94136e722", true),
			want: callbackPayload{Kind: callbackAddConfirm, MangaDexID: "40bc649f-7b49-4645-859e-6cd94136e722", IsMangaPlus: true},
		},
		{
			name: "add confirm no plus",
			raw:  cbAddConfirm("40bc649f-7b49-4645-859e-6cd94136e722", false),
			want: callbackPayload{Kind: callbackAddConfirm, MangaDexID: "40bc649f-7b49-4645-859e-6cd94136e722", IsMangaPlus: false},
		},
		{name: "gen pair", raw: cbGenPair(), want: callbackPayload{Kind: callbackGenPair}},
		{name: "main menu", raw: cbMainMenu(), want: callbackPayload{Kind: callbackMainMenu}},
		{name: "cancel pending", raw: cbCancelPending(), want: callbackPayload{Kind: callbackCancelPending}},
		{name: "manga action", raw: cbMangaAction(12, "menu"), want: callbackPayload{Kind: callbackMangaAction, MangaID: 12, NextAction: "menu"}},
		{name: "mark read chapter", raw: cbMarkChapterRead(9, "10.5"), want: callbackPayload{Kind: callbackMarkChapterRead, MangaID: 9, ChapterNumber: "10.5"}},
		{name: "mark unread chapter", raw: cbMarkChapterUnread(9, "10.5"), want: callbackPayload{Kind: callbackMarkChapterUnread, MangaID: 9, ChapterNumber: "10.5"}},
		{name: "mr pick", raw: cbMarkReadPick(5, 100, 200), want: callbackPayload{Kind: callbackMarkReadPick, MangaID: 5, Scale: 100, Start: 200}},
		{name: "mr page", raw: cbMarkReadPage(5, 3), want: callbackPayload{Kind: callbackMarkReadPage, MangaID: 5, Page: 3}},
		{name: "mr chapter page root", raw: cbMarkReadChapterPage(5, 210, true, 1), want: callbackPayload{Kind: callbackMarkReadChapterPage, MangaID: 5, Start: 210, Root: true, Page: 1}},
		{name: "mr back root", raw: cbMarkReadBackRoot(5), want: callbackPayload{Kind: callbackMarkReadBackRoot, MangaID: 5}},
		{name: "mr back hundreds", raw: cbMarkReadBackHundreds(5, 200), want: callbackPayload{Kind: callbackMarkReadBackHundreds, MangaID: 5, Start: 200}},
		{name: "mr back tens", raw: cbMarkReadBackTens(5, 210), want: callbackPayload{Kind: callbackMarkReadBackTens, MangaID: 5, Start: 210}},
		{name: "mu pick", raw: cbMarkUnreadPick(5, 100, 200), want: callbackPayload{Kind: callbackMarkUnreadPick, MangaID: 5, Scale: 100, Start: 200}},
		{name: "mu page", raw: cbMarkUnreadPage(5, 3), want: callbackPayload{Kind: callbackMarkUnreadPage, MangaID: 5, Page: 3}},
		{name: "mu chapter page root", raw: cbMarkUnreadChapterPage(5, 210, true, 1), want: callbackPayload{Kind: callbackMarkUnreadChapterPage, MangaID: 5, Start: 210, Root: true, Page: 1}},
		{name: "mu back root", raw: cbMarkUnreadBackRoot(5), want: callbackPayload{Kind: callbackMarkUnreadBackRoot, MangaID: 5}},
		{name: "mu back hundreds", raw: cbMarkUnreadBackHundreds(5, 200), want: callbackPayload{Kind: callbackMarkUnreadBackHundreds, MangaID: 5, Start: 200}},
		{name: "mu back tens", raw: cbMarkUnreadBackTens(5, 210), want: callbackPayload{Kind: callbackMarkUnreadBackTens, MangaID: 5, Start: 210}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCallbackData(tc.raw)
			if err != nil {
				t.Fatalf("parseCallbackData(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("payload mismatch: got=%+v want=%+v", got, tc.want)
			}
		})
	}
}

func TestParseCallbackData_InvalidShapes(t *testing.T) {
	tests := []string{
		"",
		"unknown_action",
		"manga_action",
		"manga_action:x:menu",
		"mark_chapter:1",
		"unread_chapter:1",
		"mr_pick:1:10",
		"mr_pick:1:a:10",
		"mr_page:1",
		"mr_page:a:1",
		"mr_chpage:1:10:1",
		"mr_chpage:1:10:x:1",
		"mr_back_root",
		"mr_back_hundreds:1",
		"mu_pick:1:10",
		"mu_page:a:1",
		"mu_chpage:1:10:x:1",
		"mu_back_tens:1",
		"add_confirm:only-id",
		"add_confirm:some-id:bad",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseCallbackData(raw); err == nil {
				t.Fatalf("parseCallbackData(%q) expected error", raw)
			}
		})
	}
}
