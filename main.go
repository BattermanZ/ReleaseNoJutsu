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

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	baseURL = "https://api.mangadex.org"
	appName = "ReleaseNoJutsu"
)

var (
	logger  *log.Logger
	dbMutex sync.Mutex
)

type MangaResponse struct {
	Data struct {
		Attributes struct {
			Title map[string]string `json:"title"`
		} `json:"attributes"`
	} `json:"data"`
}

type ChapterFeedResponse struct {
	Data []struct {
		Attributes struct {
			Chapter     string    `json:"chapter"`
			Title       string    `json:"title"`
			PublishedAt time.Time `json:"publishedAt"`
		} `json:"attributes"`
	} `json:"data"`
}

func initLogger() {
	logFile, err := os.OpenFile(filepath.Join("logs", appName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Println("Application started")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	folders := []string{"logs", "database"}
	for _, folder := range folders {
		err := os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			log.Fatalf("Failed to create %s folder: %v", folder, err)
		}
	}

	initLogger()

	dbPath := filepath.Join("database", appName+".db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	err = createTables(db)
	if err != nil {
		logger.Fatalf("Failed to create tables: %v", err)
	}

	fmt.Println("ReleaseNoJutsu initialized successfully!")

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		logger.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)
	fmt.Println("Telegram bot initialized. Starting to handle updates...")

	handleUpdates(bot, db)
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

func handleUpdates(bot *tgbotapi.BotAPI, db *sql.DB) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			handleMessage(bot, update.Message, db)
		} else if update.CallbackQuery != nil {
			handleCallbackQuery(bot, update.CallbackQuery, db)
		}
	}
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *sql.DB) {
	logAction(message.From.ID, "Received message", message.Text)

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			sendMainMenu(bot, message.Chat.ID)
		case "help":
			sendHelpMessage(bot, message.Chat.ID)
		default:
			msg := tgbotapi.NewMessage(message.Chat.ID, "Unknown command. Use /start to see the main menu.")
			_, err := bot.Send(msg)
			if err != nil {
				logger.Printf("Error sending unknown command message: %v\n", err)
			}
		}
	} else if message.ReplyToMessage != nil && message.ReplyToMessage.Text != "" {
		handleReply(bot, message, db)
	} else {
		msg := tgbotapi.NewMessage(message.Chat.ID, "I'm sorry, I didn't understand that. Please use the menu or commands.")
		_, err := bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending misunderstanding message: %v\n", err)
		}
	}
}

func handleReply(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *sql.DB) {
	logAction(message.From.ID, "Received reply", message.Text)

	replyTo := message.ReplyToMessage.Text
	switch {
	case strings.Contains(replyTo, "MangaDex ID"):
		result := addManga(db, message.Text)
		msg := tgbotapi.NewMessage(message.Chat.ID, result)
		_, err := bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending add manga result: %v\n", err)
		}
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "I'm sorry, I didn't understand that reply. Please use the menu or commands.")
		_, err := bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending default reply message: %v\n", err)
		}
	}
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, db *sql.DB) {
	logAction(query.From.ID, "Received callback query", query.Data)

	parts := strings.Split(query.Data, ":")
	action := parts[0]

	switch action {
	case "add_manga":
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, "Please enter the MangaDex ID of the manga you want to add:")
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
		_, err := bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending add manga prompt: %v\n", err)
		}
	case "list_manga":
		result := listManga(db)
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, result)
		_, err := bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending manga list: %v\n", err)
		}
	case "check_new":
		sendMangaSelectionMenu(bot, query.Message.Chat.ID, db, "check_new")
	case "mark_read":
		sendMangaSelectionMenu(bot, query.Message.Chat.ID, db, "mark_read")
	case "list_read":
		sendMangaSelectionMenu(bot, query.Message.Chat.ID, db, "list_read")
	case "select_manga":
		if len(parts) < 3 {
			logger.Printf("Invalid callback data for select_manga: %s\n", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.Printf("Error converting manga ID: %v\n", err)
			return
		}
		nextAction := parts[2]
		handleMangaSelection(bot, query.Message.Chat.ID, db, mangaID, nextAction)
	case "mark_chapter":
		if len(parts) < 3 {
			logger.Printf("Invalid callback data for mark_chapter: %s\n", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.Printf("Error converting manga ID: %v\n", err)
			return
		}
		chapterNumber := parts[2]
		result := markChapterAsRead(db, mangaID, chapterNumber)
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, result)
		_, err = bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending mark chapter result: %v\n", err)
		}
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := bot.Request(callback); err != nil {
		logger.Printf("Error answering callback query: %v\n", err)
	}
}

func sendMangaSelectionMenu(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, nextAction string) {
	rows, err := db.Query("SELECT id, title FROM manga")
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return
	}
	defer rows.Close()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for rows.Next() {
		var id int
		var title string
		err := rows.Scan(&id, &title)
		if err != nil {
			logger.Printf("Error scanning manga row: %v\n", err)
			continue
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(title, fmt.Sprintf("select_manga:%d:%s", id, nextAction)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Select a manga:")
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	_, err = bot.Send(msg)
	if err != nil {
		logger.Printf("Error sending manga selection menu: %v\n", err)
	}
}

func handleMangaSelection(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, mangaID int, nextAction string) {
	switch nextAction {
	case "check_new":
		result := checkNewChaptersForManga(db, mangaID)
		msg := tgbotapi.NewMessage(chatID, result)
		_, err := bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending new chapters result: %v\n", err)
		}
	case "mark_read":
		sendChapterSelectionMenu(bot, chatID, db, mangaID)
	case "list_read":
		result := listReadChapters(db, mangaID)
		msg := tgbotapi.NewMessage(chatID, result)
		_, err := bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending read chapters list: %v\n", err)
		}
	}
}

func sendChapterSelectionMenu(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, mangaID int) {
	rows, err := db.Query(`
		SELECT chapter_number, title 
		FROM chapters 
		WHERE manga_id = ? AND is_read = false
		ORDER BY 
			CAST(
				CASE 
					WHEN chapter_number GLOB '[0-9]*.[0-9]*' THEN chapter_number
					WHEN chapter_number GLOB '[0-9]*' THEN chapter_number || '.0'
					ELSE '999999.0'
				END 
			AS DECIMAL) DESC
		LIMIT 3
	`, mangaID)
	if err != nil {
		logger.Printf("Error querying chapters: %v\n", err)
		return
	}
	defer rows.Close()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for rows.Next() {
		var chapterNumber, title string
		err := rows.Scan(&chapterNumber, &title)
		if err != nil {
			logger.Printf("Error scanning chapter row: %v\n", err)
			continue
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Chapter %s: %s", chapterNumber, title), fmt.Sprintf("mark_chapter:%d:%s", mangaID, chapterNumber)),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Select a chapter to mark as read:")
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	_, err = bot.Send(msg)
	if err != nil {
		logger.Printf("Error sending chapter selection menu: %v\n", err)
	}
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	logAction(chatID, "Sent main menu", "")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Add manga", "add_manga"),
			tgbotapi.NewInlineKeyboardButtonData("List followed manga", "list_manga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Check for new chapters", "check_new"),
			tgbotapi.NewInlineKeyboardButtonData("Mark chapter as read", "mark_read"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("List read chapters", "list_read"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Welcome to ReleaseNoJutsu! Please choose an action:")
	msg.ReplyMarkup = keyboard
	_, err := bot.Send(msg)
	if err != nil {
		logger.Printf("Error sending main menu: %v\n", err)
	}
}

func sendHelpMessage(bot *tgbotapi.BotAPI, chatID int64) {
	logAction(chatID, "Sent help message", "")

	helpText := `Available commands:
/start - Show the main menu
/help - Show this help message

You can use the main menu to:
- Add manga
- List followed manga
- Check for new chapters
- Mark chapters as read
- List read chapters for a manga`

	msg := tgbotapi.NewMessage(chatID, helpText)
	_, err := bot.Send(msg)
	if err != nil {
		logger.Printf("Error sending help message: %v\n", err)
	}
}

func addManga(db *sql.DB, mangaID string) string {
	logAction(0, "Add manga", mangaID)

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
	logAction(0, "Add Manga", fmt.Sprintf("Title: %s, ID: %s", title, mangaID))

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

	sort.Slice(chapterFeedResp.Data, func(i, j int) bool {
		chapterI, _ := strconv.ParseFloat(chapterFeedResp.Data[i].Attributes.Chapter, 64)
		chapterJ, _ := strconv.ParseFloat(chapterFeedResp.Data[j].Attributes.Chapter, 64)
		return chapterI > chapterJ
	})

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

	logAction(0, "Fetch Chapters", fmt.Sprintf("MangaDex ID: %s, Chapters fetched: %d", mangadexID, len(chaptersToStore)))
}

func listManga(db *sql.DB) string {
	logAction(0, "List manga", "")

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
	logAction(0, "List Manga", "Listed all followed manga")
	return mangaList.String()
}

func checkNewChaptersForManga(db *sql.DB, mangaID int) string {
	logAction(0, "Check new chapters", fmt.Sprintf("Manga ID: %d", mangaID))

	var mangadexID, title string
	var lastChecked time.Time
	err := db.QueryRow("SELECT mangadex_id, title, last_checked FROM manga WHERE id = ?", mangaID).Scan(&mangadexID, &title, &lastChecked)
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return "Error querying manga. Please try again."
	}

	chapterURL := fmt.Sprintf("%s/manga/%s/feed?order[chapter]=desc&translatedLanguage[]=en&limit=100", baseURL, mangadexID)
	chapterResp, err := fetchJSON(chapterURL)
	if err != nil {
		logger.Printf("Error fetching chapter data for %s: %v\n", title, err)
		return "Error fetching chapter data. Please try again."
	}

	var chapterFeedResp ChapterFeedResponse
	err = json.Unmarshal(chapterResp, &chapterFeedResp)
	if err != nil {
		logger.Printf("Error unmarshaling chapter JSON for %s: %v\n", title, err)
		return "Error processing chapter data. Please try again."
	}

	sort.Slice(chapterFeedResp.Data, func(i, j int) bool {
		chapterI, _ := strconv.ParseFloat(chapterFeedResp.Data[i].Attributes.Chapter, 64)
		chapterJ, _ := strconv.ParseFloat(chapterFeedResp.Data[j].Attributes.Chapter, 64)
		return chapterI > chapterJ
	})

	var newChaptersInfo strings.Builder
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
			`, mangaID, chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt)
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
	_, err = db.Exec("UPDATE manga SET last_checked = ? WHERE id = ?", time.Now(), mangaID)
	dbMutex.Unlock()
	if err != nil {
		logger.Printf("Error updating last_checked for manga %s: %v\n", title, err)
	}

	logAction(0, "Check New Chapters", fmt.Sprintf("Manga: %s, New chapters: %d", title, newChaptersCount))

	if newChaptersCount == 0 {
		return fmt.Sprintf("No new chapters found for %s", title)
	}
	return newChaptersInfo.String()
}

func markChapterAsRead(db *sql.DB, mangaID int, chapterNumber string) string {
	logAction(0, "Mark chapter as read", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))

	dbMutex.Lock()
	_, err := db.Exec(`
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
	`, mangaID, chapterNumber, chapterNumber, chapterNumber, chapterNumber)
	dbMutex.Unlock()

	if err != nil {
		logger.Printf("Error marking chapters as read: %v\n", err)
		return "Error marking chapter as read. Please try again."
	}

	logAction(0, "Mark Chapter as Read", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))
	return fmt.Sprintf("Chapter %s and all previous chapters for Manga ID %d marked as read successfully.", chapterNumber, mangaID)
}

func listReadChapters(db *sql.DB, mangaID int) string {
	logAction(0, "List read chapters", fmt.Sprintf("Manga ID: %d", mangaID))

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
		logger.Printf("Error querying read chapters: %v\n", err)
		return "Error fetching read chapters. Please try again."
	}
	defer rows.Close()

	var readChapters strings.Builder
	readChapters.WriteString(fmt.Sprintf("Read Chapters for Manga ID %d:\n", mangaID))
	for rows.Next() {
		var chapterNumber, title string
		err := rows.Scan(&chapterNumber, &title)
		if err != nil {
			logger.Printf("Error scanning chapter row: %v\n", err)
			continue
		}
		readChapters.WriteString(fmt.Sprintf("Chapter %s: %s\n", chapterNumber, title))
	}

	logAction(0, "List Read Chapters", fmt.Sprintf("Manga ID: %d", mangaID))
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

func logAction(userID int64, action, details string) {
	logger.Printf("[User: %d] [%s] %s\n", userID, action, details)
}

