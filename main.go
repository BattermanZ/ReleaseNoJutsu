package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	baseURL = "https://api.mangadex.org"
	appName = "ReleaseNoJutsu"
)

type MangaResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Title map[string]string `json:"title"`
		} `json:"attributes"`
	} `json:"data"`
}

type ChapterFeedResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Chapter     string    `json:"chapter"`
			Title       string    `json:"title"`
			PublishedAt time.Time `json:"publishAt"`
		} `json:"attributes"`
	} `json:"data"`
}

var dbMutex sync.Mutex

func main() {
	// Create logs and database folders if they don't exist
	folders := []string{"logs", "database"}
	for _, folder := range folders {
		err := os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			log.Fatalf("Failed to create %s folder: %v", folder, err)
		}
	}

	// Set up logging
	logFile, err := os.OpenFile(filepath.Join("logs", appName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.Println("Application started")

	// Set up database
	dbPath := filepath.Join("database", appName+".db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables if they don't exist
	err = createTables(db)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	fmt.Println("ReleaseNoJutsu initialized successfully!")

	for {
		fmt.Println("\nChoose an action:")
		fmt.Println("1. Add manga")
		fmt.Println("2. List followed manga")
		fmt.Println("3. Check for new chapters")
		fmt.Println("4. Mark chapter as read")
		fmt.Println("5. List read chapters")
		fmt.Println("6. Exit")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			addManga(db)
		case 2:
			listManga(db)
		case 3:
			checkNewChapters(db)
		case 4:
			markChapterAsRead(db)
		case 5:
			listReadChapters(db)
		case 6:
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid choice. Please try again.")
		}
	}
}

func addManga(db *sql.DB) {
	var mangaID string
	fmt.Print("Enter the MangaDex ID of the manga: ")
	fmt.Scanln(&mangaID)

	mangaURL := fmt.Sprintf("%s/manga/%s", baseURL, mangaID)
	mangaResp, err := fetchJSON(mangaURL)
	if err != nil {
		log.Printf("Error fetching manga data: %v\n", err)
		return
	}

	var mangaData MangaResponse
	err = json.Unmarshal(mangaResp, &mangaData)
	if err != nil {
		log.Printf("Error unmarshaling manga JSON: %v\n", err)
		return
	}

	title := mangaData.Data.Attributes.Title["en"]
	if title == "" {
		title = mangaData.Data.Attributes.Title["ja"]
	}
	if title == "" {
		title = "Title not available"
	}

	dbMutex.Lock()
	result, err := db.Exec("INSERT INTO manga (mangadex_id, title, last_checked) VALUES (?, ?, ?)",
		mangaID, title, time.Now())
	dbMutex.Unlock()
	if err != nil {
		log.Printf("Error inserting manga into database: %v\n", err)
		return
	}

	mangaDBID, _ := result.LastInsertId()
	fmt.Printf("Added manga: %s (ID: %s)\n", title, mangaID)

	// Fetch and store the last 5 chapters
	fetchLastChapters(db, mangaID, mangaDBID)
}

func fetchLastChapters(db *sql.DB, mangadexID string, mangaDBID int64) {
	chapterURL := fmt.Sprintf("%s/manga/%s/feed?order[chapter]=desc&translatedLanguage[]=en&limit=100", baseURL, mangadexID)
	chapterResp, err := fetchJSON(chapterURL)
	if err != nil {
		log.Printf("Error fetching chapter data: %v\n", err)
		return
	}

	var chapterFeedResp ChapterFeedResponse
	err = json.Unmarshal(chapterResp, &chapterFeedResp)
	if err != nil {
		log.Printf("Error unmarshaling chapter JSON: %v\n", err)
		return
	}

	// Sort chapters by chapter number in descending order
	sort.Slice(chapterFeedResp.Data, func(i, j int) bool {
		chapterI, _ := strconv.ParseFloat(chapterFeedResp.Data[i].Attributes.Chapter, 64)
		chapterJ, _ := strconv.ParseFloat(chapterFeedResp.Data[j].Attributes.Chapter, 64)
		return chapterI > chapterJ
	})

	// Take only the first 3 chapters after sorting
	chaptersToStore := chapterFeedResp.Data
	if len(chaptersToStore) > 3 {
		chaptersToStore = chaptersToStore[:3]
	}

	for _, chapter := range chaptersToStore {
		dbMutex.Lock()
		_, err = db.Exec(`
			INSERT OR REPLACE INTO chapters (manga_id, chapter_number, title, published_at) 
			VALUES (?, ?, ?, ?)
		`, mangaDBID, chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt)
		dbMutex.Unlock()
		if err != nil {
			log.Printf("Error inserting chapter into database: %v\n", err)
		}
	}

	fmt.Printf("Fetched and stored the last %d chapters.\n", len(chaptersToStore))
}

func listManga(db *sql.DB) {
	rows, err := db.Query("SELECT id, mangadex_id, title FROM manga")
	if err != nil {
		log.Printf("Error querying manga: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\nFollowed Manga:")
	for rows.Next() {
		var id int
		var mangadexID, title string
		err := rows.Scan(&id, &mangadexID, &title)
		if err != nil {
			log.Printf("Error scanning manga row: %v\n", err)
			continue
		}
		fmt.Printf("%d. %s (ID: %s)\n", id, title, mangadexID)
	}
}

func checkNewChapters(db *sql.DB) {
	rows, err := db.Query("SELECT id, mangadex_id, title, last_checked FROM manga")
	if err != nil {
		log.Printf("Error querying manga: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var mangadexID, title string
		var lastChecked time.Time
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked)
		if err != nil {
			log.Printf("Error scanning manga row: %v\n", err)
			continue
		}

		chapterURL := fmt.Sprintf("%s/manga/%s/feed?order[chapter]=desc&translatedLanguage[]=en&limit=100", baseURL, mangadexID)
		chapterResp, err := fetchJSON(chapterURL)
		if err != nil {
			log.Printf("Error fetching chapter data for %s: %v\n", title, err)
			continue
		}

		var chapterFeedResp ChapterFeedResponse
		err = json.Unmarshal(chapterResp, &chapterFeedResp)
		if err != nil {
			log.Printf("Error unmarshaling chapter JSON for %s: %v\n", title, err)
			continue
		}

		// Sort chapters by chapter number in descending order
		sort.Slice(chapterFeedResp.Data, func(i, j int) bool {
			chapterI, _ := strconv.ParseFloat(chapterFeedResp.Data[i].Attributes.Chapter, 64)
			chapterJ, _ := strconv.ParseFloat(chapterFeedResp.Data[j].Attributes.Chapter, 64)
			return chapterI > chapterJ
		})

		fmt.Printf("\nChecking new chapters for: %s\n", title)
		newChaptersCount := 0
		for _, chapter := range chapterFeedResp.Data {
			if chapter.Attributes.PublishedAt.After(lastChecked) && newChaptersCount < 3 {
				fmt.Printf("New chapter: %s - %s (Published: %s)\n",
					chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt)

				dbMutex.Lock()
				_, err = db.Exec(`
					INSERT OR REPLACE INTO chapters (manga_id, chapter_number, title, published_at) 
					VALUES (?, ?, ?, ?)
				`, id, chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt)
				dbMutex.Unlock()
				if err != nil {
					log.Printf("Error inserting chapter into database: %v\n", err)
				}
				newChaptersCount++
			}
			if newChaptersCount >= 3 {
				break
			}
		}

		dbMutex.Lock()
		_, err = db.Exec("UPDATE manga SET last_checked = ? WHERE id = ?", time.Now(), id)
		dbMutex.Unlock()
		if err != nil {
			log.Printf("Error updating last_checked for manga %s: %v\n", title, err)
		}
	}
}

func markChapterAsRead(db *sql.DB) {
	listManga(db)

	var mangaID int
	fmt.Print("Enter the ID of the manga: ")
	fmt.Scanln(&mangaID)

	rows, err := db.Query(`
		SELECT id, chapter_number, title, is_read 
		FROM chapters 
		WHERE manga_id = ? 
		ORDER BY 
			CAST(
				CASE 
					WHEN chapter_number GLOB '[0-9]*.[0-9]*' THEN chapter_number
					WHEN chapter_number GLOB '[0-9]*' THEN chapter_number || '.0'
					ELSE '999999.0'
				END 
			AS DECIMAL) DESC
		LIMIT 10
	`, mangaID)
	if err != nil {
		log.Printf("Error querying chapters: %v\n", err)
		return
	}
	defer rows.Close()

	type ChapterInfo struct {
		ID            int
		ChapterNumber string
		Title         string
		IsRead        bool
	}
	var chapters []ChapterInfo

	fmt.Println("\nRecent Chapters:")
	for rows.Next() {
		var chapter ChapterInfo
		err := rows.Scan(&chapter.ID, &chapter.ChapterNumber, &chapter.Title, &chapter.IsRead)
		if err != nil {
			log.Printf("Error scanning chapter row: %v\n", err)
			continue
		}
		chapters = append(chapters, chapter)
	}

	for i, chapter := range chapters {
		readStatus := "Unread"
		if chapter.IsRead {
			readStatus = "Read"
		}
		fmt.Printf("%d. Chapter %s: %s (%s)\n", i+1, chapter.ChapterNumber, chapter.Title, readStatus)
	}

	var selectedNumber int
	fmt.Print("Enter the number of the chapter to mark as read: ")
	fmt.Scanln(&selectedNumber)

	if selectedNumber < 1 || selectedNumber > len(chapters) {
		fmt.Println("Invalid chapter number")
		return
	}

	selectedChapter := chapters[selectedNumber-1]

	dbMutex.Lock()
	_, err = db.Exec(`
		UPDATE chapters 
		SET is_read = true 
		WHERE manga_id = ? AND 
		CAST(
			CASE 
				WHEN chapter_number GLOB '[0-9]*.[0-9]*' THEN chapter_number
				WHEN chapter_number GLOB '[0-9]*' THEN chapter_number || '.0'
				ELSE '999999.0'
			END 
		AS DECIMAL) <= CAST(
			CASE 
				WHEN ? GLOB '[0-9]*.[0-9]*' THEN ?
				WHEN ? GLOB '[0-9]*' THEN ? || '.0'
				ELSE '999999.0'
			END 
		AS DECIMAL)
	`, mangaID, selectedChapter.ChapterNumber, selectedChapter.ChapterNumber,
		selectedChapter.ChapterNumber, selectedChapter.ChapterNumber)
	dbMutex.Unlock()

	if err != nil {
		log.Printf("Error marking chapters as read: %v\n", err)
		return
	}

	fmt.Println("Chapter and all previous chapters marked as read successfully.")
}

func listReadChapters(db *sql.DB) {
	listManga(db)

	var mangaID int
	fmt.Print("Enter the ID of the manga to list read chapters: ")
	fmt.Scanln(&mangaID)

	rows, err := db.Query(`
		SELECT chapter_number, title 
		FROM chapters 
		WHERE manga_id = ? AND is_read = true 
		ORDER BY 
			CAST(
				CASE 
					WHEN chapter_number GLOB '[0-9]*.[0-9]*' THEN chapter_number
					WHEN chapter_number GLOB '[0-9]*' THEN chapter_number || '.0'
					ELSE '999999.0'
				END 
			AS DECIMAL) DESC
	`, mangaID)
	if err != nil {
		log.Printf("Error querying read chapters: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\nRead Chapters:")
	for rows.Next() {
		var chapterNumber, title string
		err := rows.Scan(&chapterNumber, &title)
		if err != nil {
			log.Printf("Error scanning chapter row: %v\n", err)
			continue
		}
		fmt.Printf("Chapter %s: %s\n", chapterNumber, title)
	}
}

func fetchJSON(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	return body, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mangadex_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			last_checked TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP,
			is_read BOOLEAN DEFAULT FALSE,
			FOREIGN KEY (manga_id) REFERENCES manga (id)
		);
	`)
	return err
}

