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

func TestNewClientWithLanguages(t *testing.T) {
	langs := []string{"fr", "en"}
	c := NewClientWithLanguages(langs)
	if len(c.TranslatedLanguages) != 2 || c.TranslatedLanguages[0] != "fr" || c.TranslatedLanguages[1] != "en" {
		t.Fatalf("TranslatedLanguages=%v, want [fr en]", c.TranslatedLanguages)
	}
}

func TestSleepWithContext(t *testing.T) {
	if err := sleepWithContext(context.Background(), 0); err != nil {
		t.Fatalf("sleepWithContext(0): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := sleepWithContext(ctx, 5*time.Second)
	if err == nil {
		t.Fatal("sleepWithContext canceled context: expected error")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("sleepWithContext canceled took too long: %s", time.Since(start))
	}
}

func TestRetryAfterDuration(t *testing.T) {
	if d := retryAfterDuration("5"); d != 5*time.Second {
		t.Fatalf("retryAfterDuration(\"5\")=%s, want 5s", d)
	}

	date := time.Now().UTC().Add(2 * time.Second).Format(http.TimeFormat)
	if d := retryAfterDuration(date); d <= 0 {
		t.Fatalf("retryAfterDuration(http-date)=%s, want >0", d)
	}

	if d := retryAfterDuration("not-a-date"); d != 0 {
		t.Fatalf("retryAfterDuration(invalid)=%s, want 0", d)
	}
}

func TestFetchJSON_429ThenContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.FetchJSON(ctx, srv.URL)
	if err == nil {
		t.Fatal("FetchJSON expected error")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("FetchJSON cancel path took too long: %s", time.Since(start))
	}
}

func TestGetManga_ErrorPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	c := NewClient()
	c.BaseURL = srv.URL

	if _, err := c.GetManga(context.Background(), "abc"); err == nil {
		t.Fatal("GetManga expected error for non-200 response")
	}
}

func TestGetManga_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient()
	c.BaseURL = srv.URL

	if _, err := c.GetManga(context.Background(), "abc"); err == nil {
		t.Fatal("GetManga expected JSON decode error")
	}
}

func TestGetChapterFeedPage_AppliesLanguageQueryAndBounds(t *testing.T) {
	var gotPath string
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(ChapterFeedResponse{Data: []Chapter{}})
	}))
	t.Cleanup(srv.Close)

	c := NewClientWithLanguages([]string{"fr", "en", ""})
	c.BaseURL = srv.URL
	_, err := c.GetChapterFeedPage(context.Background(), "abc", -1, -5)
	if err != nil {
		t.Fatalf("GetChapterFeedPage(): %v", err)
	}

	if !strings.HasPrefix(gotPath, "/manga/abc/feed") {
		t.Fatalf("path=%q, want /manga/abc/feed prefix", gotPath)
	}
	if !strings.Contains(gotQuery, "translatedLanguage%5B%5D=fr") || !strings.Contains(gotQuery, "translatedLanguage%5B%5D=en") {
		t.Fatalf("query missing language filters: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=100") || !strings.Contains(gotQuery, "offset=0") {
		t.Fatalf("query missing default limit/offset: %q", gotQuery)
	}
}
