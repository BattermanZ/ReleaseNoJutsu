package mangadex

import "time"

// MangaResponse represents the response for a single manga from the MangaDex API.

type MangaResponse struct {
	Data struct {
		Id         string `json:"id"`
		Attributes struct {
			Title map[string]string `json:"title"`
		} `json:"attributes"`
	} `json:"data"`
}

// ChapterFeedResponse represents the response for a manga's chapter feed.

type ChapterFeedResponse struct {
	Data []struct {
		Attributes struct {
			Chapter     string    `json:"chapter"`
			Title       string    `json:"title"`
			PublishedAt time.Time `json:"publishAt"`
		} `json:"attributes"`
	} `json:"data"`
}

// ChapterInfo holds simplified chapter information.

type ChapterInfo struct {
	Number string
	Title  string
}
