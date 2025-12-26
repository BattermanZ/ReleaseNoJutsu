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
