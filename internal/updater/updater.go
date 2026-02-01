package updater

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"releasenojutsu/internal/appcopy"
	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
)

type Store interface {
	ListManga() ([]db.Manga, error)
	GetManga(mangaID int) (mangaDexID string, title string, lastChecked time.Time, lastSeenAt time.Time, err error)

	AddChapter(mangaID int64, chapterNumber, title string, publishedAt, readableAt, createdAt, updatedAt time.Time) error
	UpdateMangaLastChecked(mangaID int) error
	UpdateMangaLastSeenAt(mangaID int, seenAt time.Time) error
	CountUnreadChapters(mangaID int) (int, error)
	RecalculateUnreadCount(mangaID int) error
}

type MangaDex interface {
	GetChapterFeedPage(ctx context.Context, mangaID string, limit, offset int) (*mangadex.ChapterFeedResponse, error)
}

type Updater struct {
	store        Store
	mangadex     MangaDex
	syncMangaDex MangaDex
}

type Result struct {
	MangaID     int
	UserID      int64
	MangaDexID  string
	Title       string
	NewChapters []mangadex.ChapterInfo
	UnreadCount int
	LastSeenAt  time.Time
	Err         error
}

func New(store Store, md MangaDex, syncMD MangaDex) *Updater {
	return &Updater{
		store:        store,
		mangadex:     md,
		syncMangaDex: syncMD,
	}
}

func (u *Updater) SyncAll(ctx context.Context, mangaID int) (synced int, maxSeenAt time.Time, err error) {
	mangaDexID, _, _, currentLastSeenAt, err := u.store.GetManga(mangaID)
	if err != nil {
		return 0, time.Time{}, err
	}

	const pageLimit = 500
	offset := 0
	maxSeenAt = currentLastSeenAt
	// Keep only one entry per chapter key to avoid duplicates across languages/groups.
	seen := make(map[string]mangadex.Chapter, 512)

	for {
		feed, err := u.syncMangaDex.GetChapterFeedPage(ctx, mangaDexID, pageLimit, offset)
		if err != nil {
			return synced, maxSeenAt, err
		}
		if len(feed.Data) == 0 {
			break
		}

		for _, chapter := range feed.Data {
			seenAt := chapterSeenAt(chapter.Attributes).UTC()
			if maxSeenAt.Before(seenAt) {
				maxSeenAt = seenAt
			}

			key := chapterKey(chapter)
			if cur, ok := seen[key]; ok {
				// Prefer French titles, then English, then anything else.
				if chapterLanguageScore(chapter) < chapterLanguageScore(cur) {
					seen[key] = chapter
				} else if chapterLanguageScore(chapter) == chapterLanguageScore(cur) {
					// Tie-breaker: prefer non-empty titles.
					if strings.TrimSpace(chapter.Attributes.Title) != "" && strings.TrimSpace(cur.Attributes.Title) == "" {
						seen[key] = chapter
					}
				}
				continue
			}
			seen[key] = chapter
		}

		// Stop if we reached the end of the feed.
		if feed.Total > 0 && offset+len(feed.Data) >= feed.Total {
			break
		}
		if len(feed.Data) < pageLimit {
			break
		}
		offset += len(feed.Data)
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		chapter := seen[key]
		publishedAtUTC := chapter.Attributes.PublishedAt.UTC()
		readableAtUTC := chapter.Attributes.ReadableAt.UTC()
		if chapter.Attributes.ReadableAt.IsZero() {
			readableAtUTC = publishedAtUTC
		}
		createdAtUTC := chapter.Attributes.CreatedAt.UTC()
		if chapter.Attributes.CreatedAt.IsZero() {
			createdAtUTC = readableAtUTC
		}
		updatedAtUTC := chapter.Attributes.UpdatedAt.UTC()
		if chapter.Attributes.UpdatedAt.IsZero() {
			updatedAtUTC = createdAtUTC
		}

		title := ""
		if lang := strings.ToLower(strings.TrimSpace(chapter.Attributes.Language)); lang == "fr" || lang == "en" {
			title = chapter.Attributes.Title
		}
		if err := u.store.AddChapter(int64(mangaID), key, title, publishedAtUTC, readableAtUTC, createdAtUTC, updatedAtUTC); err != nil {
			return synced, maxSeenAt, err
		}
		synced++
	}

	_ = u.store.UpdateMangaLastChecked(mangaID)
	if currentLastSeenAt.IsZero() || maxSeenAt.After(currentLastSeenAt) {
		_ = u.store.UpdateMangaLastSeenAt(mangaID, maxSeenAt)
	}
	_ = u.store.RecalculateUnreadCount(mangaID)

	return synced, maxSeenAt, nil
}

func (u *Updater) UpdateAll(ctx context.Context) ([]Result, error) {
	manga, err := u.store.ListManga()
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(manga))
	for _, m := range manga {
		res, err := u.updateManga(ctx, m.ID, m.MangaDexID, m.Title, m.LastSeenAt)
		if err != nil {
			res = Result{
				MangaID:    m.ID,
				UserID:     m.UserID,
				MangaDexID: m.MangaDexID,
				Title:      m.Title,
				LastSeenAt: m.LastSeenAt,
				Err:        err,
			}
		} else {
			res.UserID = m.UserID
		}
		results = append(results, res)
	}

	return results, nil
}

func (u *Updater) UpdateOne(ctx context.Context, mangaID int) (Result, error) {
	mangaDexID, title, _, lastSeenAt, err := u.store.GetManga(mangaID)
	if err != nil {
		return Result{}, err
	}
	return u.updateManga(ctx, mangaID, mangaDexID, title, lastSeenAt)
}

func (u *Updater) updateManga(ctx context.Context, mangaID int, mangaDexID, title string, lastSeenAt time.Time) (Result, error) {
	const pageLimit = 100

	type chapterWithSeenAt struct {
		info   mangadex.ChapterInfo
		seenAt time.Time
	}

	var newChaptersWithTimes []chapterWithSeenAt
	maxSeenAt := lastSeenAt

	offset := 0
	for {
		feed, err := u.mangadex.GetChapterFeedPage(ctx, mangaDexID, pageLimit, offset)
		if err != nil {
			return Result{}, err
		}

		if len(feed.Data) == 0 {
			break
		}

		pageHasAnyNew := false
		for _, chapter := range feed.Data {
			seenAt := chapterSeenAt(chapter.Attributes).UTC()
			if maxSeenAt.Before(seenAt) {
				maxSeenAt = seenAt
			}

			if !seenAt.After(lastSeenAt.UTC()) {
				continue
			}
			pageHasAnyNew = true

			publishedAtUTC := chapter.Attributes.PublishedAt.UTC()
			readableAtUTC := chapter.Attributes.ReadableAt.UTC()
			if chapter.Attributes.ReadableAt.IsZero() {
				readableAtUTC = publishedAtUTC
			}
			createdAtUTC := chapter.Attributes.CreatedAt.UTC()
			if chapter.Attributes.CreatedAt.IsZero() {
				createdAtUTC = readableAtUTC
			}
			updatedAtUTC := chapter.Attributes.UpdatedAt.UTC()
			if chapter.Attributes.UpdatedAt.IsZero() {
				updatedAtUTC = createdAtUTC
			}

			key := chapterKey(chapter)
			if err := u.store.AddChapter(int64(mangaID), key, chapter.Attributes.Title, publishedAtUTC, readableAtUTC, createdAtUTC, updatedAtUTC); err != nil {
				return Result{}, err
			}

			newChaptersWithTimes = append(newChaptersWithTimes, chapterWithSeenAt{
				info: mangadex.ChapterInfo{
					Number: displayChapterNumber(chapter),
					Title:  chapter.Attributes.Title,
				},
				seenAt: seenAt,
			})
		}

		if !pageHasAnyNew {
			// We reached chapters at/before the watermark; later pages are older.
			break
		}

		// Stop if we reached the end of the feed.
		if feed.Total > 0 && offset+len(feed.Data) >= feed.Total {
			break
		}
		if len(feed.Data) < pageLimit {
			break
		}

		offset += len(feed.Data)
	}

	sort.Slice(newChaptersWithTimes, func(i, j int) bool {
		return newChaptersWithTimes[i].seenAt.After(newChaptersWithTimes[j].seenAt)
	})
	newChapters := make([]mangadex.ChapterInfo, 0, len(newChaptersWithTimes))
	for _, c := range newChaptersWithTimes {
		newChapters = append(newChapters, c.info)
	}

	_ = u.store.UpdateMangaLastChecked(mangaID)
	if maxSeenAt.After(lastSeenAt) {
		_ = u.store.UpdateMangaLastSeenAt(mangaID, maxSeenAt)
	}
	_ = u.store.RecalculateUnreadCount(mangaID)

	unreadCount, err := u.store.CountUnreadChapters(mangaID)
	if err != nil {
		unreadCount = len(newChapters)
	}

	return Result{
		MangaID:     mangaID,
		MangaDexID:  mangaDexID,
		Title:       title,
		NewChapters: newChapters,
		UnreadCount: unreadCount,
		LastSeenAt:  maxSeenAt,
	}, nil
}

func FormatNewChaptersMessage(mangaTitle string, newChapters []mangadex.ChapterInfo, unreadCount int, warnOnThreePlus bool) string {
	var messageBuilder strings.Builder
	messageBuilder.WriteString(appcopy.Copy.Info.NewChapterAlertTitlePlain)
	messageBuilder.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertHeaderPlain, mangaTitle))
	for _, chapter := range newChapters {
		label := fmt.Sprintf(appcopy.Copy.Labels.ChapterPrefix, chapter.Number)
		messageBuilder.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertItemPlain, label, chapter.Title))
	}
	messageBuilder.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertUnreadPlain, unreadCount))
	if warnOnThreePlus && unreadCount >= 3 {
		messageBuilder.WriteString(appcopy.Copy.Info.NewChapterAlertWarningPlain)
	}
	messageBuilder.WriteString(fmt.Sprintf(appcopy.Copy.Info.NewChapterAlertFooterPlain, appcopy.Copy.Commands.Start))
	return messageBuilder.String()
}

func chapterSeenAt(attrs mangadex.ChapterAttributes) time.Time {
	if !attrs.CreatedAt.IsZero() {
		return attrs.CreatedAt
	}
	if !attrs.ReadableAt.IsZero() {
		return attrs.ReadableAt
	}
	return attrs.PublishedAt
}

func chapterKey(ch mangadex.Chapter) string {
	num := strings.TrimSpace(ch.Attributes.Chapter)
	if num != "" {
		return num
	}
	// Some entries (extras/oneshots) may not have a chapter number. Use the MangaDex chapter ID
	// to keep the row unique in our schema.
	if strings.TrimSpace(ch.ID) != "" {
		return "extra:" + ch.ID
	}
	return "extra:unknown"
}

func displayChapterNumber(ch mangadex.Chapter) string {
	num := strings.TrimSpace(ch.Attributes.Chapter)
	if num != "" {
		return num
	}
	return appcopy.Copy.Labels.ExtraChapterNumber
}

func chapterLanguageScore(ch mangadex.Chapter) int {
	lang := strings.ToLower(strings.TrimSpace(ch.Attributes.Language))
	if lang == "fr" {
		return 0
	}
	if lang == "en" {
		return 1
	}
	if lang == "" {
		return 3
	}
	return 2
}
