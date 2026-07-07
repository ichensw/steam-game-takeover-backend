package httpapi

import (
	"testing"
	"time"

	"steam-game-takeover-backend/internal/model"
)

func TestNextTakeoverPlayAt(t *testing.T) {
	now := time.Date(2026, 7, 7, 19, 40, 0, 0, time.Local)

	t.Run("specified date future", func(t *testing.T) {
		start := truncateDate(now)
		playAt, ok := nextTakeoverPlayAt(model.Takeover{
			ScheduleType: model.ScheduleSpecifiedDate,
			StartDate:    &start,
			PlayTime:     "20:00:00",
		}, now)
		if !ok || playAt.Format("2006-01-02 15:04") != "2026-07-07 20:00" {
			t.Fatalf("unexpected play time: %v %v", playAt, ok)
		}
	})

	t.Run("date range uses next day when today passed", func(t *testing.T) {
		start := truncateDate(now)
		end := start.AddDate(0, 0, 2)
		playAt, ok := nextTakeoverPlayAt(model.Takeover{
			ScheduleType: model.ScheduleDateRange,
			StartDate:    &start,
			EndDate:      &end,
			PlayTime:     "19:00:00",
		}, now)
		if !ok || playAt.Format("2006-01-02 15:04") != "2026-07-08 19:00" {
			t.Fatalf("unexpected play time: %v %v", playAt, ok)
		}
	})

	t.Run("daily uses tomorrow when today passed", func(t *testing.T) {
		playAt, ok := nextTakeoverPlayAt(model.Takeover{
			ScheduleType: model.ScheduleDaily,
			PlayTime:     "19:00:00",
		}, now)
		if !ok || playAt.Format("2006-01-02 15:04") != "2026-07-08 19:00" {
			t.Fatalf("unexpected play time: %v %v", playAt, ok)
		}
	})
}
