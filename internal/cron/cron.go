package cron

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"

	"releasenojutsu/internal/db"
	"releasenojutsu/internal/logger"
	"releasenojutsu/internal/notify"
	"releasenojutsu/internal/updater"
)

// Scheduler manages the cron jobs.

type Scheduler struct {
	DB       *db.DB
	Notifier notify.Notifier
	Updater  *updater.Updater
	cron     *cron.Cron
}

// NewScheduler creates a new scheduler.

func NewScheduler(db *db.DB, notifier notify.Notifier, upd *updater.Updater) *Scheduler {
	return &Scheduler{
		DB:       db,
		Notifier: notifier,
		Updater:  upd,
	}
}

// Run starts the cron jobs and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.cron = cron.New()
	logger.LogMsg(logger.LogInfo, "Scheduler started (runs immediately, then every 6 hours)")

	go s.performUpdate(ctx)

	_, err := s.cron.AddFunc("@every 6h", func() {
		if ctx.Err() != nil {
			return
		}
		s.performUpdate(ctx)
	})
	if err != nil {
		logger.LogMsg(logger.LogError, "Failed to set up cron job: %v", err)
		return
	}
	s.cron.Start()

	<-ctx.Done()
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-time.After(10 * time.Second):
	}
}

func (s *Scheduler) performUpdate(ctx context.Context) {
	logger.LogMsg(logger.LogInfo, "Starting scheduled update")

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	results, err := s.Updater.UpdateAll(runCtx)
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying manga for scheduled update: %v", err)
		return
	}
	users, err := s.DB.ListUsers()
	if err != nil {
		logger.LogMsg(logger.LogError, "Error querying user chat IDs: %v", err)
		return
	}

	for _, res := range results {
		if res.Err != nil {
			logger.LogMsg(logger.LogError, "Update failed for manga %s (%s): %v", res.Title, res.MangaDexID, res.Err)
			continue
		}
		if len(res.NewChapters) == 0 {
			continue
		}

		isMangaPlus, err := s.DB.IsMangaPlus(res.MangaID)
		if err != nil {
			isMangaPlus = false
		}
		message := updater.FormatNewChaptersMessageHTML(res.Title, res.NewChapters, res.UnreadCount, isMangaPlus)
		for _, chatID := range users {
			if err := s.Notifier.SendHTML(chatID, message); err != nil {
				logger.LogMsg(logger.LogError, "Error sending new chapters notification to chat ID %d: %v", chatID, err)
			}
		}
	}

	s.DB.UpdateCronLastRun()
	logger.LogMsg(logger.LogInfo, "Scheduled update completed")
}
