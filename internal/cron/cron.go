package cron

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"

	"releasenojutsu/internal/db"
	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/mangadex"
)

// Scheduler manages the cron jobs.

type Scheduler struct {
	DB       *db.DB
	Bot      *tgbotapi.BotAPI
	MangaDex *mangadex.Client
}

// NewScheduler creates a new scheduler.

func NewScheduler(db *db.DB, bot *tgbotapi.BotAPI, md *mangadex.Client) *Scheduler {
	return &Scheduler{
		DB:       db,
		Bot:      bot,
		MangaDex: md,
	}
}

// Start starts the cron jobs.

func (s *Scheduler) Start() {
	c := cron.New()
	logger.LogMsg(logger.LogInfo, "Scheduler started (runs immediately, then every 6 hours)")
	go func() {
		s.performUpdate()
		s.DB.UpdateCronLastRun()
	}()

	_, err := c.AddFunc("@every 6h", func() {
		s.performUpdate()
		s.DB.UpdateCronLastRun()
	})
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to set up cron job: %v", err)
		return
	}
	c.Start()
}

func (s *Scheduler) performUpdate() {
	logger.LogMsg(logger.LogInfo, "Starting scheduled update")

	manga, err := s.DB.ListManga()
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying manga for scheduled update: %v", err)
		return
	}

	for _, m := range manga {
		newChapters := s.checkNewChaptersForManga(m.ID, m.MangaDexID, m.Title, m.LastChecked)
		if len(newChapters) > 0 {
			s.sendNewChaptersNotificationToAllUsers(m.ID, m.Title, newChapters)
		}
	}

	logger.LogMsg(logger.LogInfo, "Scheduled update completed")
}

func (s *Scheduler) checkNewChaptersForManga(mangaID int, mangadexID, title string, lastChecked time.Time) []mangadex.ChapterInfo {
	chapterFeedResp, err := s.MangaDex.GetChapterFeed(mangadexID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error fetching chapter data for %s (ID: %s): %v", title, mangadexID, err)
		return nil
	}

	if len(chapterFeedResp.Data) == 0 {
		logger.LogMsg(logger.LogInfo, "No chapters found for manga %s (ID: %s)", title, mangadexID)
		_ = s.DB.UpdateMangaLastChecked(mangaID)
		return nil
	}

	sort.Slice(chapterFeedResp.Data, func(i, j int) bool {
		return chapterFeedResp.Data[i].Attributes.PublishedAt.After(chapterFeedResp.Data[j].Attributes.PublishedAt)
	})

	var newChapters []mangadex.ChapterInfo

	lastCheckedUTC := lastChecked.UTC()

	for _, chapter := range chapterFeedResp.Data {
		chapterTimeUTC := chapter.Attributes.PublishedAt.UTC()
		if chapterTimeUTC.After(lastCheckedUTC) {
			newChapters = append(newChapters, mangadex.ChapterInfo{
				Number: chapter.Attributes.Chapter,
				Title:  chapter.Attributes.Title,
			})

			err = s.DB.AddChapter(int64(mangaID), chapter.Attributes.Chapter, chapter.Attributes.Title, chapterTimeUTC)
			if err != nil {
				logger.LogMsg(logger.LogError, "Error inserting chapter into database: %v", err)
			}
		} else {
			break
		}
	}

	if len(newChapters) > 0 {
		_ = s.DB.UpdateMangaUnreadCount(mangaID, len(newChapters))
	}
	_ = s.DB.UpdateMangaLastChecked(mangaID)

	return newChapters
}

func (s *Scheduler) sendNewChaptersNotificationToAllUsers(mangaID int, mangaTitle string, newChapters []mangadex.ChapterInfo) {
	unreadCount, err := s.DB.GetUnreadCount(mangaID)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying unread count: %v", err)
		unreadCount = len(newChapters)
	}

	var messageBuilder strings.Builder
	messageBuilder.WriteString("📢 *New Chapter Alert!*\n\n")
	messageBuilder.WriteString(fmt.Sprintf("*%s* has new chapters:\n", mangaTitle))
	for _, chapter := range newChapters {
		messageBuilder.WriteString(fmt.Sprintf("• *Ch. %s*: %s\n", chapter.Number, chapter.Title))
	}
	messageBuilder.WriteString(fmt.Sprintf("\nYou now have *%d unread chapter(s)* for this series.\n", unreadCount))

	if unreadCount >= 3 {
		messageBuilder.WriteString("\n⚠️ *Warning: You have 3 or more unread chapters for this manga!*")
	}

	messageBuilder.WriteString("\nUse /start to mark chapters as read or explore other options.")
	message := messageBuilder.String()

	rows, err := s.DB.GetAllUsers()
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying user chat IDs: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var chatID int64
		err := rows.Scan(&chatID)
		if err != nil {
			logger.LogMsg(logger.LogError, "Error scanning user chat ID: %v", err)
			continue
		}

		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = "Markdown"
		if _, sendErr := s.Bot.Send(msg); sendErr != nil {
			logger.LogMsg(logger.LogError, "Error sending new chapters notification to chat ID %d: %v", chatID, sendErr)
		}
	}
}
