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

type ChapterAttributes struct {
	Chapter     string    `json:"chapter"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishAt"`
	ReadableAt  time.Time `json:"readableAt"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Chapter struct {
	Attributes ChapterAttributes `json:"attributes"`
}

// ChapterFeedResponse represents the response for a manga's chapter feed.

type ChapterFeedResponse struct {
	Data []Chapter `json:"data"`
}

// ChapterInfo holds simplified chapter information.

type ChapterInfo struct {
	Number string
	Title  string
}
