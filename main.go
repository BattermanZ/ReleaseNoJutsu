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
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

const (
	baseURL = "https://api.mangadex.org"
	appName = "ReleaseNoJutsu"
)

var (
	dbMutex sync.Mutex
	logger  *log.Logger
)

func initLogger() {
	logFile, err := os.OpenFile(filepath.Join("logs", appName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Println("Application started")
}

func logAction(action, details string) {
	logger.Printf("ACTION: %s - %s", action, details)
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Create logs and database folders if they don't exist
	folders := []string{"logs", "database"}
	for _, folder := range folders {
		err := os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			log.Fatalf("Failed to create %s folder: %v", folder, err)
		}
	}

	initLogger()

	// Set up database
	dbPath := filepath.Join("database", appName+".db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables if they don't exist
	err = createTables(db)
	if err != nil {
		logger.Fatalf("Failed to create tables: %v", err)
	}

	fmt.Println("ReleaseNoJutsu initialized successfully!")

	// Initialize Telegram bot
	bot, err := InitBot(db)
	if err != nil {
		logger.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	// Start handling updates from Telegram
	go HandleUpdates(bot, db)

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

		var result string
		switch choice {
		case 1:
			var mangaID string
			fmt.Print("Enter the MangaDex ID of the manga: ")
			fmt.Scanln(&mangaID)
			result = addManga(db, mangaID)
		case 2:
			result = listManga(db)
		case 3:
			result = checkNewChapters(db)
		case 4:
			result = markChapterAsRead(db)
		case 5:
			result = listReadChapters(db)
		case 6:
			fmt.Println("Exiting...")
			return
		default:
			result = "Invalid choice. Please try again."
		}
		fmt.Println(result)
	}
}

func addManga(db *sql.DB, mangaID string) string {
	mangaURL := fmt.Sprintf("%s/manga/%s", baseURL, mangaID)
	mangaResp, err := fetchJSON(mangaURL)
	if err != nil {
		logger.Printf("Error fetching manga data: %v\n", err)
		return "Error fetching manga data. Please try again."
	}

	var mangaData MangaResponse
	err = json.Unmarshal(mangaResp, &mangaData)
	if err != nil {
		logger.Printf("Error unmarshaling manga JSON: %v\n", err)
		return "Error unmarshaling manga JSON. Please try again."
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
		logger.Printf("Error inserting manga into database: %v\n", err)
		return "Error inserting manga into database. Please try again."
	}

	mangaDBID, _ := result.LastInsertId()
	logAction("Add Manga", fmt.Sprintf("Title: %s, ID: %s", title, mangaID))

	fetchLastChapters(db, mangaID, mangaDBID)

	return fmt.Sprintf("Added manga: %s (ID: %s)", title, mangaID)
}

func fetchLastChapters(db *sql.DB, mangadexID string, mangaDBID int64) {
	chapterURL := fmt.Sprintf("%s/manga/%s/feed?order[chapter]=desc&translatedLanguage[]=en&limit=100", baseURL, mangadexID)
	chapterResp, err := fetchJSON(chapterURL)
	if err != nil {
		logger.Printf("Error fetching chapter data: %v\n", err)
		return
	}

	var chapterFeedResp ChapterFeedResponse
	err = json.Unmarshal(chapterResp, &chapterFeedResp)
	if err != nil {
		logger.Printf("Error unmarshaling chapter JSON: %v\n", err)
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
			logger.Printf("Error inserting chapter into database: %v\n", err)
		}
	}

	logAction("Fetch Chapters", fmt.Sprintf("MangaDex ID: %s, Chapters fetched: %d", mangadexID, len(chaptersToStore)))
	fmt.Printf("Fetched and stored the last %d chapters.\n", len(chaptersToStore))
}

func listManga(db *sql.DB) string {
	rows, err := db.Query("SELECT id, mangadex_id, title FROM manga")
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return "Error querying manga. Please try again."
	}
	defer rows.Close()

	var mangaList strings.Builder
	mangaList.WriteString("\nFollowed Manga:\n")
	for rows.Next() {
		var id int
		var mangadexID, title string
		err := rows.Scan(&id, &mangadexID, &title)
		if err != nil {
			logger.Printf("Error scanning manga row: %v\n", err)
			continue
		}
		mangaList.WriteString(fmt.Sprintf("%d. %s (ID: %s)\n", id, title, mangadexID))
	}
	logAction("List Manga", "Listed all followed manga")
	return mangaList.String()
}

func checkNewChapters(db *sql.DB) string {
	rows, err := db.Query("SELECT id, mangadex_id, title, last_checked FROM manga")
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return "Error querying manga. Please try again."
	}
	defer rows.Close()

	var newChaptersInfo strings.Builder
	for rows.Next() {
		var id int
		var mangadexID, title string
		var lastChecked time.Time
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked)
		if err != nil {
			logger.Printf("Error scanning manga row: %v\n", err)
			continue
		}

		chapterURL := fmt.Sprintf("%s/manga/%s/feed?order[chapter]=desc&translatedLanguage[]=en&limit=100", baseURL, mangadexID)
		chapterResp, err := fetchJSON(chapterURL)
		if err != nil {
			logger.Printf("Error fetching chapter data for %s: %v\n", title, err)
			continue
		}

		var chapterFeedResp ChapterFeedResponse
		err = json.Unmarshal(chapterResp, &chapterFeedResp)
		if err != nil {
			logger.Printf("Error unmarshaling chapter JSON for %s: %v\n", title, err)
			continue
		}

		// Sort chapters by chapter number in descending order
		sort.Slice(chapterFeedResp.Data, func(i, j int) bool {
			chapterI, _ := strconv.ParseFloat(chapterFeedResp.Data[i].Attributes.Chapter, 64)
			chapterJ, _ := strconv.ParseFloat(chapterFeedResp.Data[j].Attributes.Chapter, 64)
			return chapterI > chapterJ
		})

		newChaptersInfo.WriteString(fmt.Sprintf("\nChecking new chapters for: %s\n", title))
		newChaptersCount := 0
		for _, chapter := range chapterFeedResp.Data {
			if chapter.Attributes.PublishedAt.After(lastChecked) && newChaptersCount < 3 {
				newChaptersInfo.WriteString(fmt.Sprintf("New chapter: %s - %s (Published: %s)\n",
					chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt))

				dbMutex.Lock()
				_, err = db.Exec(`
					INSERT OR REPLACE INTO chapters (manga_id, chapter_number, title, published_at) 
					VALUES (?, ?, ?, ?)
				`, id, chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt)
				dbMutex.Unlock()
				if err != nil {
					logger.Printf("Error inserting chapter into database: %v\n", err)
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
			logger.Printf("Error updating last_checked for manga %s: %v\n", title, err)
		}

		logAction("Check New Chapters", fmt.Sprintf("Manga: %s, New chapters: %d", title, newChaptersCount))
	}
	return newChaptersInfo.String()
}

func markChapterAsRead(db *sql.DB) string {
	rows, err := db.Query("SELECT id, mangadex_id, title FROM manga")
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return "Error querying manga. Please try again."
	}
	defer rows.Close()

	var mangaList strings.Builder
	mangaList.WriteString("\nFollowed Manga:\n")
	for rows.Next() {
		var id int
		var mangadexID, title string
		err := rows.Scan(&id, &mangadexID, &title)
		if err != nil {
			logger.Printf("Error scanning manga row: %v\n", err)
			continue
		}
		mangaList.WriteString(fmt.Sprintf("%d. %s (ID: %s)\n", id, title, mangadexID))
	}

	var mangaID int
	fmt.Print("Enter the ID of the manga: ")
	fmt.Scanln(&mangaID)

	rows, err = db.Query(`
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
		logger.Printf("Error querying chapters: %v\n", err)
		return "Error querying chapters. Please try again."
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
			logger.Printf("Error scanning chapter row: %v\n", err)
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
		return "Invalid chapter number"
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
		logger.Printf("Error marking chapters as read: %v\n", err)
		return "Error marking chapters as read. Please try again."
	}

	logAction("Mark Chapter as Read", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, selectedChapter.ChapterNumber))
	return "Chapter and all previous chapters marked as read successfully."
}

func listReadChapters(db *sql.DB) string {
	rows, err := db.Query("SELECT id, mangadex_id, title FROM manga")
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return "Error querying manga. Please try again."
	}
	defer rows.Close()

	var mangaList strings.Builder
	mangaList.WriteString("\nFollowed Manga:\n")
	for rows.Next() {
		var id int
		var mangadexID, title string
		err := rows.Scan(&id, &mangadexID, &title)
		if err != nil {
			logger.Printf("Error scanning manga row: %v\n", err)
			continue
		}
		mangaList.WriteString(fmt.Sprintf("%d. %s (ID: %s)\n", id, title, mangadexID))
	}

	var mangaID int
	fmt.Print("Enter the ID of the manga to list read chapters: ")
	fmt.Scanln(&mangaID)

	rows, err = db.Query(`
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
		logger.Printf("Error querying read chapters: %v\n", err)
		return "Error querying read chapters. Please try again."
	}
	defer rows.Close()

	var readChapters strings.Builder
	readChapters.WriteString("\nRead Chapters:\n")
	for rows.Next() {
		var chapterNumber, title string
		err := rows.Scan(&chapterNumber, &title)
		if err != nil {
			logger.Printf("Error scanning chapter row: %v\n", err)
			continue
		}
		readChapters.WriteString(fmt.Sprintf("Chapter %s: %s\n", chapterNumber, title))
	}
	logAction("List Read Chapters", fmt.Sprintf("Manga ID: %d", mangaID))
	return readChapters.String()
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


type MangaResponse struct {
	Data struct {
		Attributes struct {
			Title map[string]string
		}
	}
}

type ChapterFeedResponse struct {
	Data []struct {
		Attributes struct {
			Chapter     string
			Title       string
			PublishedAt time.Time
		}
	}
}

// Telegram bot functions.
func InitBot(db *sql.DB) (*tgbotapi.BotAPI, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	bot.Debug = true
	logAction("Bot Initialized", fmt.Sprintf("Bot username: %s", bot.Self.UserName))
	return bot, nil
}

func HandleUpdates(bot *tgbotapi.BotAPI, db *sql.DB) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message updates
			continue
		}

		if !update.Message.IsCommand() { // ignore any non-command Messages
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		switch update.Message.Command() {
		case "start":
			msg.Text = "Welcome to ReleaseNoJutsu! Use /help to see available commands."
		case "help":
			msg.Text = "/addmanga [MangaDex ID]: Add a manga to follow.\n/listmanga: List followed manga.\n/check: Check for new chapters.\n/read: Mark a chapter as read.\n/readlist: List read chapters."
		case "addmanga":
			if len(update.Message.CommandArguments()) > 0 {
				mangaID := update.Message.CommandArguments()
				result := addManga(db, mangaID)
				msg.Text = result
			} else {
				msg.Text = "Please provide a MangaDex ID."
			}
		case "listmanga":
			result := listManga(db)
			msg.Text = result
		case "check":
			result := checkNewChapters(db)
			msg.Text = result
		case "read":
			result := markChapterAsRead(db)
			msg.Text = result
		case "readlist":
			result := listReadChapters(db)
			msg.Text = result
		default:
			msg.Text = "Invalid command. Use /help to see available commands."
		}

		if _, err := bot.Send(msg); err != nil {
			logger.Printf("Error sending message: %v", err)
		}
	}
}

