package httpapi

import (
	"context"
	"log"
	"time"

	"steam-game-takeover-backend/internal/model"

	"gorm.io/gorm"
)

const dailyTakeoverExpirationInterval = time.Hour

func expiredDailyTakeoversUpdate(db *gorm.DB, now time.Time, days int) *gorm.DB {
	cutoff := now.AddDate(0, 0, -days)
	return db.Model(&model.Takeover{}).
		Where(
			"schedule_type = ? AND is_deleted = ? AND takeover_state = ? AND gmt_create <= ?",
			model.ScheduleDaily,
			false,
			model.TakeoverStateNormal,
			cutoff,
		).
		Update("takeover_state", model.TakeoverStateClosed)
}

func closeExpiredDailyTakeovers(db *gorm.DB, now time.Time, days int) (int64, error) {
	result := expiredDailyTakeoversUpdate(db, now, days)
	return result.RowsAffected, result.Error
}

func runDailyTakeoverExpirationLoop(ctx context.Context, interval time.Duration, run func()) {
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (h *Handler) StartDailyTakeoverExpirationWorker(ctx context.Context) {
	run := func() {
		days := h.dailyTakeoverExpirationDays()
		count, err := closeExpiredDailyTakeovers(h.db, time.Now(), days)
		if err != nil {
			log.Printf("close expired daily takeovers: %v", err)
			return
		}
		if count > 0 {
			log.Printf("closed %d expired daily takeovers using %d days", count, days)
		}
	}
	go runDailyTakeoverExpirationLoop(ctx, dailyTakeoverExpirationInterval, run)
}
