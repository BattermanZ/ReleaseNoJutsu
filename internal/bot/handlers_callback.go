package bot

import (
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"releasenojutsu/internal/logger"
)

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	b.logAction(query.From.ID, "Received callback query", query.Data)

	parts := strings.Split(query.Data, ":")
	action := parts[0]

	switch action {
	case "add_confirm":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for add_confirm: %s", query.Data)
			return
		}
		mangaDexID := parts[1]
		isPlusInt, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting isMangaPlus flag: %v", err)
			return
		}
		b.confirmAddManga(query.Message.Chat.ID, mangaDexID, isPlusInt != 0)
	case "add_manga":
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, `📚 *Add a New Manga*
Please send the MangaDex URL or ID of the manga you want to track.`)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: "MangaDex ID"}
		b.sendMessageWithMainMenuButton(msg)
	case "list_manga":
		b.handleListManga(query.Message.Chat.ID)
	case "check_new":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "check_new")
	case "mark_read":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "mark_read")
	case "list_read":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "list_read")
	case "sync_all":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "sync_all")
	case "select_manga": // This case is for manga selection menus (check_new, mark_read, list_read, remove_manga)
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for select_manga: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		nextAction := parts[2]
		b.handleMangaSelection(query.Message.Chat.ID, mangaID, nextAction)
	case "manga_action": // This case is for actions directly from the manga list
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for manga_action: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		action := parts[2]
		b.handleMangaSelection(query.Message.Chat.ID, mangaID, action)
	case "mark_chapter":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mark_chapter: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		chapterNumber := parts[2]
		b.handleMarkChapterAsRead(query.Message.Chat.ID, mangaID, chapterNumber)
	case "mr_pick":
		if len(parts) < 4 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_pick: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		scale, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting scale: %v", err)
			return
		}
		start, err := strconv.Atoi(parts[3])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting start: %v", err)
			return
		}
		switch scale {
		case 1000:
			b.sendMarkReadHundredsMenu(query.Message.Chat.ID, mangaID, start, false)
		case 100:
			b.sendMarkReadTensMenu(query.Message.Chat.ID, mangaID, start, false)
		case 10:
			b.sendMarkReadChaptersMenuPage(query.Message.Chat.ID, mangaID, start, false, 0)
		default:
			logger.LogMsg(logger.LogError, "Unknown mr_pick scale: %d", scale)
		}
	case "mr_page":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_page: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkReadThousandsMenuPage(query.Message.Chat.ID, mangaID, page)
	case "mr_chpage":
		if len(parts) < 5 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_chpage: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		tenStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting tenStart: %v", err)
			return
		}
		rootInt, err := strconv.Atoi(parts[3])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting root flag: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[4])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkReadChaptersMenuPage(query.Message.Chat.ID, mangaID, tenStart, intToBool(rootInt), page)
	case "mr_back_root":
		if len(parts) < 2 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_back_root: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		b.sendMarkReadStartMenu(query.Message.Chat.ID, mangaID)
	case "mr_back_hundreds":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_back_hundreds: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		hundredStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting hundredStart: %v", err)
			return
		}
		b.sendMarkReadHundredsMenu(query.Message.Chat.ID, mangaID, thousandBucketStart(hundredStart), false)
	case "mr_back_tens":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mr_back_tens: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		tenStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting tenStart: %v", err)
			return
		}
		b.sendMarkReadTensMenu(query.Message.Chat.ID, mangaID, hundredBucketStart(tenStart), false)
	case "mu_pick":
		if len(parts) < 4 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_pick: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		scale, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting scale: %v", err)
			return
		}
		start, err := strconv.Atoi(parts[3])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting start: %v", err)
			return
		}
		switch scale {
		case 1000:
			b.sendMarkUnreadHundredsMenu(query.Message.Chat.ID, mangaID, start, false)
		case 100:
			b.sendMarkUnreadTensMenu(query.Message.Chat.ID, mangaID, start, false)
		case 10:
			b.sendMarkUnreadChaptersMenuPage(query.Message.Chat.ID, mangaID, start, false, 0)
		default:
			logger.LogMsg(logger.LogError, "Unknown mu_pick scale: %d", scale)
		}
	case "mu_page":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_page: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkUnreadThousandsMenuPage(query.Message.Chat.ID, mangaID, page)
	case "mu_chpage":
		if len(parts) < 5 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_chpage: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		tenStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting tenStart: %v", err)
			return
		}
		rootInt, err := strconv.Atoi(parts[3])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting root flag: %v", err)
			return
		}
		page, err := strconv.Atoi(parts[4])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting page: %v", err)
			return
		}
		b.sendMarkUnreadChaptersMenuPage(query.Message.Chat.ID, mangaID, tenStart, intToBool(rootInt), page)
	case "mu_back_root":
		if len(parts) < 2 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_back_root: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		b.sendMarkUnreadStartMenu(query.Message.Chat.ID, mangaID)
	case "mu_back_hundreds":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_back_hundreds: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		hundredStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting hundredStart: %v", err)
			return
		}
		b.sendMarkUnreadHundredsMenu(query.Message.Chat.ID, mangaID, thousandBucketStart(hundredStart), false)
	case "mu_back_tens":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for mu_back_tens: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		tenStart, err := strconv.Atoi(parts[2])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting tenStart: %v", err)
			return
		}
		b.sendMarkUnreadTensMenu(query.Message.Chat.ID, mangaID, hundredBucketStart(tenStart), false)
	case "unread_chapter":
		if len(parts) < 3 {
			logger.LogMsg(logger.LogError, "Invalid callback data for unread_chapter: %s", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.LogMsg(logger.LogError, "Error converting manga ID: %v", err)
			return
		}
		chapterNumber := parts[2]
		b.handleMarkChapterAsUnread(query.Message.Chat.ID, mangaID, chapterNumber)
	case "remove_manga":
		b.sendMangaSelectionMenu(query.Message.Chat.ID, "remove_manga")
	case "main_menu":
		b.sendMainMenu(query.Message.Chat.ID)
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		logger.LogMsg(logger.LogError, "Error answering callback query: %v", err)
	}
}
