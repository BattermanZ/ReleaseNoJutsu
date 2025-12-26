package updater

import (
	"context"
	"testing"
	"time"

	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
)

type fakeStore struct {
	mangaDexID string
	title      string
	lastSeenAt time.Time

	added []mangadex.ChapterAttributes
}

func (s *fakeStore) ListManga() ([]db.Manga, error) { // not used
	return nil, nil
}

func (s *fakeStore) GetManga(mangaID int) (string, string, time.Time, time.Time, error) {
	return s.mangaDexID, s.title, time.Time{}, s.lastSeenAt, nil
}

func (s *fakeStore) AddChapter(mangaID int64, chapterNumber, title string, publishedAt, readableAt, createdAt, updatedAt time.Time) error {
	s.added = append(s.added, mangadex.ChapterAttributes{
		Chapter:     chapterNumber,
		Title:       title,
		PublishedAt: publishedAt,
		ReadableAt:  readableAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	})
	return nil
}

func (s *fakeStore) UpdateMangaLastChecked(mangaID int) error { return nil }

func (s *fakeStore) UpdateMangaLastSeenAt(mangaID int, seenAt time.Time) error {
	s.lastSeenAt = seenAt
	return nil
}

func (s *fakeStore) CountUnreadChapters(mangaID int) (int, error) { return len(s.added), nil }

func (s *fakeStore) RecalculateUnreadCount(mangaID int) error { return nil }

type fakeMangaDex struct {
	feed *mangadex.ChapterFeedResponse
}

func (m *fakeMangaDex) GetChapterFeed(ctx context.Context, mangaID string) (*mangadex.ChapterFeedResponse, error) {
	return m.feed, nil
}

func TestUpdateOne_UsesCreatedAtWatermark(t *testing.T) {
	lastSeen := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC)
	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	store := &fakeStore{
		mangaDexID: "md",
		title:      "Title",
		lastSeenAt: lastSeen,
	}
	md := &fakeMangaDex{
		feed: &mangadex.ChapterFeedResponse{
			Data: []mangadex.Chapter{
				{Attributes: mangadex.ChapterAttributes{Chapter: "2", Title: "New", CreatedAt: newer}},
				{Attributes: mangadex.ChapterAttributes{Chapter: "1", Title: "Old", CreatedAt: older}},
			},
		},
	}

	u := New(store, md)
	res, err := u.UpdateOne(context.Background(), 1)
	if err != nil {
		t.Fatalf("UpdateOne(): %v", err)
	}

	if len(store.added) != 1 {
		t.Fatalf("added chapters=%d, want 1", len(store.added))
	}
	if store.added[0].Chapter != "2" {
		t.Fatalf("added chapter=%q, want %q", store.added[0].Chapter, "2")
	}
	if res.LastSeenAt != newer {
		t.Fatalf("LastSeenAt=%v, want %v", res.LastSeenAt, newer)
	}
	if store.lastSeenAt != newer {
		t.Fatalf("store.lastSeenAt=%v, want %v", store.lastSeenAt, newer)
	}
}
