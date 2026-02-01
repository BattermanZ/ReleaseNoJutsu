package updater

import (
	"fmt"
	"html"
	"strings"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/mangadex"
)

func FormatNewChaptersMessageHTML(mangaTitle string, newChapters []mangadex.ChapterInfo, unreadCount int, warnOnThreePlus bool) string {
	var b strings.Builder
	b.WriteString(appcopy.Copy.Info.NewChapterAlertTitle)
	b.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertHeader, html.EscapeString(mangaTitle)))
	for _, chapter := range newChapters {
		// Keep chapter number unescaped for readability, but escape anyway to be safe.
		label := fmt.Sprintf(appcopy.Copy.Labels.ChapterPrefix, html.EscapeString(chapter.Number))
		b.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertItem, label, html.EscapeString(chapter.Title)))
	}
	b.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertUnread, unreadCount))
	if warnOnThreePlus && unreadCount >= 3 {
		b.WriteString(appcopy.Copy.Info.NewChapterAlertWarning)
	}
	b.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertFooter, appcopy.Copy.Commands.Start))
	return b.String()
}
