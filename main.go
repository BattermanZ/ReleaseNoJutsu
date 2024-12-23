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
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
	_ "github.com/mattn/go-sqlite3"
)

const (
	baseURL = "https://api.mangadex.org"
	appName = "ReleaseNoJutsu"
)

var (
	logger       *log.Logger
	dbMutex      sync.Mutex
	allowedUsers []int64
)

type MangaResponse struct {
	Data struct {
		Id         string `json:"id"`
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

type ChapterInfo struct {
	Number string
	Title  string
}

func initLogger() {
	err := os.MkdirAll("logs", os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create logs folder: %v", err)
	}
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

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		logger.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	// Set bot commands so the user sees them as menu options in the Telegram client
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Show the main menu"},
		{Command: "help", Description: "Show help information"},
	}
	bot.Request(tgbotapi.NewSetMyCommands(commands...))

	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)

	allowedUsersStr := os.Getenv("TELEGRAM_ALLOWED_USERS")
	allowedUserIDs := strings.Split(allowedUsersStr, ",")
	for _, userID := range allowedUserIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 64)
		if err == nil {
			allowedUsers = append(allowedUsers, id)
		}
	}

	fmt.Printf("Number of allowed users: %d\n", len(allowedUsers))
	fmt.Println("ReleaseNoJutsu initialized successfully!")

	// Set up the cron job for daily updates
	c := cron.New()
	_, err = c.AddFunc("0 7 * * *", func() { performDailyUpdate(db, bot) })
	if err != nil {
		logger.Fatalf("Failed to set up cron job: %v", err)
	}
	c.Start()

	handleUpdates(bot, db)
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mangadex_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			last_checked TIMESTAMP,
			unread_count INTEGER DEFAULT 0
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

		CREATE TABLE IF NOT EXISTS users (
			chat_id INTEGER PRIMARY KEY
		);
	`)
	return err
}

func performDailyUpdate(db *sql.DB, bot *tgbotapi.BotAPI) {
	logger.Println("Starting daily update")

	rows, err := db.Query("SELECT id, mangadex_id, title, last_checked FROM manga")
	if err != nil {
		logger.Printf("Error querying manga for daily update: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var mangadexID, title string
		var lastChecked time.Time
		err := rows.Scan(&id, &mangadexID, &title, &lastChecked)
		if err != nil {
			logger.Printf("Error scanning manga row: %v\n", err)
			continue
		}

		newChapters := checkNewChaptersForManga(db, id)
		if len(newChapters) > 0 {
			sendNewChaptersNotificationToAllUsers(bot, db, id, title, newChapters)
		}
	}

	logger.Println("Daily update completed")
}

func checkNewChaptersForManga(db *sql.DB, mangaID int) []ChapterInfo {
	var mangadexID, title string
	var lastChecked time.Time
	err := db.QueryRow("SELECT mangadex_id, title, last_checked FROM manga WHERE id = ?", mangaID).
		Scan(&mangadexID, &title, &lastChecked)
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return nil
	}

	chapterURL := fmt.Sprintf("%s/manga/%s/feed?order[chapter]=desc&translatedLanguage[]=en&limit=100", baseURL, mangadexID)
	chapterResp, err := fetchJSON(chapterURL)
	if err != nil {
		logger.Printf("Error fetching chapter data for %s: %v\n", title, err)
		return nil
	}

	var chapterFeedResp ChapterFeedResponse
	err = json.Unmarshal(chapterResp, &chapterFeedResp)
	if err != nil {
		logger.Printf("Error unmarshaling chapter JSON for %s: %v\n", title, err)
		return nil
	}

	sort.Slice(chapterFeedResp.Data, func(i, j int) bool {
		chapterI, _ := strconv.ParseFloat(chapterFeedResp.Data[i].Attributes.Chapter, 64)
		chapterJ, _ := strconv.ParseFloat(chapterFeedResp.Data[j].Attributes.Chapter, 64)
		return chapterI > chapterJ
	})

	var newChapters []ChapterInfo
	for _, chapter := range chapterFeedResp.Data {
		if chapter.Attributes.PublishedAt.After(lastChecked) {
			newChapters = append(newChapters, ChapterInfo{
				Number: chapter.Attributes.Chapter,
				Title:  chapter.Attributes.Title,
			})

			dbMutex.Lock()
			_, err = db.Exec(`
				INSERT OR REPLACE INTO chapters (manga_id, chapter_number, title, published_at, is_read) 
				VALUES (?, ?, ?, ?, false)
			`, mangaID, chapter.Attributes.Chapter, chapter.Attributes.Title, chapter.Attributes.PublishedAt)
			dbMutex.Unlock()
			if err != nil {
				logger.Printf("Error inserting chapter into database: %v\n", err)
			}
		} else {
			break
		}
	}

	if len(newChapters) > 0 {
		dbMutex.Lock()
		_, err = db.Exec("UPDATE manga SET last_checked = ?, unread_count = unread_count + ? WHERE id = ?",
			time.Now(), len(newChapters), mangaID)
		dbMutex.Unlock()
		if err != nil {
			logger.Printf("Error updating manga last_checked and unread_count: %v\n", err)
		}
	}

	return newChapters
}

func sendNewChaptersNotificationToAllUsers(bot *tgbotapi.BotAPI, db *sql.DB, mangaID int, mangaTitle string, newChapters []ChapterInfo) {
	var unreadCount int
	err := db.QueryRow("SELECT unread_count FROM manga WHERE id = ?", mangaID).Scan(&unreadCount)
	if err != nil {
		logger.Printf("Error querying unread count: %v\n", err)
		unreadCount = len(newChapters)
	}

	message := "📢 *New Chapter Alert!*\n\n"
	message += fmt.Sprintf("*%s* has new chapters:\n", mangaTitle)
	for _, chapter := range newChapters {
		message += fmt.Sprintf("• *Ch. %s*: %s\n", chapter.Number, chapter.Title)
	}
	message += fmt.Sprintf("\nYou now have *%d unread chapter(s)* for this series.\n\nUse /start to mark chapters as read or explore other options.", unreadCount)

	rows, err := db.Query("SELECT chat_id FROM users")
	if err != nil {
		logger.Printf("Error querying user chat IDs: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var chatID int64
		err := rows.Scan(&chatID)
		if err != nil {
			logger.Printf("Error scanning user chat ID: %v\n", err)
			continue
		}

		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = "Markdown"
		_, err = bot.Send(msg)
		if err != nil {
			logger.Printf("Error sending new chapters notification to chat ID %d: %v\n", chatID, err)
		}
	}
}

func handleUpdates(bot *tgbotapi.BotAPI, db *sql.DB) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			if !checkAuthorization(update.Message.From.ID) {
				sendUnauthorizedMessage(bot, update.Message.Chat.ID)
				continue
			}
			handleMessage(bot, update.Message, db)
		} else if update.CallbackQuery != nil {
			if !checkAuthorization(update.CallbackQuery.From.ID) {
				sendUnauthorizedMessage(bot, update.CallbackQuery.Message.Chat.ID)
				continue
			}
			handleCallbackQuery(bot, update.CallbackQuery, db)
		}
	}
}

func checkAuthorization(userID int64) bool {
	for _, allowedID := range allowedUsers {
		if userID == allowedID {
			return true
		}
	}
	return false
}

func sendUnauthorizedMessage(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🚫 Sorry, you are not authorised to use this bot.")
	bot.Send(msg)
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
			msg := tgbotapi.NewMessage(message.Chat.ID, "❓ Unknown command. Please use /start or /help.")
			bot.Send(msg)
		}
	} else if message.ReplyToMessage != nil && message.ReplyToMessage.Text != "" {
		handleReply(bot, message, db)
	} else {
		msg := tgbotapi.NewMessage(message.Chat.ID, "I’m not sure what you mean. Use /start to see available options.")
		bot.Send(msg)
	}
}

func handleReply(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *sql.DB) {
	logAction(message.From.ID, "Received reply", message.Text)

	replyTo := message.ReplyToMessage.Text
	switch {
	case strings.Contains(replyTo, "enter the MangaDex ID"):
		result := addManga(db, message.Text)
		msg := tgbotapi.NewMessage(message.Chat.ID, result)
		msg.ParseMode = "Markdown"
		sendMessageWithMainMenuButton(bot, msg)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "I didn’t understand that reply. Please use /start for options.")
		bot.Send(msg)
	}
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, db *sql.DB) {
	logAction(query.From.ID, "Received callback query", query.Data)

	parts := strings.Split(query.Data, ":")
	action := parts[0]

	switch action {
	case "add_manga":
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, "📚 *Add a New Manga*\n\nPlease enter the MangaDex ID (by using the reply function from Telegram) of the manga you want to track.\n\nFor example:\nIf the URL is `https://mangadex.org/title/123e4567-e89b-12d3-a456-426614174000`, then the ID is `123e4567-e89b-12d3-a456-426614174000`.\n\nReply with the ID below:")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: "MangaDex ID"}
		sendMessageWithMainMenuButton(bot, msg)
	case "list_manga":
		result := listManga(db)
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, result)
		msg.ParseMode = "Markdown"
		sendMessageWithMainMenuButton(bot, msg)
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
		msg.ParseMode = "Markdown"
		sendMessageWithMainMenuButton(bot, msg)
	case "unread_chapter":
		if len(parts) < 3 {
			logger.Printf("Invalid callback data for unread_chapter: %s\n", query.Data)
			return
		}
		mangaID, err := strconv.Atoi(parts[1])
		if err != nil {
			logger.Printf("Error converting manga ID: %v\n", err)
			return
		}
		chapterNumber := parts[2]
		result := markChapterAsUnread(db, mangaID, chapterNumber)
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, result)
		msg.ParseMode = "Markdown"
		sendMessageWithMainMenuButton(bot, msg)
	case "main_menu":
		sendMainMenu(bot, query.Message.Chat.ID)
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := bot.Request(callback); err != nil {
		logger.Printf("Error answering callback query: %v\n", err)
	}
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	logAction(chatID, "Sent main menu", "")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📚 Add manga", "add_manga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 List followed manga", "list_manga"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 Check for new chapters", "check_new"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Mark chapter as read", "mark_read"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 List read chapters", "list_read"),
		),
	)

	welcomeMessage := `👋 *Welcome to ReleaseNoJutsu!*
Your personal manga update assistant.

Use the menu below to:
• Add a new manga to track
• View followed manga
• Check for new chapters
• Mark chapters as read
• Review read chapters

Or use /help for more info.`

	msg := tgbotapi.NewMessage(chatID, welcomeMessage)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func sendMessageWithMainMenuButton(bot *tgbotapi.BotAPI, msg tgbotapi.MessageConfig) {
	mainMenuButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "main_menu"),
		),
	)

	if msg.ReplyMarkup != nil {
		if keyboard, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, mainMenuButton.InlineKeyboard...)
			msg.ReplyMarkup = keyboard
		} else {
			msg.ReplyMarkup = mainMenuButton
		}
	} else {
		msg.ReplyMarkup = mainMenuButton
	}

	bot.Send(msg)
}

func sendHelpMessage(bot *tgbotapi.BotAPI, chatID int64) {
	logAction(chatID, "Sent help message", "")

	helpText := `ℹ️ *Help Information*
Welcome to ReleaseNoJutsu!

*Commands:*
• /start - Return to the main menu
• /help - Show this help message

*Main Features:*
- *Add manga:* Start tracking a new manga by entering its MangaDex ID.
- *List followed manga:* See which series you're currently tracking.
- *Check for new chapters:* Quickly see if any of your followed manga have fresh releases.
- *Mark chapter as read:* Update your progress so you know which chapters you've finished.
- *List read chapters:* Review what you've read recently.

If you need further assistance, feel free to /start and explore the menu options!`

	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "Markdown"
	sendMessageWithMainMenuButton(bot, msg)
}

func addManga(db *sql.DB, mangaID string) string {
	logAction(0, "Add manga", mangaID)

	mangaURL := fmt.Sprintf("%s/manga/%s", baseURL, mangaID)
	mangaResp, err := fetchJSON(mangaURL)
	if err != nil {
		logger.Printf("Error fetching manga data: %v\n", err)
		return "❌ Could not retrieve manga data. Please check the ID and try again."
	}

	var mangaData MangaResponse
	err = json.Unmarshal(mangaResp, &mangaData)
	if err != nil {
		logger.Printf("Error unmarshaling manga JSON: %v\n", err)
		return "❌ Could not process the manga data. Please try again."
	}

	var title string
	if enTitle, ok := mangaData.Data.Attributes.Title["en"]; ok && enTitle != "" {
		title = enTitle
	} else {
		for _, val := range mangaData.Data.Attributes.Title {
			if val != "" {
				title = val
				break
			}
		}
		if title == "" {
			title = "Title not available"
		}
	}

	dbMutex.Lock()
	result, err := db.Exec("INSERT INTO manga (mangadex_id, title, last_checked) VALUES (?, ?, ?)",
		mangaID, title, time.Now())
	dbMutex.Unlock()
	if err != nil {
		logger.Printf("Error inserting manga into database: %v\n", err)
		return "❌ Error adding the manga to the database. It may already exist or the ID is invalid."
	}

	mangaDBID, _ := result.LastInsertId()
	logAction(0, "Add Manga", fmt.Sprintf("Title: %s, ID: %s", title, mangaID))

	fetchLastChapters(db, mangaID, mangaDBID)

	return fmt.Sprintf("✅ *%s* has been added successfully!\nThe last few chapters have also been fetched.\n\nUse /start to see what you can do next.", title)
}

func listManga(db *sql.DB) string {
	logAction(0, "List manga", "")

	rows, err := db.Query("SELECT id, mangadex_id, title FROM manga")
	if err != nil {
		logger.Printf("Error querying manga: %v\n", err)
		return "❌ Unable to retrieve your manga list. Please try again."
	}
	defer rows.Close()

	var mangaList strings.Builder
	mangaList.WriteString("📚 *Your Followed Manga:*\n\n")
	count := 0
	for rows.Next() {
		var id int
		var mangadexID, title string
		err := rows.Scan(&id, &mangadexID, &title)
		if err != nil {
			logger.Printf("Error scanning manga row: %v\n", err)
			continue
		}
		count++
		mangaList.WriteString(fmt.Sprintf("%d. *%s*\n   ID: `%s`\n\n", count, title, mangadexID))
	}

	if count == 0 {
		mangaList.WriteString("You’re not following any manga yet. Choose 'Add manga' to start tracking a series!")
	} else {
		mangaList.WriteString(fmt.Sprintf("Total: %d manga\nUse /start to perform other actions.", count))
	}

	logAction(0, "List Manga", "Listed all followed manga")
	return mangaList.String()
}

func markChapterAsRead(db *sql.DB, mangaID int, chapterNumber string) string {
	logAction(0, "Mark chapter as read", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))

	var title string
	err := db.QueryRow("SELECT title FROM manga WHERE id = ?", mangaID).Scan(&title)
	if err != nil {
		logger.Printf("Error querying manga title: %v\n", err)
		return "❌ Could not find that manga. Please try again."
	}

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
	`, mangaID, chapterNumber, chapterNumber, chapterNumber, chapterNumber)
	dbMutex.Unlock()

	if err != nil {
		logger.Printf("Error marking chapters as read: %v\n", err)
		return "❌ Could not update the chapter status. Please try again."
	}

	return fmt.Sprintf("✅ Updated!\nAll chapters up to Chapter %s of *%s* have been marked as read.\nUse /start to continue.", chapterNumber, title)
}

func markChapterAsUnread(db *sql.DB, mangaID int, chapterNumber string) string {
	logAction(0, "Mark chapter as unread", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))

	var title string
	err := db.QueryRow("SELECT title FROM manga WHERE id = ?", mangaID).Scan(&title)
	if err != nil {
		logger.Printf("Error querying manga title: %v\n", err)
		return "❌ Could not find that manga. Please try again."
	}

	dbMutex.Lock()
	_, err = db.Exec(`
		UPDATE chapters 
		SET is_read = false 
		WHERE manga_id = ? AND chapter_number = ?
	`, mangaID, chapterNumber)
	dbMutex.Unlock()

	if err != nil {
		logger.Printf("Error marking chapter as unread: %v\n", err)
		return "❌ Could not update the chapter status. Please try again."
	}

	return fmt.Sprintf("✅ Chapter %s of *%s* is now marked as unread.\nUse /start to continue.", chapterNumber, title)
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
		callbackData := fmt.Sprintf("select_manga:%d:%s", id, nextAction)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(title, callbackData),
		})
	}

	var messageText string
	switch nextAction {
	case "check_new":
		messageText = "🔍 *Check for New Chapters*\n\nSelect a manga to see if new chapters are available:"
	case "mark_read":
		messageText = "✅ *Mark Chapters as Read*\n\nSelect a manga to update your reading progress:"
	case "list_read":
		messageText = "📖 *List Read Chapters*\n\nSelect a manga to see the chapters you’ve marked as read:"
	default:
		messageText = "📚 *Select a Manga*\n\nChoose a manga to proceed."
	}

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	sendMessageWithMainMenuButton(bot, msg)
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
		var chapterNumber, ctitle string
		err := rows.Scan(&chapterNumber, &ctitle)
		if err != nil {
			logger.Printf("Error scanning chapter row: %v\n", err)
			continue
		}
		callbackData := fmt.Sprintf("mark_chapter:%d:%s", mangaID, chapterNumber)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Ch. %s: %s", chapterNumber, ctitle), callbackData),
		})
	}

	var mangaTitle string
	err = db.QueryRow("SELECT title FROM manga WHERE id = ?", mangaID).Scan(&mangaTitle)
	if err != nil {
		logger.Printf("Error querying manga title: %v\n", err)
		mangaTitle = "Unknown Manga"
	}

	messageText := fmt.Sprintf("📖 *%s*\n\nSelect a chapter below to mark it (and all previous ones) as read:", mangaTitle)
	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	sendMessageWithMainMenuButton(bot, msg)
}

func sendReadChaptersMenu(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, mangaID int) {
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
		LIMIT 3
	`, mangaID)
	if err != nil {
		logger.Printf("Error querying chapters: %v\n", err)
		return
	}
	defer rows.Close()

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for rows.Next() {
		var chapterNumber, ctitle string
		err := rows.Scan(&chapterNumber, &ctitle)
		if err != nil {
			logger.Printf("Error scanning chapter row: %v\n", err)
			continue
		}
		callbackData := fmt.Sprintf("unread_chapter:%d:%s", mangaID, chapterNumber)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Ch. %s: %s", chapterNumber, ctitle), callbackData),
		})
	}

	var mangaTitle string
	err = db.QueryRow("SELECT title FROM manga WHERE id = ?", mangaID).Scan(&mangaTitle)
	if err != nil {
		logger.Printf("Error querying manga title: %v\n", err)
		mangaTitle = "Unknown Manga"
	}

	messageText := fmt.Sprintf("📚 *%s*\n\nBelow are some chapters you've read. Select one to mark as unread:", mangaTitle)
	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	sendMessageWithMainMenuButton(bot, msg)
}

func handleMangaSelection(bot *tgbotapi.BotAPI, chatID int64, db *sql.DB, mangaID int, nextAction string) {
	switch nextAction {
	case "check_new":
		newChapters := checkNewChaptersForManga(db, mangaID)
		result := formatNewChaptersMessage(db, mangaID, newChapters)
		msg := tgbotapi.NewMessage(chatID, result)
		msg.ParseMode = "Markdown"
		sendMessageWithMainMenuButton(bot, msg)
	case "mark_read":
		sendChapterSelectionMenu(bot, chatID, db, mangaID)
	case "list_read":
		sendReadChaptersMenu(bot, chatID, db, mangaID)
	default:
		logger.Printf("Unknown next action: %s\n", nextAction)
	}
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

func formatNewChaptersMessage(db *sql.DB, mangaID int, newChapters []ChapterInfo) string {
	var mangaTitle string
	err := db.QueryRow("SELECT title FROM manga WHERE id = ?", mangaID).Scan(&mangaTitle)
	if err != nil {
		logger.Printf("Error querying manga title: %v\n", err)
		mangaTitle = "Unknown Manga"
	}

	var message strings.Builder
	message.WriteString("🔍 *Check for New Chapters*\n\n")
	message.WriteString(fmt.Sprintf("Manga: *%s*\n\n", mangaTitle))

	if len(newChapters) == 0 {
		message.WriteString("No new chapters found.\nTry again later or use /start for more options.")
	} else {
		message.WriteString("Newly released chapters:\n")
		for _, chapter := range newChapters {
			message.WriteString(fmt.Sprintf("• *Ch. %s*: %s\n", chapter.Number, chapter.Title))
		}
		message.WriteString(fmt.Sprintf("\nTotal new chapters: %d\nUse /start to mark them as read or explore other options.", len(newChapters)))
	}

	return message.String()
}
