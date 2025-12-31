package db

import "time"

type Manga struct {
	ID             int
	MangaDexID     string
	Title          string
	IsMangaPlus    bool
	LastChecked    time.Time
	LastSeenAt     time.Time
	LastReadNumber float64
	UnreadCount    int
}

type Status struct {
	MangaCount     int
	ChapterCount   int
	UserCount      int
	UnreadTotal    int
	CronLastRun    time.Time
	HasCronLastRun bool
}

type ChapterListItem struct {
	Number string
	Title  string
	SeenAt time.Time
}

type MangaDetails struct {
	ID                   int
	MangaDexID           string
	Title                string
	IsMangaPlus          bool
	HasLastChecked       bool
	LastChecked          time.Time
	HasLastSeenAt        bool
	LastSeenAt           time.Time
	HasLastReadNumber    bool
	LastReadNumber       float64
	UnreadCount          int
	ChaptersTotal        int
	NumericChaptersTotal int
	HasMinNumber         bool
	MinNumber            float64
	HasMaxNumber         bool
	MaxNumber            float64
}
