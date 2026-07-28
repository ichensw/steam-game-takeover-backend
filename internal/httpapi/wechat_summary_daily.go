package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"
	"steam-game-takeover-backend/internal/wechatadmin"
)

const wechatSummaryDailyInterval = time.Minute

var defaultWechatSummaryDailySchedules = []wechatSummaryDailySchedule{
	{Enabled: true, Time: "12:00", DateMode: "today", Period: "morning", Name: "上午总结"},
	{Enabled: true, Time: "18:00", DateMode: "today", Period: "afternoon", Name: "下午总结"},
	{Enabled: true, Time: "23:00", DateMode: "today", Period: "evening", Name: "晚上总结"},
}

type wechatSummaryDailySchedule struct {
	Enabled  bool   `json:"enabled"`
	Time     string `json:"time"`
	DateMode string `json:"dateMode,omitempty"`
	Period   string `json:"period"`
	RoomID   string `json:"roomId,omitempty"`
	Name     string `json:"name,omitempty"`
}

func (h *Handler) StartWechatSummaryDailyWorker(ctx context.Context) {
	run := func(now time.Time) {
		if err := h.runWechatSummaryDaily(ctx, now); err != nil {
			log.Printf("wechat summary daily: %v", err)
		}
	}
	go runWechatSummaryDailyLoop(ctx, wechatSummaryDailyInterval, run)
}

func runWechatSummaryDailyLoop(ctx context.Context, interval time.Duration, run func(time.Time)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			run(now)
		}
	}
}

func (h *Handler) runWechatSummaryDaily(ctx context.Context, now time.Time) error {
	if !h.wechatSummaryAutoDaily() {
		return nil
	}
	runs := parseWechatSummaryDailyRunKeys(h.appConfigValue(model.AppConfigWechatSummaryLastRunKeys))
	var lastErr error
	changed := false
	for _, schedule := range h.wechatSummaryDailySchedules() {
		if !schedule.Enabled || !wechatSummaryDailyScheduleDue(schedule, now) {
			continue
		}
		date := wechatSummaryDailyDate(now, schedule)
		key := wechatSummaryDailyRunKey(date, schedule)
		if runs[key] {
			continue
		}
		if err := h.createWechatSummaryDailyJob(ctx, date, schedule); err != nil {
			lastErr = err
			continue
		}
		runs[key] = true
		changed = true
	}
	if changed {
		if err := h.saveAppConfig(model.AppConfigWechatSummaryLastRunKeys, marshalWechatSummaryDailyRunKeys(runs)); err != nil {
			return err
		}
	}
	return lastErr
}

func (h *Handler) createWechatSummaryDailyJob(ctx context.Context, date string, schedule wechatSummaryDailySchedule) error {
	if h.wechatBotDB == nil {
		return errors.New("wechat bot database is not configured")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"period":      schedule.Period,
		"date":        date,
		"roomId":      schedule.RoomID,
		"sendToGroup": h.wechatSummaryAutoSend(),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/messages/summary-jobs", bytes.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	h.applyWechatBotHeaders(request, "0", "system")
	response := httptest.NewRecorder()
	wechatadmin.NewServer(h.wechatBotAdminConfig(), h.wechatBotDB).ServeHTTP(response, request)
	if response.Code == http.StatusConflict {
		return nil
	}
	if response.Code < 200 || response.Code >= 300 {
		return errors.New("wechat summary job create failed")
	}
	return nil
}

func parseWechatSummaryDailySchedules(raw string) []wechatSummaryDailySchedule {
	if strings.TrimSpace(raw) == "" {
		return []wechatSummaryDailySchedule{}
	}
	var input []wechatSummaryDailySchedule
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return []wechatSummaryDailySchedule{}
	}
	result := make([]wechatSummaryDailySchedule, 0, len(input))
	for _, schedule := range input {
		if item, ok := normalizeWechatSummaryDailySchedule(schedule); ok {
			result = append(result, item)
		}
	}
	return result
}

func marshalWechatSummaryDailySchedules(schedules []wechatSummaryDailySchedule) (string, error) {
	data, err := json.Marshal(schedules)
	return string(data), err
}

func normalizeWechatSummaryDailySchedule(schedule wechatSummaryDailySchedule) (wechatSummaryDailySchedule, bool) {
	schedule.Time = strings.TrimSpace(schedule.Time)
	if _, err := time.Parse("15:04", schedule.Time); err != nil || len(schedule.Time) != 5 {
		return wechatSummaryDailySchedule{}, false
	}
	schedule.Period = strings.TrimSpace(schedule.Period)
	if schedule.Period == "" {
		schedule.Period = "day"
	}
	if schedule.Period != "day" && schedule.Period != "morning" && schedule.Period != "afternoon" && schedule.Period != "evening" {
		return wechatSummaryDailySchedule{}, false
	}
	schedule.DateMode = strings.TrimSpace(schedule.DateMode)
	if schedule.DateMode == "" {
		schedule.DateMode = "today"
	}
	if schedule.DateMode != "today" && schedule.DateMode != "yesterday" {
		return wechatSummaryDailySchedule{}, false
	}
	schedule.RoomID = strings.TrimSpace(schedule.RoomID)
	schedule.Name = strings.TrimSpace(schedule.Name)
	return schedule, true
}

func wechatSummaryDailyScheduleDue(schedule wechatSummaryDailySchedule, now time.Time) bool {
	scheduled, err := time.Parse("15:04", schedule.Time)
	if err != nil {
		return false
	}
	return now.Hour() == scheduled.Hour() && now.Minute() == scheduled.Minute()
}

func wechatSummaryDailyDate(now time.Time, schedule wechatSummaryDailySchedule) string {
	if schedule.DateMode == "yesterday" {
		return now.AddDate(0, 0, -1).Format("2006-01-02")
	}
	return now.Format("2006-01-02")
}

func wechatSummaryDailyRunKey(date string, schedule wechatSummaryDailySchedule) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", date, schedule.Time, schedule.DateMode, schedule.Period, schedule.RoomID)
}

func parseWechatSummaryDailyRunKeys(raw string) map[string]bool {
	var runs map[string]bool
	if err := json.Unmarshal([]byte(raw), &runs); err != nil || runs == nil {
		return map[string]bool{}
	}
	return runs
}

func marshalWechatSummaryDailyRunKeys(runs map[string]bool) string {
	data, _ := json.Marshal(runs)
	return string(data)
}
