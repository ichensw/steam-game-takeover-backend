package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
)

const kookDanmakuKeepaliveInterval = 15 * time.Second

type kookDanmakuMessage struct {
	MsgID       string `json:"msgId"`
	ChannelID   string `json:"channelId"`
	AuthorName  string `json:"authorName"`
	Content     string `json:"content"`
	ContentType int    `json:"contentType"`
	Timestamp   int64  `json:"timestamp"`
}

type kookDanmakuMemberDTO struct {
	KookUserID  string `json:"kookUserId"`
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatarUrl"`
}

type kookDanmakuMemberChannelDTO struct {
	KookUserID  string `json:"kookUserId"`
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	JoinedAt    string `json:"joinedAt"`
}

type kookDanmakuHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan kookDanmakuMessage]struct{}
}

func newKookDanmakuHub() *kookDanmakuHub {
	return &kookDanmakuHub{subscribers: map[string]map[chan kookDanmakuMessage]struct{}{}}
}

func (h *kookDanmakuHub) subscribe(channelID string) (<-chan kookDanmakuMessage, func()) {
	listener := make(chan kookDanmakuMessage, 32)
	h.mu.Lock()
	if h.subscribers[channelID] == nil {
		h.subscribers[channelID] = map[chan kookDanmakuMessage]struct{}{}
	}
	h.subscribers[channelID][listener] = struct{}{}
	h.mu.Unlock()

	return listener, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subscribers[channelID], listener)
		if len(h.subscribers[channelID]) == 0 {
			delete(h.subscribers, channelID)
		}
	}
}

func (h *kookDanmakuHub) publish(message kookDanmakuMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for listener := range h.subscribers[message.ChannelID] {
		select {
		case listener <- message:
		default:
		}
	}
}

func (h *Handler) ListKookDanmakuMembers(c *gin.Context) {
	var members []model.KookMember
	query := h.db.Where("member_status = ? AND is_bot = ? AND is_blacklisted = ?", model.KookMemberStatusJoined, false, false)
	if guildID := h.kookGuildID(); guildID != "" {
		query = query.Where("guild_id = ?", guildID)
	}
	if err := query.
		Order("COALESCE(NULLIF(nickname, ''), NULLIF(username, ''), kook_user_id)").
		Find(&members).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]kookDanmakuMemberDTO, 0, len(members))
	for _, member := range members {
		username := strings.TrimSpace(stringValue(member.Username))
		nickname := strings.TrimSpace(stringValue(member.Nickname))
		displayName := nickname
		if displayName == "" {
			displayName = username
		}
		if displayName == "" {
			displayName = member.KookUserID
		}
		list = append(list, kookDanmakuMemberDTO{
			KookUserID:  member.KookUserID,
			DisplayName: displayName,
			Username:    username,
			Nickname:    nickname,
			AvatarURL:   stringValue(member.AvatarURL),
		})
	}
	ok(c, "success", gin.H{"list": list})
}

func (h *Handler) GetKookDanmakuMemberChannel(c *gin.Context) {
	userID := strings.TrimSpace(c.Query("kookUserId"))
	if userID == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "kookUserId is required")
		return
	}

	query := h.db.Where("kook_user_id = ? AND exited_at IS NULL AND status = ?", userID, model.KookVoiceSessionActive)
	if guildID := h.kookGuildID(); guildID != "" {
		query = query.Where("guild_id = ?", guildID)
	}
	var session model.KookVoiceSession
	if err := query.Order("joined_at DESC").First(&session).Error; err != nil {
		if isNotFound(err) {
			ok(c, "success", nil)
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	channelName := h.kookVoiceChannelNames()[session.ChannelID]
	ok(c, "success", kookDanmakuMemberChannelDTO{
		KookUserID:  session.KookUserID,
		ChannelID:   session.ChannelID,
		ChannelName: channelName,
		JoinedAt:    session.JoinedAt.Format(time.RFC3339),
	})
}

func (h *Handler) StreamKookDanmaku(c *gin.Context) {
	channelID := strings.TrimSpace(c.Query("channelId"))
	if channelID == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "channelId is required")
		return
	}

	flusher, okFlush := c.Writer.(http.Flusher)
	if !okFlush {
		fail(c, http.StatusInternalServerError, CodeSystemError, "stream unavailable")
		return
	}
	listener, unsubscribe := h.danmaku.subscribe(channelID)
	defer unsubscribe()

	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Content-Type", "text/event-stream")
	c.Header("X-Accel-Buffering", "no")
	fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(kookDanmakuKeepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(c.Writer, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case message := <-listener:
			data, err := json.Marshal(message)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *Handler) publishKookDanmaku(payload map[string]interface{}) {
	message, okMessage := kookDanmakuMessageFromPayload(payload)
	if !okMessage {
		return
	}
	h.danmaku.publish(message)
}

func kookDanmakuMessageFromPayload(payload map[string]interface{}) (kookDanmakuMessage, bool) {
	contentType := intFromAny(kookPayloadValue(payload, "type"))
	if (contentType != 1 && contentType != 9) || kookPayloadBool(payload, "bot", "is_bot") {
		return kookDanmakuMessage{}, false
	}
	channelID := kookPayloadString(payload, "target_id", "targetId", "channel_id", "channelId")
	content := strings.TrimSpace(kookPayloadString(payload, "content"))
	if channelID == "" || content == "" {
		return kookDanmakuMessage{}, false
	}

	timestamp := time.Now().UnixMilli()
	if eventTime := kookPayloadEventTime(payload, "msg_timestamp", "msgTimestamp", "timestamp"); eventTime != nil {
		timestamp = eventTime.UnixMilli()
	}
	return kookDanmakuMessage{
		MsgID:       kookPayloadString(payload, "msg_id", "msgId"),
		ChannelID:   channelID,
		AuthorName:  kookPayloadString(payload, "nickname", "username"),
		Content:     content,
		ContentType: contentType,
		Timestamp:   timestamp,
	}, true
}

func intFromAny(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		var parsed int
		_, _ = fmt.Sscan(stringFromAny(value), &parsed)
		return parsed
	}
}
