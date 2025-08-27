package mangadex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"releasenojutsu/internal/logger"
)

const (
	baseURL = "https://api.mangadex.org"
	appName = "ReleaseNoJutsu"
)

// Client is a client for the MangaDex API.

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new MangaDex API client.

func NewClient() *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchJSON fetches JSON data from the given URL.

func (c *Client) FetchJSON(url string) ([]byte, error) {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			sleepDuration := time.Duration(1<<uint(i)) * time.Second
			time.Sleep(sleepDuration)
			logger.LogMsg(logger.LogInfo, "Retry %d/%d for URL: %s", i+1, maxRetries, url)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("error creating request: %v", err)
			continue
		}

		req.Header.Set("User-Agent", fmt.Sprintf("%s/1.0", appName))
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error making request: %v", err)
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("API returned non-200 status code %d: %s", resp.StatusCode, string(body))
			if resp.StatusCode == 429 { // Too Many Requests
				logger.LogMsg(logger.LogWarning, "Rate limit hit, waiting before retry")
			}
			continue
		}

		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			if rem, err := strconv.Atoi(remaining); err == nil && rem < 5 {
				logger.LogMsg(logger.LogWarning, "Rate limit remaining is low: %d", rem)
			}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("error reading response body: %v", err)
			continue
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

func (c *Client) GetManga(mangaID string) (*MangaResponse, error) {
	mangaURL := fmt.Sprintf("%s/manga/%s", c.BaseURL, mangaID)
	mangaResp, err := c.FetchJSON(mangaURL)
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

func (c *Client) GetChapterFeed(mangaID string) (*ChapterFeedResponse, error) {
	chapterURL := fmt.Sprintf("%s/manga/%s/feed?order[chapter]=desc&translatedLanguage[]=en&limit=100", c.BaseURL, mangaID)
	chapterResp, err := c.FetchJSON(chapterURL)
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
