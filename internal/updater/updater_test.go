package updater

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"releasenojutsu/internal/db"
	"releasenojutsu/internal/mangadex"
)

type fakeStore struct {
	mangaDexID string
	title      string
	lastSeenAt time.Time

	list    []db.Manga
	listErr error
	added   []mangadex.ChapterAttributes
}

func (s *fakeStore) ListManga() ([]db.Manga, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.list, nil
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
	feed  *mangadex.ChapterFeedResponse
	pages map[int]*mangadex.ChapterFeedResponse
	total int
}

func (m *fakeMangaDex) GetChapterFeedPage(ctx context.Context, mangaID string, limit, offset int) (*mangadex.ChapterFeedResponse, error) {
	if m.pages != nil {
		if page, ok := m.pages[offset]; ok {
			cp := *page
			cp.Limit = limit
			cp.Offset = offset
			if cp.Total == 0 {
				cp.Total = m.totalRows()
			}
			return &cp, nil
		}
		return &mangadex.ChapterFeedResponse{Data: nil, Limit: limit, Offset: offset, Total: m.totalRows()}, nil
	}
	if offset != 0 {
		return &mangadex.ChapterFeedResponse{Data: nil, Limit: limit, Offset: offset, Total: len(m.feed.Data)}, nil
	}
	cp := *m.feed
	cp.Limit = limit
	cp.Offset = offset
	cp.Total = len(m.feed.Data)
	return &cp, nil
}

func (m *fakeMangaDex) totalRows() int {
	if m.total > 0 {
		return m.total
	}
	if m.pages == nil {
		if m.feed == nil {
			return 0
		}
		return len(m.feed.Data)
	}
	count := 0
	for _, page := range m.pages {
		if page == nil {
			continue
		}
		count += len(page.Data)
	}
	return count
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
				{ID: "c2", Attributes: mangadex.ChapterAttributes{Chapter: "2", Title: "New", CreatedAt: newer}},
				{ID: "c1", Attributes: mangadex.ChapterAttributes{Chapter: "1", Title: "Old", CreatedAt: older}},
			},
		},
	}

	u := New(store, md, md)
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

func TestSyncAll_IncludesChaptersWithoutNumbers(t *testing.T) {
	store := &fakeStore{
		mangaDexID: "md",
		title:      "Title",
		lastSeenAt: time.Time{},
	}

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	md := &fakeMangaDex{
		feed: &mangadex.ChapterFeedResponse{
			Data: []mangadex.Chapter{
				{ID: "id-extra", Attributes: mangadex.ChapterAttributes{Chapter: "", Title: "Extra", CreatedAt: t2}},
				{ID: "id-1", Attributes: mangadex.ChapterAttributes{Chapter: "1", Title: "One", CreatedAt: t1}},
			},
		},
	}

	u := New(store, md, md)
	synced, _, err := u.SyncAll(context.Background(), 1)
	if err != nil {
		t.Fatalf("SyncAll(): %v", err)
	}
	if synced != 2 {
		t.Fatalf("synced=%d, want 2", synced)
	}
	if len(store.added) != 2 {
		t.Fatalf("added=%d, want 2", len(store.added))
	}
	if store.added[0].Chapter != "extra:id-extra" && store.added[1].Chapter != "extra:id-extra" {
		t.Fatalf("expected one chapter key to be extra:id-extra, got %q and %q", store.added[0].Chapter, store.added[1].Chapter)
	}
}

func TestSyncAll_PrefersFrenchThenEnglishAndClearsOthers(t *testing.T) {
	store := &fakeStore{
		mangaDexID: "md",
		title:      "Title",
		lastSeenAt: time.Time{},
	}

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	md := &fakeMangaDex{
		feed: &mangadex.ChapterFeedResponse{
			Data: []mangadex.Chapter{
				{ID: "en", Attributes: mangadex.ChapterAttributes{Chapter: "1", Title: "English Title", Language: "en", CreatedAt: t1}},
				{ID: "fr", Attributes: mangadex.ChapterAttributes{Chapter: "1", Title: "Titre FR", Language: "fr", CreatedAt: t1}},
				{ID: "es", Attributes: mangadex.ChapterAttributes{Chapter: "2", Title: "Titulo ES", Language: "es", CreatedAt: t1}},
				{ID: "en2", Attributes: mangadex.ChapterAttributes{Chapter: "3", Title: "English 3", Language: "en", CreatedAt: t1}},
			},
		},
	}

	u := New(store, md, md)
	_, _, err := u.SyncAll(context.Background(), 1)
	if err != nil {
		t.Fatalf("SyncAll(): %v", err)
	}

	var got1, got2 *mangadex.ChapterAttributes
	for i := range store.added {
		ch := &store.added[i]
		if ch.Chapter == "1" {
			got1 = ch
		}
		if ch.Chapter == "2" {
			got2 = ch
		}
	}
	if got1 == nil || got2 == nil {
		t.Fatalf("expected chapters 1 and 2 to be added, got %v", store.added)
	}
	if got1.Title != "Titre FR" {
		t.Fatalf("chapter 1 title=%q, want Titre FR", got1.Title)
	}
	if got2.Title != "" {
		t.Fatalf("chapter 2 title=%q, want empty (non-English)", got2.Title)
	}

	var got3 *mangadex.ChapterAttributes
	for i := range store.added {
		ch := &store.added[i]
		if ch.Chapter == "3" {
			got3 = ch
		}
	}
	if got3 == nil {
		t.Fatalf("expected chapter 3 to be added, got %v", store.added)
	}
	if got3.Title != "English 3" {
		t.Fatalf("chapter 3 title=%q, want English 3", got3.Title)
	}
}

func TestUpdateOne_DoesNotPoisonWatermarkWithFuturePublishAt(t *testing.T) {
	lastSeen := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	created := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	futurePublish := time.Date(2037, 12, 31, 15, 0, 0, 0, time.UTC)

	store := &fakeStore{
		mangaDexID: "md",
		title:      "Title",
		lastSeenAt: lastSeen,
	}
	md := &fakeMangaDex{
		feed: &mangadex.ChapterFeedResponse{
			Data: []mangadex.Chapter{
				{
					ID: "c1",
					Attributes: mangadex.ChapterAttributes{
						Chapter:     "1",
						Title:       "One",
						CreatedAt:   created,
						ReadableAt:  created,
						PublishedAt: futurePublish,
					},
				},
			},
		},
	}

	u := New(store, md, md)
	res, err := u.UpdateOne(context.Background(), 1)
	if err != nil {
		t.Fatalf("UpdateOne(): %v", err)
	}

	if len(store.added) != 1 {
		t.Fatalf("added=%d, want 1", len(store.added))
	}
	if !res.LastSeenAt.Equal(created) {
		t.Fatalf("LastSeenAt=%v, want %v", res.LastSeenAt, created)
	}
	if store.added[0].PublishedAt.After(time.Now().UTC().Add(24 * time.Hour)) {
		t.Fatalf("published_at was not normalized, got %v", store.added[0].PublishedAt)
	}
}

func TestUpdateOne_SkipsFuturePublishOnlyTimestamps(t *testing.T) {
	lastSeen := time.Time{}
	futurePublish := time.Date(2037, 12, 31, 15, 0, 0, 0, time.UTC)

	store := &fakeStore{
		mangaDexID: "md",
		title:      "Title",
		lastSeenAt: lastSeen,
	}
	md := &fakeMangaDex{
		feed: &mangadex.ChapterFeedResponse{
			Data: []mangadex.Chapter{
				{
					ID: "c1",
					Attributes: mangadex.ChapterAttributes{
						Chapter:     "1",
						Title:       "One",
						PublishedAt: futurePublish,
					},
				},
			},
		},
	}

	u := New(store, md, md)
	res, err := u.UpdateOne(context.Background(), 1)
	if err != nil {
		t.Fatalf("UpdateOne(): %v", err)
	}

	if len(store.added) != 0 {
		t.Fatalf("added=%d, want 0 for publishAt-only future sentinel", len(store.added))
	}
	if !res.LastSeenAt.Equal(lastSeen) {
		t.Fatalf("LastSeenAt=%v, want unchanged %v", res.LastSeenAt, lastSeen)
	}
}

func TestUpdateOne_ContinuesPastFuturePublishOnlyPage(t *testing.T) {
	lastSeen := time.Time{}
	futurePublish := time.Date(2037, 12, 31, 15, 0, 0, 0, time.UTC)
	created := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	page0 := make([]mangadex.Chapter, 0, 100)
	for i := 1; i <= 100; i++ {
		page0 = append(page0, mangadex.Chapter{
			ID: "f" + strconv.Itoa(i),
			Attributes: mangadex.ChapterAttributes{
				Chapter:     strconv.Itoa(i),
				Title:       "Future sentinel",
				PublishedAt: futurePublish,
			},
		})
	}

	store := &fakeStore{
		mangaDexID: "md",
		title:      "Title",
		lastSeenAt: lastSeen,
	}
	md := &fakeMangaDex{
		pages: map[int]*mangadex.ChapterFeedResponse{
			0: {Data: page0},
			100: {
				Data: []mangadex.Chapter{
					{
						ID: "c101",
						Attributes: mangadex.ChapterAttributes{
							Chapter:    "101",
							Title:      "Real chapter",
							CreatedAt:  created,
							ReadableAt: created,
						},
					},
				},
			},
		},
	}

	u := New(store, md, md)
	res, err := u.UpdateOne(context.Background(), 1)
	if err != nil {
		t.Fatalf("UpdateOne(): %v", err)
	}

	if len(store.added) != 1 {
		t.Fatalf("added=%d, want 1", len(store.added))
	}
	if store.added[0].Chapter != "101" {
		t.Fatalf("added chapter=%q, want 101", store.added[0].Chapter)
	}
	if !res.LastSeenAt.Equal(created) {
		t.Fatalf("LastSeenAt=%v, want %v", res.LastSeenAt, created)
	}
}

func TestUpdateAll_ListMangaError(t *testing.T) {
	store := &fakeStore{listErr: errors.New("boom")}
	md := &fakeMangaDex{feed: &mangadex.ChapterFeedResponse{Data: []mangadex.Chapter{}}}
	u := New(store, md, md)

	if _, err := u.UpdateAll(context.Background()); err == nil {
		t.Fatal("UpdateAll() expected error from ListManga")
	}
}

func TestUpdateAll_PropagatesUserIDsInResults(t *testing.T) {
	lastSeen := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{
		list: []db.Manga{
			{ID: 1, UserID: 42, MangaDexID: "md-1", Title: "One", LastSeenAt: lastSeen},
			{ID: 2, UserID: 84, MangaDexID: "md-2", Title: "Two", LastSeenAt: lastSeen},
		},
	}
	md := &fakeMangaDex{feed: &mangadex.ChapterFeedResponse{Data: []mangadex.Chapter{}}}
	u := New(store, md, md)

	results, err := u.UpdateAll(context.Background())
	if err != nil {
		t.Fatalf("UpdateAll(): %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len=%d, want 2", len(results))
	}
	if results[0].UserID != 42 || results[1].UserID != 84 {
		t.Fatalf("unexpected user ids in results: %+v", results)
	}
}
