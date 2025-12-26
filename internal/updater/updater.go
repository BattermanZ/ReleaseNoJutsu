package updater

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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
	GetChapterFeed(ctx context.Context, mangaID string) (*mangadex.ChapterFeedResponse, error)
}

type Updater struct {
	store    Store
	mangadex MangaDex
}

type Result struct {
	MangaID     int
	MangaDexID  string
	Title       string
	NewChapters []mangadex.ChapterInfo
	UnreadCount int
	LastSeenAt  time.Time
	Err         error
}

func New(store Store, md MangaDex) *Updater {
	return &Updater{
		store:    store,
		mangadex: md,
	}
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
				MangaDexID: m.MangaDexID,
				Title:      m.Title,
				LastSeenAt: m.LastSeenAt,
				Err:        err,
			}
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
	feed, err := u.mangadex.GetChapterFeed(ctx, mangaDexID)
	if err != nil {
		return Result{}, err
	}

	if len(feed.Data) == 0 {
		_ = u.store.UpdateMangaLastChecked(mangaID)
		unread, _ := u.store.CountUnreadChapters(mangaID)
		return Result{
			MangaID:     mangaID,
			MangaDexID:  mangaDexID,
			Title:       title,
			NewChapters: nil,
			UnreadCount: unread,
			LastSeenAt:  lastSeenAt,
		}, nil
	}

	sort.Slice(feed.Data, func(i, j int) bool {
		return chapterSeenAt(feed.Data[i].Attributes).After(chapterSeenAt(feed.Data[j].Attributes))
	})

	var newChapters []mangadex.ChapterInfo
	maxSeenAt := lastSeenAt

	for _, chapter := range feed.Data {
		seenAt := chapterSeenAt(chapter.Attributes).UTC()
		if maxSeenAt.Before(seenAt) {
			maxSeenAt = seenAt
		}

		if !seenAt.After(lastSeenAt.UTC()) {
			continue
		}

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

		if err := u.store.AddChapter(int64(mangaID), chapter.Attributes.Chapter, chapter.Attributes.Title, publishedAtUTC, readableAtUTC, createdAtUTC, updatedAtUTC); err != nil {
			return Result{}, err
		}

		newChapters = append(newChapters, mangadex.ChapterInfo{
			Number: chapter.Attributes.Chapter,
			Title:  chapter.Attributes.Title,
		})
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

func FormatNewChaptersMessage(mangaTitle string, newChapters []mangadex.ChapterInfo, unreadCount int) string {
	var messageBuilder strings.Builder
	messageBuilder.WriteString("📢 *New Chapter Alert!*\n\n")
	messageBuilder.WriteString(fmt.Sprintf("*%s* has new chapters:\n", mangaTitle))
	for _, chapter := range newChapters {
		messageBuilder.WriteString(fmt.Sprintf("• *Ch. %s*: %s\n", chapter.Number, chapter.Title))
	}
	messageBuilder.WriteString(fmt.Sprintf("\nYou now have *%d unread chapter(s)* for this series.\n", unreadCount))
	if unreadCount >= 3 {
		messageBuilder.WriteString("\n⚠️ *Warning: You have 3 or more unread chapters for this manga!*")
	}
	messageBuilder.WriteString("\nUse /start to mark chapters as read or explore other options.")
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
