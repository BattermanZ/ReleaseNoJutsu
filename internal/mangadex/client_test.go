package mangadex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractMangaIDFromURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr bool
	}{
		{
			name:   "valid URL with slug",
			input:  "https://mangadex.org/title/a1c7c817-4e59-43b7-9365-09675a149a6f/one-piece",
			wantID: "a1c7c817-4e59-43b7-9365-09675a149a6f",
		},
		{
			name:   "valid URL no slug",
			input:  "https://mangadex.org/title/a1c7c817-4e59-43b7-9365-09675a149a6f",
			wantID: "a1c7c817-4e59-43b7-9365-09675a149a6f",
		},
		{
			name:    "invalid URL",
			input:   "https://mangadex.org/foo/bar",
			wantErr: true,
		},
		{
			name:    "non-uuid id",
			input:   "https://mangadex.org/title/not-a-uuid/something",
			wantErr: true,
		},
	}

	c := NewClient()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.ExtractMangaIDFromURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractMangaIDFromURL() error = %v, wantErr=%v", err, tt.wantErr)
			}
			if got != tt.wantID {
				t.Fatalf("ExtractMangaIDFromURL() = %q, want %q", got, tt.wantID)
			}
		})
	}
}

func TestGetChapterFeed_UsesClientBaseURLAndParsesPublishAt(t *testing.T) {
	t.Parallel()

	wantMangaID := "37b87be0-b1f4-4507-affa-06c99ebb27f8"
	wantPathPrefix := "/manga/" + wantMangaID + "/feed"
	wantPublishedAt := time.Date(2025, 2, 19, 15, 1, 38, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "ReleaseNoJutsu") {
			t.Fatalf("User-Agent = %q, want contains ReleaseNoJutsu", ua)
		}
		if ct := r.Header.Get("Accept"); ct != "application/json" {
			t.Fatalf("Accept = %q, want application/json", ct)
		}
		if !strings.HasPrefix(r.URL.Path, wantPathPrefix) {
			t.Fatalf("path = %q, want prefix %q", r.URL.Path, wantPathPrefix)
		}
		q := r.URL.Query()
		if q.Get("limit") != "100" {
			t.Fatalf("limit = %q, want 100", q.Get("limit"))
		}

		resp := ChapterFeedResponse{
			Data: []Chapter{
				{
					Attributes: ChapterAttributes{
						Chapter:     "104",
						Title:       "The Birth of Saiyaman X",
						PublishedAt: wantPublishedAt,
						ReadableAt:  wantPublishedAt,
						CreatedAt:   wantPublishedAt,
						UpdatedAt:   wantPublishedAt,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient()
	c.BaseURL = srv.URL

	got, err := c.GetChapterFeed(context.Background(), wantMangaID)
	if err != nil {
		t.Fatalf("GetChapterFeed() error: %v", err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("GetChapterFeed() len(Data)=%d, want 1", len(got.Data))
	}
	if got.Data[0].Attributes.PublishedAt.UTC() != wantPublishedAt {
		t.Fatalf("PublishedAt=%v, want %v", got.Data[0].Attributes.PublishedAt.UTC(), wantPublishedAt)
	}
}
