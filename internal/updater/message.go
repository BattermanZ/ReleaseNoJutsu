package updater

import (
	"fmt"
	"html"
	"strings"

	"releasenojutsu/internal/mangadex"
)

func FormatNewChaptersMessageHTML(mangaTitle string, newChapters []mangadex.ChapterInfo, unreadCount int) string {
	var b strings.Builder
	b.WriteString("📢 <b>New Chapter Alert!</b>\n\n")
	b.WriteString(fmt.Sprintf("<b>%s</b> has new chapters:\n", html.EscapeString(mangaTitle)))
	for _, chapter := range newChapters {
		// Keep chapter number unescaped for readability, but escape anyway to be safe.
		b.WriteString(fmt.Sprintf("• <b>Ch. %s</b>: %s\n", html.EscapeString(chapter.Number), html.EscapeString(chapter.Title)))
	}
	b.WriteString(fmt.Sprintf("\nYou now have <b>%d</b> unread chapter(s) for this series.\n", unreadCount))
	if unreadCount >= 3 {
		b.WriteString("\n⚠️ <b>Warning:</b> You have 3 or more unread chapters for this manga!")
	}
	b.WriteString("\nUse /start to mark chapters as read or explore other options.")
	return b.String()
}
