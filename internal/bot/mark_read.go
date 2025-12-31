package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) handleMarkChapterAsRead(chatID int64, mangaID int, chapterNumber string) {
	b.logAction(chatID, "Mark chapter as read", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))

	err := b.db.MarkChapterAsRead(mangaID, chapterNumber)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error marking chapters as read: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not update the chapter status. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	result := fmt.Sprintf("✅ Updated!\nAll chapters up to Chapter %s of *%s* have been marked as read.", chapterNumber, mangaTitle)
	msg := tgbotapi.NewMessage(chatID, result)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadStartMenu(chatID int64, mangaID int) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)

	if unreadCount == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: 0\n\n✅ You're up to date.", mangaTitle, lastReadLine))
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	if unreadCount <= 10 {
		b.sendMarkReadDirectChaptersMenu(chatID, mangaID, unreadCount, mangaTitle, lastReadLine)
		return
	}

	thousands, err := b.db.ListUnreadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(thousands) > 1 {
		b.sendMarkReadThousandsMenu(chatID, mangaID, thousands, unreadCount, mangaTitle, lastReadLine, 0)
		return
	}

	thousandStart := 1
	if len(thousands) == 1 {
		thousandStart = thousands[0]
	}
	rs, re := bucketRange(thousandStart, 1000)

	hundreds, err := b.db.ListUnreadBucketStarts(mangaID, 100, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(hundreds) > 1 {
		b.sendMarkReadHundredsMenu(chatID, mangaID, thousandStart, true)
		return
	}

	hundredStart := thousandStart
	if len(hundreds) == 1 {
		hundredStart = hundreds[0]
	}
	rs, re = bucketRange(hundredStart, 100)

	tens, err := b.db.ListUnreadBucketStarts(mangaID, 10, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(tens) > 1 {
		b.sendMarkReadTensMenu(chatID, mangaID, hundredStart, true)
		return
	}

	tenStart := hundredStart
	if len(tens) == 1 {
		tenStart = tens[0]
	}
	b.sendMarkReadChaptersMenuPage(chatID, mangaID, tenStart, true, 0)
}

func (b *Bot) sendMarkReadDirectChaptersMenu(chatID int64, mangaID int, unreadCount int, mangaTitle, lastReadLine string) {
	chapters, err := b.db.ListUnreadChapters(mangaID, 10, 0)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, ch := range chapters {
		label := fmt.Sprintf("Ch. %s", ch.Number)
		if strings.TrimSpace(ch.Title) != "" {
			label = fmt.Sprintf("Ch. %s: %s", ch.Number, ch.Title)
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mark_chapter:%d:%s", mangaID, ch.Number)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\n\nSelect a chapter to mark it (and all previous ones) as read:", mangaTitle, lastReadLine, unreadCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadThousandsMenuPage(chatID int64, mangaID int, page int) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	starts, err := b.db.ListUnreadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	b.sendMarkReadThousandsMenu(chatID, mangaID, starts, unreadCount, mangaTitle, lastReadLine, page)
}

func (b *Bot) sendMarkReadThousandsMenu(chatID int64, mangaID int, starts []int, unreadCount int, mangaTitle, lastReadLine string, page int) {
	const pageSize = 24
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if len(starts) > 0 {
		maxPage = (len(starts) - 1) / pageSize
	}
	if page > maxPage {
		page = maxPage
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(starts) {
		end = len(starts)
	}

	buttons := make([]tgbotapi.InlineKeyboardButton, 0, end-start)
	for _, bucketStart := range starts[start:end] {
		label := bucketLabel(bucketStart, 1000)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 1000, bucketStart)))
	}
	keyboard := appendButtonsInRows(nil, buttons, 2)

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mr_page:%d:%d", mangaID, page-1)))
	}
	if page < maxPage {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mr_page:%d:%d", mangaID, page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\n\nPick a range:", mangaTitle, lastReadLine, unreadCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadHundredsMenu(chatID int64, mangaID int, thousandStart int, root bool) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(thousandStart, 1000)

	starts, err := b.db.ListUnreadBucketStarts(mangaID, 100, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkReadTensMenu(chatID, mangaID, starts[0], root)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 100), fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 100, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mr_back_root:%d", mangaID)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, unreadCount, bucketLabel(thousandStart, 1000)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadTensMenu(chatID int64, mangaID int, hundredStart int, root bool) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(hundredStart, 100)

	starts, err := b.db.ListUnreadBucketStarts(mangaID, 10, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkReadChaptersMenuPage(chatID, mangaID, starts[0], root, 0)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 10), fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, 10, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mr_back_hundreds:%d:%d", mangaID, hundredStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, unreadCount, bucketLabel(hundredStart, 100)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkReadChaptersMenuPage(chatID int64, mangaID int, tenStart int, root bool, page int) {
	unreadCount, err := b.db.CountUnreadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting unread chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(tenStart, 10)

	totalInRange, err := b.db.CountUnreadNumericChaptersInRange(mangaID, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	const pageSize = 30
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if totalInRange > 0 {
		maxPage = (totalInRange - 1) / pageSize
	}
	if page > maxPage {
		page = maxPage
	}
	chapters, err := b.db.ListUnreadNumericChaptersInRange(mangaID, rangeStart, rangeEnd, pageSize, page*pageSize)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load unread chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, ch := range chapters {
		label := fmt.Sprintf("Ch. %s", ch.Number)
		if strings.TrimSpace(ch.Title) != "" {
			label = fmt.Sprintf("Ch. %s: %s", ch.Number, ch.Title)
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mark_chapter:%d:%s", mangaID, ch.Number)),
		})
	}

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mr_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page-1)))
	}
	if (page+1)*pageSize < totalInRange {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mr_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
	}
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mr_back_tens:%d:%d", mangaID, tenStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nSelect a chapter to mark it (and all previous ones) as read:", mangaTitle, lastReadLine, unreadCount, bucketLabel(tenStart, 10)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}
