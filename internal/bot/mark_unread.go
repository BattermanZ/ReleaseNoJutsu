package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) handleMarkChapterAsUnread(chatID int64, mangaID int, chapterNumber string) {
	b.logAction(chatID, "Mark chapter as unread", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))

	err := b.db.MarkChapterAsUnread(mangaID, chapterNumber)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error marking chapter as unread: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not update the chapter status. Please try again.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	result := fmt.Sprintf("✅ Chapter %s of *%s* is now marked as unread.", chapterNumber, mangaTitle)
	msg := tgbotapi.NewMessage(chatID, result)
	msg.ParseMode = "Markdown"
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadStartMenu(chatID int64, mangaID int) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)

	if readCount == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: 0\n\nNothing to mark unread yet.", mangaTitle, lastReadLine))
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	if readCount <= 10 {
		b.sendMarkUnreadDirectChaptersMenu(chatID, mangaID, readCount, mangaTitle, lastReadLine)
		return
	}

	thousands, err := b.db.ListReadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(thousands) > 1 {
		b.sendMarkUnreadThousandsMenu(chatID, mangaID, thousands, readCount, mangaTitle, lastReadLine, 0)
		return
	}

	thousandStart := 1
	if len(thousands) == 1 {
		thousandStart = thousands[0]
	}
	rs, re := bucketRange(thousandStart, 1000)

	hundreds, err := b.db.ListReadBucketStarts(mangaID, 100, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(hundreds) > 1 {
		b.sendMarkUnreadHundredsMenu(chatID, mangaID, thousandStart, true)
		return
	}

	hundredStart := thousandStart
	if len(hundreds) == 1 {
		hundredStart = hundreds[0]
	}
	rs, re = bucketRange(hundredStart, 100)

	tens, err := b.db.ListReadBucketStarts(mangaID, 10, rs, re)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(tens) > 1 {
		b.sendMarkUnreadTensMenu(chatID, mangaID, hundredStart, true)
		return
	}

	tenStart := hundredStart
	if len(tens) == 1 {
		tenStart = tens[0]
	}
	b.sendMarkUnreadChaptersMenuPage(chatID, mangaID, tenStart, true, 0)
}

func (b *Bot) sendMarkUnreadDirectChaptersMenu(chatID int64, mangaID int, readCount int, mangaTitle, lastReadLine string) {
	chapters, err := b.db.ListReadNumericChaptersInRange(mangaID, 1, 1.0e18, 10, 0)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
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
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("unread_chapter:%d:%s", mangaID, ch.Number)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\n\nSelect a chapter to mark it (and all following ones) as unread:", mangaTitle, lastReadLine, readCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadThousandsMenuPage(chatID int64, mangaID int, page int) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	starts, err := b.db.ListReadBucketStarts(mangaID, 1000, 1, 1.0e18)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing thousand buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	b.sendMarkUnreadThousandsMenu(chatID, mangaID, starts, readCount, mangaTitle, lastReadLine, page)
}

func (b *Bot) sendMarkUnreadThousandsMenu(chatID int64, mangaID int, starts []int, readCount int, mangaTitle, lastReadLine string, page int) {
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
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("mu_pick:%d:%d:%d", mangaID, 1000, bucketStart)))
	}
	keyboard := appendButtonsInRows(nil, buttons, 2)

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mu_page:%d:%d", mangaID, page-1)))
	}
	if page < maxPage {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mu_page:%d:%d", mangaID, page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\n\nPick a range:", mangaTitle, lastReadLine, readCount))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadHundredsMenu(chatID int64, mangaID int, thousandStart int, root bool) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(thousandStart, 1000)

	starts, err := b.db.ListReadBucketStarts(mangaID, 100, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing hundred buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkUnreadTensMenu(chatID, mangaID, starts[0], root)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 100), fmt.Sprintf("mu_pick:%d:%d:%d", mangaID, 100, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mu_back_root:%d", mangaID)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, readCount, bucketLabel(thousandStart, 1000)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadTensMenu(chatID int64, mangaID int, hundredStart int, root bool) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(hundredStart, 100)

	starts, err := b.db.ListReadBucketStarts(mangaID, 10, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing tens buckets: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}
	if len(starts) == 1 {
		b.sendMarkUnreadChaptersMenuPage(chatID, mangaID, starts[0], root, 0)
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	var buttons []tgbotapi.InlineKeyboardButton
	for _, start := range starts {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(bucketLabel(start, 10), fmt.Sprintf("mu_pick:%d:%d:%d", mangaID, 10, start)))
	}
	keyboard = appendButtonsInRows(keyboard, buttons, 2)
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mu_back_hundreds:%d:%d", mangaID, hundredStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\nRange: %s\n\nPick a range:", mangaTitle, lastReadLine, readCount, bucketLabel(hundredStart, 100)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}

func (b *Bot) sendMarkUnreadChaptersMenuPage(chatID int64, mangaID int, tenStart int, root bool, page int) {
	readCount, err := b.db.CountReadChapters(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting read chapters: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
		b.sendMessageWithMainMenuButton(msg)
		return
	}

	mangaTitle, _ := b.db.GetMangaTitle(mangaID)
	lastReadLine := b.lastReadLine(mangaID)
	rangeStart, rangeEnd := bucketRange(tenStart, 10)

	totalInRange, err := b.db.CountReadNumericChaptersInRange(mangaID, rangeStart, rangeEnd)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error counting chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
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
	chapters, err := b.db.ListReadNumericChaptersInRange(mangaID, rangeStart, rangeEnd, pageSize, page*pageSize)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error listing chapters in range: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Could not load read chapters right now.")
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
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("unread_chapter:%d:%s", mangaID, ch.Number)),
		})
	}

	// Pagination.
	var nav []tgbotapi.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("mu_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page-1)))
	}
	if (page+1)*pageSize < totalInRange {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("mu_chpage:%d:%d:%d:%d", mangaID, tenStart, boolToInt(root), page+1)))
	}
	if len(nav) > 0 {
		keyboard = append(keyboard, nav)
	}
	if !root {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("mu_back_tens:%d:%d", mangaID, tenStart)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 %s\n\n%s\nRead: %d\nRange: %s\n\nSelect a chapter to mark it (and all following ones) as unread:", mangaTitle, lastReadLine, readCount, bucketLabel(tenStart, 10)))
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	b.sendMessageWithMainMenuButton(msg)
}
