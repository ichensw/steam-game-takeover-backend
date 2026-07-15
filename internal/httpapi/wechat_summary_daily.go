package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"steam-game-takeover-backend/internal/model"
)

const wechatSummaryDailyInterval = time.Minute

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
	scheduled, err := time.Parse("15:04", h.wechatSummaryDailyTime())
	if err != nil {
		return err
	}
	if now.Hour() != scheduled.Hour() || now.Minute() != scheduled.Minute() {
		return nil
	}
	date := now.Format("2006-01-02")
	if h.appConfigValue(model.AppConfigWechatSummaryLastRunDate) == date {
		return nil
	}
	if err := h.createWechatSummaryDailyJob(ctx, date); err != nil {
		return err
	}
	return h.saveAppConfig(model.AppConfigWechatSummaryLastRunDate, date)
}

func (h *Handler) createWechatSummaryDailyJob(ctx context.Context, date string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"period":      "day",
		"date":        date,
		"roomId":      h.wechatSummaryDailyRoomID(),
		"sendToGroup": h.wechatSummaryAutoSend(),
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.WechatBotAdminURL+"/messages/summary-jobs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	h.applyWechatBotHeaders(request, "0", "system")
	response, err := h.wechatBotSummaryClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("wechat summary job create failed")
	}
	return nil
}
