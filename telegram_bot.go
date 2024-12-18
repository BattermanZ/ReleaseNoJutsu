package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func InitBot(db *sql.DB) (*tgbotapi.BotAPI, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, err
	}

	logger.Printf("Authorized on account %s", bot.Self.UserName)
	return bot, nil
}

func HandleUpdates(bot *tgbotapi.BotAPI, db *sql.DB) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		switch update.Message.Command() {
		case "start":
			msg.Text = "Welcome to ReleaseNoJutsu! Use /help to see available commands."
		case "help":
			msg.Text = "Available commands:\n" +
				"/add_manga <MangaDex ID> - Add a new manga\n" +
				"/list_manga - List all followed manga\n" +
				"/check_new - Check for new chapters\n" +
				"/mark_read <Manga ID> <Chapter Number> - Mark a chapter as read\n" +
				"/list_read <Manga ID> - List read chapters for a manga"
		case "add_manga":
			args := update.Message.CommandArguments()
			if args == "" {
				msg.Text = "Please provide a MangaDex ID."
			} else {
				msg.Text = addManga(db, args)
			}
		case "list_manga":
			msg.Text = listManga(db)
		case "check_new":
			msg.Text = checkNewChapters(db)
		case "mark_read":
			args := update.Message.CommandArguments()
			msg.Text = markChapterAsReadTelegram(db, args)
		case "list_read":
			args := update.Message.CommandArguments()
			msg.Text = listReadChaptersTelegram(db, args)
		default:
			msg.Text = "Unknown command. Use /help to see available commands."
		}

		if _, err := bot.Send(msg); err != nil {
			logger.Printf("Error sending message: %v", err)
		}
	}
}

func markChapterAsReadTelegram(db *sql.DB, args string) string {
	argParts := strings.Fields(args)
	if len(argParts) != 2 {
		return "Please provide both Manga ID and Chapter Number."
	}

	mangaID, err := strconv.Atoi(argParts[0])
	if err != nil {
		return "Invalid Manga ID. Please provide a valid number."
	}

	chapterNumber := argParts[1]

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
		return "Error marking chapter as read. Please try again."
	}

	logAction("Mark Chapter as Read (Telegram)", fmt.Sprintf("Manga ID: %d, Chapter: %s", mangaID, chapterNumber))
	return fmt.Sprintf("Chapter %s and all previous chapters for Manga ID %d marked as read successfully.", chapterNumber, mangaID)
}

func listReadChaptersTelegram(db *sql.DB, args string) string {
	mangaID, err := strconv.Atoi(args)
	if err != nil {
		return "Invalid Manga ID. Please provide a valid number."
	}

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

	logAction("List Read Chapters (Telegram)", fmt.Sprintf("Manga ID: %d", mangaID))
	return readChapters.String()
}

