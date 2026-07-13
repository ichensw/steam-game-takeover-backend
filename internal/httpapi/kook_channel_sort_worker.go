package httpapi

import (
	"context"
	"fmt"
	"log"
	"time"

	"steam-game-takeover-backend/internal/model"
)

func (h *Handler) runDueKookChannelSort(ctx context.Context, now time.Time) {
	config, err := h.loadKookChannelSortConfig()
	if err != nil || !config.Enabled || config.NextRunAt == nil || config.NextRunAt.After(now) {
		return
	}
	location, err := loadKookChannelSortLocation()
	if err != nil {
		return
	}
	start, _, err := previousKookChannelSortRange(config.ScheduleType, now, location)
	if err != nil {
		return
	}
	key := fmt.Sprintf("scheduled:%s:%s", config.ScheduleType, start.Format("20060102150405"))
	run, runErr := h.executeKookChannelSort(ctx, "scheduled", &key)
	schedule := kookChannelSortSchedule{ScheduleType: config.ScheduleType, Weekday: intValue(config.Weekday), Monthday: intValue(config.Monthday), Hour: config.Hour}
	next, nextErr := nextKookChannelSortRun(now, schedule, location)
	completedOrDuplicate := run.ID != 0
	if !completedOrDuplicate {
		var count int64
		if err := h.db.Model(&model.KookChannelSortRun{}).Where("execution_key = ?", key).Count(&count).Error; err == nil {
			completedOrDuplicate = count > 0
		}
	}
	if completedOrDuplicate && nextErr == nil {
		h.db.Model(&config).Updates(map[string]interface{}{"next_run_at": next, "gmt_modified": time.Now()})
	}
	if runErr != nil {
		log.Printf("execute scheduled KOOK channel sort: %v", runErr)
	}
}

func (h *Handler) StartKookChannelSortWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				h.runDueKookChannelSort(ctx, now)
			}
		}
	}()
}
