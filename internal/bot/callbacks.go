package bot

import (
	"fmt"
	"strconv"
	"strings"
)

type callbackKind int

const (
	callbackUnknown callbackKind = iota
	callbackAddConfirm
	callbackAddManga
	callbackListManga
	callbackCheckNew
	callbackMarkRead
	callbackMarkUnread
	callbackSyncAll
	callbackGenPair
	callbackSelectManga
	callbackMangaAction
	callbackMarkChapterRead
	callbackMarkChapterUnread
	callbackMarkReadPick
	callbackMarkReadPage
	callbackMarkReadChapterPage
	callbackMarkReadBackRoot
	callbackMarkReadBackHundreds
	callbackMarkReadBackTens
	callbackMarkUnreadPick
	callbackMarkUnreadPage
	callbackMarkUnreadChapterPage
	callbackMarkUnreadBackRoot
	callbackMarkUnreadBackHundreds
	callbackMarkUnreadBackTens
	callbackRemoveManga
	callbackMainMenu
)

type callbackPayload struct {
	Kind          callbackKind
	MangaID       int
	MangaDexID    string
	IsMangaPlus   bool
	NextAction    string
	ChapterNumber string
	Scale         int
	Start         int
	Page          int
	Root          bool
}

func parseCallbackData(raw string) (callbackPayload, error) {
	parts := strings.Split(raw, ":")
	if len(parts) == 0 || parts[0] == "" {
		return callbackPayload{}, fmt.Errorf("empty callback data")
	}

	switch parts[0] {
	case "add_confirm":
		if len(parts) != 3 {
			return callbackPayload{}, fmt.Errorf("invalid add_confirm callback: %s", raw)
		}
		isPlusInt, err := strconv.Atoi(parts[2])
		if err != nil {
			return callbackPayload{}, fmt.Errorf("invalid add_confirm flag: %w", err)
		}
		return callbackPayload{
			Kind:        callbackAddConfirm,
			MangaDexID:  parts[1],
			IsMangaPlus: isPlusInt != 0,
		}, nil
	case "add_manga":
		return callbackPayload{Kind: callbackAddManga}, nil
	case "list_manga":
		return callbackPayload{Kind: callbackListManga}, nil
	case "check_new":
		return callbackPayload{Kind: callbackCheckNew}, nil
	case "mark_read":
		return callbackPayload{Kind: callbackMarkRead}, nil
	case "list_read":
		return callbackPayload{Kind: callbackMarkUnread}, nil
	case "sync_all":
		return callbackPayload{Kind: callbackSyncAll}, nil
	case "gen_pair":
		return callbackPayload{Kind: callbackGenPair}, nil
	case "select_manga":
		if len(parts) != 3 {
			return callbackPayload{}, fmt.Errorf("invalid select_manga callback: %s", raw)
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
		}
		return callbackPayload{
			Kind:       callbackSelectManga,
			MangaID:    mangaID,
			NextAction: parts[2],
		}, nil
	case "manga_action":
		if len(parts) != 3 {
			return callbackPayload{}, fmt.Errorf("invalid manga_action callback: %s", raw)
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
		}
		return callbackPayload{
			Kind:       callbackMangaAction,
			MangaID:    mangaID,
			NextAction: parts[2],
		}, nil
	case "mark_chapter":
		if len(parts) != 3 {
			return callbackPayload{}, fmt.Errorf("invalid mark_chapter callback: %s", raw)
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
		}
		return callbackPayload{
			Kind:          callbackMarkChapterRead,
			MangaID:       mangaID,
			ChapterNumber: parts[2],
		}, nil
	case "unread_chapter":
		if len(parts) != 3 {
			return callbackPayload{}, fmt.Errorf("invalid unread_chapter callback: %s", raw)
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
		}
		return callbackPayload{
			Kind:          callbackMarkChapterUnread,
			MangaID:       mangaID,
			ChapterNumber: parts[2],
		}, nil
	case "mr_pick":
		return parsePick(raw, parts, callbackMarkReadPick)
	case "mu_pick":
		return parsePick(raw, parts, callbackMarkUnreadPick)
	case "mr_page":
		return parsePage(raw, parts, callbackMarkReadPage)
	case "mu_page":
		return parsePage(raw, parts, callbackMarkUnreadPage)
	case "mr_chpage":
		return parseChapterPage(raw, parts, callbackMarkReadChapterPage)
	case "mu_chpage":
		return parseChapterPage(raw, parts, callbackMarkUnreadChapterPage)
	case "mr_back_root":
		return parseSingleManga(raw, parts, callbackMarkReadBackRoot)
	case "mu_back_root":
		return parseSingleManga(raw, parts, callbackMarkUnreadBackRoot)
	case "mr_back_hundreds":
		return parseStartBack(raw, parts, callbackMarkReadBackHundreds)
	case "mu_back_hundreds":
		return parseStartBack(raw, parts, callbackMarkUnreadBackHundreds)
	case "mr_back_tens":
		return parseStartBack(raw, parts, callbackMarkReadBackTens)
	case "mu_back_tens":
		return parseStartBack(raw, parts, callbackMarkUnreadBackTens)
	case "remove_manga":
		return callbackPayload{Kind: callbackRemoveManga}, nil
	case "main_menu":
		return callbackPayload{Kind: callbackMainMenu}, nil
	default:
		return callbackPayload{Kind: callbackUnknown}, fmt.Errorf("unknown callback action: %s", parts[0])
	}
}

func parsePick(raw string, parts []string, kind callbackKind) (callbackPayload, error) {
	if len(parts) != 4 {
		return callbackPayload{}, fmt.Errorf("invalid pick callback: %s", raw)
	}
	mangaID, err := strconv.Atoi(parts[1])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
	}
	scale, err := strconv.Atoi(parts[2])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid scale: %w", err)
	}
	start, err := strconv.Atoi(parts[3])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid start: %w", err)
	}
	return callbackPayload{
		Kind:    kind,
		MangaID: mangaID,
		Scale:   scale,
		Start:   start,
	}, nil
}

func parsePage(raw string, parts []string, kind callbackKind) (callbackPayload, error) {
	if len(parts) != 3 {
		return callbackPayload{}, fmt.Errorf("invalid page callback: %s", raw)
	}
	mangaID, err := strconv.Atoi(parts[1])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
	}
	page, err := strconv.Atoi(parts[2])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid page: %w", err)
	}
	return callbackPayload{
		Kind:    kind,
		MangaID: mangaID,
		Page:    page,
	}, nil
}

func parseChapterPage(raw string, parts []string, kind callbackKind) (callbackPayload, error) {
	if len(parts) != 5 {
		return callbackPayload{}, fmt.Errorf("invalid chapter page callback: %s", raw)
	}
	mangaID, err := strconv.Atoi(parts[1])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
	}
	start, err := strconv.Atoi(parts[2])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid start: %w", err)
	}
	rootInt, err := strconv.Atoi(parts[3])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid root flag: %w", err)
	}
	page, err := strconv.Atoi(parts[4])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid page: %w", err)
	}
	return callbackPayload{
		Kind:    kind,
		MangaID: mangaID,
		Start:   start,
		Root:    intToBool(rootInt),
		Page:    page,
	}, nil
}

func parseSingleManga(raw string, parts []string, kind callbackKind) (callbackPayload, error) {
	if len(parts) != 2 {
		return callbackPayload{}, fmt.Errorf("invalid manga callback: %s", raw)
	}
	mangaID, err := strconv.Atoi(parts[1])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
	}
	return callbackPayload{
		Kind:    kind,
		MangaID: mangaID,
	}, nil
}

func parseStartBack(raw string, parts []string, kind callbackKind) (callbackPayload, error) {
	if len(parts) != 3 {
		return callbackPayload{}, fmt.Errorf("invalid back callback: %s", raw)
	}
	mangaID, err := strconv.Atoi(parts[1])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid manga id: %w", err)
	}
	start, err := strconv.Atoi(parts[2])
	if err != nil {
		return callbackPayload{}, fmt.Errorf("invalid start: %w", err)
	}
	return callbackPayload{
		Kind:    kind,
		MangaID: mangaID,
		Start:   start,
	}, nil
}

func cbAddConfirm(mangaDexID string, isMangaPlus bool) string {
	if isMangaPlus {
		return fmt.Sprintf("add_confirm:%s:1", mangaDexID)
	}
	return fmt.Sprintf("add_confirm:%s:0", mangaDexID)
}

func cbAddManga() string {
	return "add_manga"
}

func cbListManga() string {
	return "list_manga"
}

func cbGenPair() string {
	return "gen_pair"
}

func cbMainMenu() string {
	return "main_menu"
}

func cbSelectManga(mangaID int, nextAction string) string {
	return fmt.Sprintf("select_manga:%d:%s", mangaID, nextAction)
}

func cbMangaAction(mangaID int, nextAction string) string {
	return fmt.Sprintf("manga_action:%d:%s", mangaID, nextAction)
}

func cbMarkChapterRead(mangaID int, chapterNumber string) string {
	return fmt.Sprintf("mark_chapter:%d:%s", mangaID, chapterNumber)
}

func cbMarkChapterUnread(mangaID int, chapterNumber string) string {
	return fmt.Sprintf("unread_chapter:%d:%s", mangaID, chapterNumber)
}

func cbMarkReadPick(mangaID, scale, start int) string {
	return fmt.Sprintf("mr_pick:%d:%d:%d", mangaID, scale, start)
}

func cbMarkReadPage(mangaID, page int) string {
	return fmt.Sprintf("mr_page:%d:%d", mangaID, page)
}

func cbMarkReadChapterPage(mangaID, start int, root bool, page int) string {
	return fmt.Sprintf("mr_chpage:%d:%d:%d:%d", mangaID, start, boolToInt(root), page)
}

func cbMarkReadBackRoot(mangaID int) string {
	return fmt.Sprintf("mr_back_root:%d", mangaID)
}

func cbMarkReadBackHundreds(mangaID, start int) string {
	return fmt.Sprintf("mr_back_hundreds:%d:%d", mangaID, start)
}

func cbMarkReadBackTens(mangaID, start int) string {
	return fmt.Sprintf("mr_back_tens:%d:%d", mangaID, start)
}

func cbMarkUnreadPick(mangaID, scale, start int) string {
	return fmt.Sprintf("mu_pick:%d:%d:%d", mangaID, scale, start)
}

func cbMarkUnreadPage(mangaID, page int) string {
	return fmt.Sprintf("mu_page:%d:%d", mangaID, page)
}

func cbMarkUnreadChapterPage(mangaID, start int, root bool, page int) string {
	return fmt.Sprintf("mu_chpage:%d:%d:%d:%d", mangaID, start, boolToInt(root), page)
}

func cbMarkUnreadBackRoot(mangaID int) string {
	return fmt.Sprintf("mu_back_root:%d", mangaID)
}

func cbMarkUnreadBackHundreds(mangaID, start int) string {
	return fmt.Sprintf("mu_back_hundreds:%d:%d", mangaID, start)
}

func cbMarkUnreadBackTens(mangaID, start int) string {
	return fmt.Sprintf("mu_back_tens:%d:%d", mangaID, start)
}
