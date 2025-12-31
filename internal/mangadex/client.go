package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"releasenojutsu/internal/logger"
)

const (
	baseURL = "https://api.mangadex.org"
	appName = "ReleaseNoJutsu"
)

// Client is a client for the MangaDex API.

type Client struct {
	BaseURL             string
	HTTPClient          *http.Client
	TranslatedLanguages []string
}

// NewClient creates a new MangaDex API client.

func NewClient() *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		TranslatedLanguages: []string{"en"},
	}
}

func NewClientWithLanguages(langs []string) *Client {
	c := NewClient()
	c.TranslatedLanguages = langs
	return c
}

// FetchJSON fetches JSON data from the given URL.

func (c *Client) FetchJSON(ctx context.Context, url string) ([]byte, error) {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			sleepDuration := time.Duration(1<<uint(i)) * time.Second
			logger.LogMsg(logger.LogInfo, "Retry %d/%d for URL: %s", i+1, maxRetries, url)
			if err := sleepWithContext(ctx, sleepDuration); err != nil {
				break
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("error creating request: %v", err)
			continue
		}

		req.Header.Set("User-Agent", fmt.Sprintf("%s/1.0", appName))
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error making request: %v", err)
			if ctx.Err() != nil {
				break
			}
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("error reading response body: %v", readErr)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned non-200 status code %d: %s", resp.StatusCode, string(body))
			if resp.StatusCode == 429 { // Too Many Requests
				retryAfter := retryAfterDuration(resp.Header.Get("Retry-After"))
				if retryAfter > 0 {
					logger.LogMsg(logger.LogWarning, "Rate limit hit, retrying after %s", retryAfter)
					_ = sleepWithContext(ctx, retryAfter)
				} else {
					logger.LogMsg(logger.LogWarning, "Rate limit hit, waiting before retry")
				}
			}
			continue
		}

		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			if rem, err := strconv.Atoi(remaining); err == nil && rem < 5 {
				logger.LogMsg(logger.LogWarning, "Rate limit remaining is low: %d", rem)
			}
		}

		var js map[string]interface{}
		if err := json.Unmarshal(body, &js); err != nil {
			lastErr = fmt.Errorf("invalid JSON response: %v", err)
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("failed after %d retries. Last error: %v", maxRetries, lastErr)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func retryAfterDuration(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	// Most commonly this is "seconds".
	if sec, err := strconv.Atoi(h); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	// It can also be an HTTP date.
	if when, err := http.ParseTime(h); err == nil {
		d := time.Until(when)
		if d > 0 {
			return d
		}
	}
	return 0
}

func (c *Client) GetManga(ctx context.Context, mangaID string) (*MangaResponse, error) {
	mangaURL := fmt.Sprintf("%s/manga/%s", c.BaseURL, mangaID)
	mangaResp, err := c.FetchJSON(ctx, mangaURL)
	if err != nil {
		return nil, err
	}

	var mangaData MangaResponse
	err = json.Unmarshal(mangaResp, &mangaData)
	if err != nil {
		return nil, err
	}
	return &mangaData, nil
}

func (c *Client) GetChapterFeed(ctx context.Context, mangaID string) (*ChapterFeedResponse, error) {
	return c.GetChapterFeedPage(ctx, mangaID, 100, 0)
}

func (c *Client) GetChapterFeedPage(ctx context.Context, mangaID string, limit, offset int) (*ChapterFeedResponse, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	u, err := url.Parse(fmt.Sprintf("%s/manga/%s/feed", c.BaseURL, mangaID))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("order[createdAt]", "desc")
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	for _, lang := range c.TranslatedLanguages {
		lang = strings.TrimSpace(lang)
		if lang == "" {
			continue
		}
		q.Add("translatedLanguage[]", lang)
	}
	u.RawQuery = q.Encode()
	chapterURL := u.String()

	chapterResp, err := c.FetchJSON(ctx, chapterURL)
	if err != nil {
		return nil, err
	}

	var chapterFeedResp ChapterFeedResponse
	err = json.Unmarshal(chapterResp, &chapterFeedResp)
	if err != nil {
		return nil, err
	}
	return &chapterFeedResp, nil
}

// ExtractMangaIDFromURL extracts the MangaDex ID from a given URL.
func (c *Client) ExtractMangaIDFromURL(url string) (string, error) {
	// Expected format: https://mangadex.org/title/{uuid}/...
	parts := strings.Split(url, "/title/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid MangaDex URL format: %s", url)
	}

	idPart := parts[1]
	// The ID might be followed by a slug or other path segments
	id := strings.Split(idPart, "/")[0]

	// Basic UUID validation (optional but good practice)
	// A UUID is 36 characters long (32 hex digits + 4 hyphens)
	if len(id) != 36 {
		return "", fmt.Errorf("extracted ID does not look like a valid UUID: %s", id)
	}

	return id, nil
}
