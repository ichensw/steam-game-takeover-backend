package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const reminderBatchSize = 20

type wxSubscribeMessageResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type takeoverReminderMessage struct {
	ToUser     string                 `json:"touser"`
	TemplateID string                 `json:"template_id"`
	Page       string                 `json:"page"`
	Data       map[string]interface{} `json:"data"`
}

func (h *Handler) SubscribeTakeoverReminder(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}

	var req struct {
		Accepted bool `json:"accepted"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	if !req.Accepted {
		ok(c, "success", gin.H{"subscribed": false, "reason": "not_accepted"})
		return
	}

	playAt, remindAt, subscribed, err := h.saveTakeoverReminderSubscription(takeoverID, user)
	if err != nil {
		switch {
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		case errors.Is(err, errNotJoined):
			fail(c, http.StatusConflict, CodeParamInvalid, "not joined")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		}
		return
	}
	if !subscribed {
		ok(c, "success", gin.H{"subscribed": false, "reason": "no_future_play_time"})
		return
	}

	ok(c, "success", gin.H{
		"subscribed": true,
		"playAt":     playAt.Format("2006-01-02 15:04:05"),
		"remindAt":   remindAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *Handler) saveTakeoverReminderSubscription(takeoverID uint64, user model.User) (time.Time, time.Time, bool, error) {
	var playAt time.Time
	var remindAt time.Time
	err := h.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := syncExpiredTakeovers(tx, now); err != nil {
			return err
		}

		var takeover model.Takeover
		if err := tx.Where("id = ? AND is_deleted = ?", takeoverID, false).First(&takeover).Error; err != nil {
			return err
		}
		if takeover.TakeoverState == model.TakeoverStateClosed {
			return nil
		}

		var count int64
		if err := tx.Model(&model.TakeoverMember{}).
			Where("takeover_id = ? AND user_id = ? AND member_state = ?", takeoverID, user.ID, model.MemberStateJoined).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errNotJoined
		}

		nextPlayAt, ok := nextTakeoverPlayAt(takeover, now)
		if !ok {
			return nil
		}
		playAt = nextPlayAt
		remindAt = playAt.Add(-time.Duration(h.cfg.ReminderMinutes) * time.Minute)
		if remindAt.Before(now) {
			remindAt = now
		}

		subscription := model.TakeoverReminderSubscription{
			TakeoverID: takeoverID,
			UserID:     user.ID,
			OpenID:     user.OpenID,
			RemindAt:   remindAt,
			PlayAt:     playAt,
			SendState:  model.ReminderSendPending,
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "takeover_id"}, {Name: "user_id"}, {Name: "play_at"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"openid":       user.OpenID,
				"remind_at":    remindAt,
				"send_state":   model.ReminderSendPending,
				"send_error":   nil,
				"sent_at":      nil,
				"gmt_modified": time.Now(),
			}),
		}).Create(&subscription).Error
	})
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	return playAt, remindAt, !playAt.IsZero(), nil
}

func nextTakeoverPlayAt(takeover model.Takeover, now time.Time) (time.Time, bool) {
	today := truncateDate(now)
	switch takeover.ScheduleType {
	case model.ScheduleSpecifiedDate:
		if takeover.StartDate == nil {
			return time.Time{}, false
		}
		playAt, err := combineDateAndPlayTime(*takeover.StartDate, takeover.PlayTime)
		return playAt, err == nil && playAt.After(now)
	case model.ScheduleDateRange:
		if takeover.StartDate == nil || takeover.EndDate == nil {
			return time.Time{}, false
		}
		start := truncateDate(*takeover.StartDate)
		end := truncateDate(*takeover.EndDate)
		day := today
		if day.Before(start) {
			day = start
		}
		if day.After(end) {
			return time.Time{}, false
		}
		playAt, err := combineDateAndPlayTime(day, takeover.PlayTime)
		if err != nil {
			return time.Time{}, false
		}
		if !playAt.After(now) {
			day = day.AddDate(0, 0, 1)
			if day.After(end) {
				return time.Time{}, false
			}
			playAt, err = combineDateAndPlayTime(day, takeover.PlayTime)
			if err != nil {
				return time.Time{}, false
			}
		}
		return playAt, true
	case model.ScheduleDaily:
		playAt, err := combineDateAndPlayTime(today, takeover.PlayTime)
		if err != nil {
			return time.Time{}, false
		}
		if !playAt.After(now) {
			playAt, err = combineDateAndPlayTime(today.AddDate(0, 0, 1), takeover.PlayTime)
			if err != nil {
				return time.Time{}, false
			}
		}
		return playAt, true
	default:
		return time.Time{}, false
	}
}

func (h *Handler) StartTakeoverReminderWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	go func() {
		defer ticker.Stop()
		h.sendDueTakeoverReminders()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.sendDueTakeoverReminders()
			}
		}
	}()
}

func (h *Handler) sendDueTakeoverReminders() {
	var subscriptions []model.TakeoverReminderSubscription
	if err := h.db.Where("send_state = ? AND remind_at <= ?", model.ReminderSendPending, time.Now()).
		Order("remind_at ASC").
		Limit(reminderBatchSize).
		Find(&subscriptions).Error; err != nil {
		log.Printf("query takeover reminders failed: %v", err)
		return
	}
	for _, subscription := range subscriptions {
		if err := h.sendTakeoverReminder(subscription.ID); err != nil {
			log.Printf("send takeover reminder %d failed: %v", subscription.ID, err)
		}
	}
}

func (h *Handler) sendTakeoverReminder(subscriptionID uint64) error {
	subscription, takeover, _, err := h.loadReminderSendContext(subscriptionID)
	if err != nil {
		_ = h.markReminderFailed(subscriptionID, err)
		return err
	}

	err = h.sendWechatSubscribeMessage(buildTakeoverReminderMessage(subscription, takeover))
	if err != nil {
		_ = h.markReminderFailed(subscriptionID, err)
		return err
	}
	if err := h.markReminderSent(subscriptionID); err != nil {
		return err
	}
	return nil
}

func (h *Handler) loadReminderSendContext(subscriptionID uint64) (model.TakeoverReminderSubscription, model.Takeover, model.User, error) {
	var subscription model.TakeoverReminderSubscription
	if err := h.db.Where("id = ? AND send_state = ?", subscriptionID, model.ReminderSendPending).First(&subscription).Error; err != nil {
		return subscription, model.Takeover{}, model.User{}, err
	}

	var takeover model.Takeover
	if err := h.db.Where("id = ? AND is_deleted = ? AND takeover_state = ?", subscription.TakeoverID, false, model.TakeoverStateNormal).First(&takeover).Error; err != nil {
		return subscription, takeover, model.User{}, err
	}

	var user model.User
	if err := h.db.Where("id = ? AND is_deleted = ? AND is_banned = ?", subscription.UserID, false, false).First(&user).Error; err != nil {
		return subscription, takeover, user, err
	}

	var count int64
	if err := h.db.Model(&model.TakeoverMember{}).
		Where("takeover_id = ? AND user_id = ? AND member_state = ?", subscription.TakeoverID, subscription.UserID, model.MemberStateJoined).
		Count(&count).Error; err != nil {
		return subscription, takeover, user, err
	}
	if count == 0 {
		return subscription, takeover, user, errNotJoined
	}
	return subscription, takeover, user, nil
}

func buildTakeoverReminderMessage(subscription model.TakeoverReminderSubscription, takeover model.Takeover) takeoverReminderMessage {
	return takeoverReminderMessage{
		ToUser:     subscription.OpenID,
		TemplateID: "",
		Page:       fmt.Sprintf("pages/detail/detail?id=%d", takeover.ID),
		Data: map[string]interface{}{
			"thing4":  map[string]string{"value": truncateRunes(takeover.Title, 20)},
			"thing2":  map[string]string{"value": "接龙即将开始，请准时集合"},
			"date3":   map[string]string{"value": subscription.PlayAt.Format("2006-01-02 15:04")},
			"thing12": map[string]string{"value": "进入 Kook 语音频道"},
		},
	}
}

func (h *Handler) sendWechatSubscribeMessage(message takeoverReminderMessage) error {
	token, err := h.wechatAccessToken()
	if err != nil {
		return err
	}
	message.TemplateID = h.cfg.ReminderTemplateID

	resp, err := resty.New().R().
		SetQueryParam("access_token", token).
		SetHeader("Content-Type", "application/json").
		SetBody(message).
		Post("https://api.weixin.qq.com/cgi-bin/message/subscribe/send")
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("wechat subscribe http status %d", resp.StatusCode())
	}

	var result wxSubscribeMessageResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return fmt.Errorf("decode wechat subscribe response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wechat subscribe error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

func (h *Handler) markReminderSent(subscriptionID uint64) error {
	now := time.Now()
	return h.db.Model(&model.TakeoverReminderSubscription{}).
		Where("id = ? AND send_state = ?", subscriptionID, model.ReminderSendPending).
		Updates(map[string]interface{}{
			"send_state": model.ReminderSendSent,
			"send_error": nil,
			"sent_at":    now,
		}).Error
}

func (h *Handler) markReminderFailed(subscriptionID uint64, sendErr error) error {
	message := truncateRunes(strings.TrimSpace(sendErr.Error()), 255)
	return h.db.Model(&model.TakeoverReminderSubscription{}).
		Where("id = ? AND send_state = ?", subscriptionID, model.ReminderSendPending).
		Updates(map[string]interface{}{
			"send_state": model.ReminderSendFailed,
			"send_error": stringPtr(message),
		}).Error
}

func truncateRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
