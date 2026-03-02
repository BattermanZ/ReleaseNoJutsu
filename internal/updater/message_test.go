package updater

import (
	"strings"
	"testing"

	"releasenojutsu/internal/mangadex"
)

func TestFormatNewChaptersMessageHTML_EscapesDynamicContent(t *testing.T) {
	msg := FormatNewChaptersMessageHTML(
		`My <b>manga</b> & friends`,
		[]mangadex.ChapterInfo{
			{Number: `1`, Title: `Title with <script>alert(1)</script> & stuff`},
		},
		1,
		false,
	)

	if strings.Contains(msg, "<script>") {
		t.Fatalf("message should escape HTML, got: %q", msg)
	}
	if !strings.Contains(msg, "&lt;b&gt;manga&lt;/b&gt;") {
		t.Fatalf("expected escaped title, got: %q", msg)
	}
	if !strings.Contains(msg, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("expected escaped chapter title, got: %q", msg)
	}
}

func TestFormatNewChaptersMessage_PlainTextIncludesWarningAndFooter(t *testing.T) {
	msg := FormatNewChaptersMessage(
		"Dragon Ball",
		[]mangadex.ChapterInfo{
			{Number: "104", Title: "The Birth of Saiyaman X"},
		},
		3,
		true,
	)

	if !strings.Contains(msg, "Dragon Ball has new chapters:") {
		t.Fatalf("missing plain header in message: %q", msg)
	}
	if !strings.Contains(msg, "Ch. 104") {
		t.Fatalf("missing chapter number label: %q", msg)
	}
	if !strings.Contains(msg, "3+ unread chapters piling up") {
		t.Fatalf("missing warning line: %q", msg)
	}
	if !strings.Contains(msg, "Use /start to open the menu") {
		t.Fatalf("missing footer line: %q", msg)
	}
}
