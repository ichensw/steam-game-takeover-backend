package httpapi

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const wechatAIHistoryLearningTable = "ttw_wechat_ai_history_learning_task"

type wxbotHeartbeatInput struct {
	BotID         string                 `json:"botId"`
	Name          string                 `json:"name"`
	Wxid          string                 `json:"wxid"`
	Status        string                 `json:"status"`
	Version       string                 `json:"version"`
	Host          string                 `json:"host"`
	PID           int                    `json:"pid"`
	StartedAt     string                 `json:"startedAt"`
	CurrentConfig map[string]interface{} `json:"currentConfig"`
	AIStatus      map[string]interface{} `json:"aiStatus"`
}

type wxbotInstanceRow struct {
	BotID             string       `gorm:"column:bot_id"`
	Name              string       `gorm:"column:name"`
	Wxid              string       `gorm:"column:wxid"`
	Status            string       `gorm:"column:status"`
	Version           string       `gorm:"column:version"`
	Host              string       `gorm:"column:host"`
	PID               int          `gorm:"column:pid"`
	StartedAt         string       `gorm:"column:started_at"`
	LastSeenAt        time.Time    `gorm:"column:last_seen_at"`
	CurrentConfigJSON string       `gorm:"column:current_config_json"`
	ConfigJSON        string       `gorm:"column:config_json"`
	AIStatusJSON      string       `gorm:"column:ai_status_json"`
	ConfigUpdatedAt   sql.NullTime `gorm:"column:config_updated_at"`
	ConfigAppliedAt   sql.NullTime `gorm:"column:config_applied_at"`
	GmtModified       time.Time    `gorm:"column:gmt_modified"`
}

type aiHistoryLearningRow struct {
	ID                uint64          `gorm:"column:id"`
	BotID             string          `gorm:"column:bot_id"`
	RemoteTaskID      sql.NullInt64   `gorm:"column:remote_task_id"`
	RoomID            string          `gorm:"column:room_id"`
	Status            string          `gorm:"column:status"`
	Stage             string          `gorm:"column:stage"`
	WindowStart       float64         `gorm:"column:window_start"`
	WindowEnd         float64         `gorm:"column:window_end"`
	MaxMessages       int             `gorm:"column:max_messages"`
	TotalMsgCount     int             `gorm:"column:total_msg_count"`
	ProcessedMsgCount int             `gorm:"column:processed_msg_count"`
	SegmentJobCount   int             `gorm:"column:segment_job_count"`
	ErrorMessage      sql.NullString  `gorm:"column:error_message"`
	CreatedAt         float64         `gorm:"column:created_at"`
	UpdatedAt         float64         `gorm:"column:updated_at"`
	FinishedAt        sql.NullFloat64 `gorm:"column:finished_at"`
}

func (h *Handler) requireWxbotToken(c *gin.Context) bool {
	expected := strings.TrimSpace(h.cfg.WechatBotSharedSecret)
	if expected == "" {
		expected = strings.TrimSpace(os.Getenv("WECHAT_BOT_GATEWAY_SHARED_SECRET"))
	}
	if expected == "" {
		fail(c, http.StatusServiceUnavailable, CodeSystemError, "wxbot control token is not configured")
		return false
	}
	token := strings.TrimSpace(c.GetHeader(wxbotTokenHeader))
	if token == "" {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[7:])
		}
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		fail(c, http.StatusUnauthorized, CodeUnauthorized, "wxbot token invalid")
		return false
	}
	return true
}

func (h *Handler) WxbotHeartbeat(c *gin.Context) {
	var input wxbotHeartbeatInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.BotID) == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	now := time.Now()
	currentConfig := jsonObjectString(input.CurrentConfig)
	aiStatus := jsonObjectString(input.AIStatus)
	err := h.db.Exec(`
		INSERT INTO ttw_wechat_bot_instance
			(bot_id, name, wxid, status, version, host, pid, started_at, last_seen_at,
			 current_config_json, config_json, ai_status_json, gmt_create, gmt_modified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			wxid = VALUES(wxid),
			status = VALUES(status),
			version = VALUES(version),
			host = VALUES(host),
			pid = VALUES(pid),
			started_at = VALUES(started_at),
			last_seen_at = VALUES(last_seen_at),
			current_config_json = VALUES(current_config_json),
			config_json = IF(config_json = '', VALUES(config_json), config_json),
			ai_status_json = VALUES(ai_status_json),
			gmt_modified = VALUES(gmt_modified)
	`, strings.TrimSpace(input.BotID), input.Name, input.Wxid, input.Status, input.Version, input.Host,
		input.PID, input.StartedAt, now, currentConfig, currentConfig, aiStatus, now, now).Error
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save wxbot heartbeat failed")
		return
	}
	ok(c, "success", gin.H{"serverTime": now.Format(time.RFC3339)})
}

func (h *Handler) WxbotConfig(c *gin.Context) {
	botID := strings.TrimSpace(c.Query("botId"))
	if botID == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "botId is required")
		return
	}
	row, found := h.wxbotInstance(botID)
	if !found {
		ok(c, "success", gin.H{"config": gin.H{}})
		return
	}
	ok(c, "success", gin.H{
		"config":          jsonMap(row.ConfigJSON),
		"configUpdatedAt": wxbotNullTimeString(row.ConfigUpdatedAt),
	})
}

func (h *Handler) WxbotConfigApplied(c *gin.Context) {
	var body struct {
		BotID string `json:"botId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.BotID) == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	now := time.Now()
	if err := h.db.Exec(`
		UPDATE ttw_wechat_bot_instance
		SET config_applied_at = ?, gmt_modified = ?
		WHERE bot_id = ?
	`, now, now, strings.TrimSpace(body.BotID)).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "mark config applied failed")
		return
	}
	ok(c, "success", gin.H{"applied": true})
}

func (h *Handler) WxbotNextHistoryLearning(c *gin.Context) {
	botID := strings.TrimSpace(c.Query("botId"))
	if botID == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "botId is required")
		return
	}
	var row aiHistoryLearningRow
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`
			SELECT * FROM ttw_wechat_ai_history_learning_task
			WHERE status = 'queued'
			ORDER BY id ASC
			LIMIT 1
			FOR UPDATE
		`).Scan(&row).Error; err != nil {
			return err
		}
		if row.ID == 0 {
			return nil
		}
		now := unixNow()
		return tx.Exec(`
			UPDATE ttw_wechat_ai_history_learning_task
			SET status = 'running', bot_id = ?, updated_at = ?
			WHERE id = ? AND status = 'queued'
		`, botID, now, row.ID).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "claim history learning failed")
		return
	}
	if row.ID == 0 {
		ok(c, "success", gin.H{"task": nil})
		return
	}
	row.BotID = botID
	row.Status = "running"
	row.UpdatedAt = unixNow()
	ok(c, "success", gin.H{"task": aiHistoryLearningDTO(row)})
}

func (h *Handler) WxbotReportHistoryLearning(c *gin.Context, path string) {
	rawID := strings.TrimSuffix(strings.TrimPrefix(path, "/ai/history-learning/"), "/progress")
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid task id")
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	updates := map[string]interface{}{"updated_at": unixNow()}
	if botID := wxbotStringValue(body["botId"]); botID != "" {
		updates["bot_id"] = botID
	}
	if remoteID, ok := int64Value(body["remoteTaskId"]); ok {
		updates["remote_task_id"] = remoteID
	}
	for jsonKey, column := range map[string]string{
		"status":            "status",
		"stage":             "stage",
		"totalMsgCount":     "total_msg_count",
		"processedMsgCount": "processed_msg_count",
		"segmentJobCount":   "segment_job_count",
		"errorMessage":      "error_message",
	} {
		if value, ok := body[jsonKey]; ok {
			updates[column] = value
		}
	}
	if finishedAt, ok := floatValue(body["finishedAt"]); ok {
		updates["finished_at"] = finishedAt
	} else if status := wxbotStringValue(body["status"]); status == "succeeded" || status == "failed" {
		updates["finished_at"] = unixNow()
	}
	if err := h.db.Table(wechatAIHistoryLearningTable).Where("id = ?", id).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "update history learning failed")
		return
	}
	row, found := h.aiHistoryLearningTask(id)
	if !found {
		fail(c, http.StatusNotFound, CodeParamInvalid, "task not found")
		return
	}
	ok(c, "success", aiHistoryLearningDTO(row))
}

func (h *Handler) AdminWxbots(c *gin.Context, path string) {
	if path == "/wxbots" && c.Request.Method == http.MethodGet {
		ok(c, "success", gin.H{"list": h.listWxbotDTOs()})
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "wxbots" || parts[2] != "config" {
		fail(c, http.StatusNotFound, CodeParamInvalid, "wechat bot endpoint not found")
		return
	}
	botID, _ := url.PathUnescape(parts[1])
	if botID == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid bot id")
		return
	}
	if c.Request.Method == http.MethodGet {
		ok(c, "success", h.wxbotConfigDetail(botID))
		return
	}
	var body struct {
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Config == nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	now := time.Now()
	if err := h.db.Exec(`
		INSERT INTO ttw_wechat_bot_instance
			(bot_id, last_seen_at, current_config_json, config_json, ai_status_json, config_updated_at, gmt_create, gmt_modified)
		VALUES (?, ?, '{}', ?, '{}', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			config_json = VALUES(config_json),
			config_updated_at = VALUES(config_updated_at),
			gmt_modified = VALUES(gmt_modified)
	`, botID, time.Unix(0, 0), jsonObjectString(body.Config), now, now, now).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save wxbot config failed")
		return
	}
	ok(c, "success", h.wxbotConfigDetail(botID))
}

func (h *Handler) AdminWechatAI(c *gin.Context, path string) {
	switch {
	case c.Request.Method == http.MethodGet && path == "/ai/status":
		ok(c, "success", h.wechatAIStatus())
	case c.Request.Method == http.MethodGet && path == "/ai/history-learning":
		ok(c, "success", gin.H{"items": h.listAIHistoryLearningTasks(c)})
	case c.Request.Method == http.MethodPost && path == "/ai/history-learning":
		h.createAIHistoryLearningTask(c)
	case c.Request.Method == http.MethodGet && path == "/ai/jobs":
		ok(c, "success", gin.H{"items": []interface{}{}})
	case c.Request.Method == http.MethodGet && strings.HasPrefix(path, "/ai/jobs/"):
		fail(c, http.StatusNotFound, CodeParamInvalid, "task not found")
	case c.Request.Method == http.MethodGet && path == "/ai/errors":
		ok(c, "success", gin.H{"items": []interface{}{}})
	case c.Request.Method == http.MethodGet && (path == "/ai/memory/runs" || path == "/ai/memory/member-profiles" || path == "/ai/memory/persona-candidates"):
		ok(c, "success", gin.H{"items": []interface{}{}})
	case c.Request.Method == http.MethodGet && path == "/ai/memory/room-persona":
		ok(c, "success", gin.H{})
	default:
		fail(c, http.StatusServiceUnavailable, CodeSystemError, "AI command must be picked up by wxbot")
	}
}

func (h *Handler) createAIHistoryLearningTask(c *gin.Context) {
	var body struct {
		RoomID      string `json:"roomId"`
		Start       string `json:"start"`
		End         string `json:"end"`
		MaxMessages int    `json:"maxMessages"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	roomID := strings.TrimSpace(body.RoomID)
	if !strings.HasSuffix(roomID, "@chatroom") {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "roomId must be a chatroom id")
		return
	}
	start, err := parseWechatAITime(body.Start, 0)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid start time")
		return
	}
	end, err := parseWechatAITime(body.End, unixNow())
	if err != nil || start >= end {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid end time")
		return
	}
	var active int64
	if err := h.db.Table(wechatAIHistoryLearningTable).
		Where("room_id = ? AND status IN ?", roomID, []string{"queued", "running"}).
		Count(&active).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query history learning failed")
		return
	}
	if active > 0 {
		fail(c, http.StatusConflict, CodeParamInvalid, "history learning already running")
		return
	}
	now := unixNow()
	if err := h.db.Exec(`
		INSERT INTO ttw_wechat_ai_history_learning_task
			(room_id, status, stage, window_start, window_end, max_messages,
			 total_msg_count, processed_msg_count, segment_job_count, created_at, updated_at)
		VALUES (?, 'queued', 'segment', ?, ?, ?, 0, 0, 0, ?, ?)
	`, roomID, start, end, max(0, body.MaxMessages), now, now).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "create history learning failed")
		return
	}
	var row aiHistoryLearningRow
	_ = h.db.Raw(`
		SELECT * FROM ttw_wechat_ai_history_learning_task
		WHERE room_id = ? AND created_at = ?
		ORDER BY id DESC LIMIT 1
	`, roomID, now).Scan(&row).Error
	ok(c, "历史聊天学习已排队", aiHistoryLearningDTO(row))
}

func (h *Handler) wxbotInstance(botID string) (wxbotInstanceRow, bool) {
	var row wxbotInstanceRow
	err := h.db.Raw("SELECT * FROM ttw_wechat_bot_instance WHERE bot_id = ? LIMIT 1", botID).Scan(&row).Error
	return row, err == nil && row.BotID != ""
}

func (h *Handler) listWxbotDTOs() []gin.H {
	var rows []wxbotInstanceRow
	_ = h.db.Raw("SELECT * FROM ttw_wechat_bot_instance ORDER BY last_seen_at DESC LIMIT 100").Scan(&rows).Error
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, wxbotDTO(row))
	}
	return items
}

func (h *Handler) wxbotConfigDetail(botID string) gin.H {
	row, found := h.wxbotInstance(botID)
	if !found {
		return gin.H{"botId": botID, "config": gin.H{}, "currentConfig": gin.H{}}
	}
	config := jsonMap(row.ConfigJSON)
	if len(config) == 0 {
		config = jsonMap(row.CurrentConfigJSON)
	}
	return gin.H{
		"botId":           row.BotID,
		"config":          config,
		"currentConfig":   jsonMap(row.CurrentConfigJSON),
		"configUpdatedAt": wxbotNullTimeString(row.ConfigUpdatedAt),
	}
}

func wxbotDTO(row wxbotInstanceRow) gin.H {
	config := jsonMap(row.ConfigJSON)
	if len(config) == 0 {
		config = jsonMap(row.CurrentConfigJSON)
	}
	return gin.H{
		"botId":           row.BotID,
		"name":            row.Name,
		"wxid":            row.Wxid,
		"status":          row.Status,
		"version":         row.Version,
		"host":            row.Host,
		"pid":             row.PID,
		"online":          time.Since(row.LastSeenAt) <= 2*time.Minute,
		"startedAt":       row.StartedAt,
		"lastSeenAt":      row.LastSeenAt.Format(time.RFC3339),
		"config":          config,
		"currentConfig":   jsonMap(row.CurrentConfigJSON),
		"configUpdatedAt": wxbotNullTimeString(row.ConfigUpdatedAt),
		"configAppliedAt": wxbotNullTimeString(row.ConfigAppliedAt),
		"updatedAt":       row.GmtModified.Format(time.RFC3339),
	}
}

func (h *Handler) wechatAIStatus() gin.H {
	var rows []wxbotInstanceRow
	_ = h.db.Raw("SELECT * FROM ttw_wechat_bot_instance ORDER BY last_seen_at DESC LIMIT 20").Scan(&rows).Error
	status := gin.H{
		"enabled":           false,
		"configured":        false,
		"running":           false,
		"autoMemoryEnabled": false,
		"queues":            gin.H{},
		"models":            gin.H{},
		"rooms":             []gin.H{},
		"recentJobs":        []interface{}{},
	}
	rooms := map[string]struct{}{}
	for index, row := range rows {
		if index == 0 {
			mergeAIStatus(status, jsonMap(row.AIStatusJSON))
			status["running"] = time.Since(row.LastSeenAt) <= 2*time.Minute
		}
		for _, roomID := range groupWhitelist(jsonMap(row.CurrentConfigJSON)) {
			rooms[roomID] = struct{}{}
		}
	}
	var roomRows []struct {
		RoomID string `gorm:"column:room_id"`
	}
	_ = h.db.Raw("SELECT DISTINCT room_id FROM ttw_wechat_ai_history_learning_task ORDER BY room_id LIMIT 100").Scan(&roomRows).Error
	for _, row := range roomRows {
		rooms[row.RoomID] = struct{}{}
	}
	roomList := make([]gin.H, 0, len(rooms))
	for roomID := range rooms {
		roomList = append(roomList, gin.H{"roomId": roomID, "activeJobs": []interface{}{}})
	}
	status["rooms"] = roomList
	return status
}

func (h *Handler) listAIHistoryLearningTasks(c *gin.Context) []gin.H {
	limit := queryLimit(c, 100, 200)
	roomID := strings.TrimSpace(c.Query("roomId"))
	query := h.db.Table(wechatAIHistoryLearningTable).Order("id DESC").Limit(limit)
	if roomID != "" {
		query = query.Where("room_id = ?", roomID)
	}
	var rows []aiHistoryLearningRow
	_ = query.Find(&rows).Error
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, aiHistoryLearningDTO(row))
	}
	return items
}

func (h *Handler) aiHistoryLearningTask(id uint64) (aiHistoryLearningRow, bool) {
	var row aiHistoryLearningRow
	err := h.db.Raw("SELECT * FROM ttw_wechat_ai_history_learning_task WHERE id = ? LIMIT 1", id).Scan(&row).Error
	return row, err == nil && row.ID != 0
}

func aiHistoryLearningDTO(row aiHistoryLearningRow) gin.H {
	return gin.H{
		"id":                row.ID,
		"botId":             row.BotID,
		"remoteTaskId":      nullInt(row.RemoteTaskID),
		"roomId":            row.RoomID,
		"status":            row.Status,
		"stage":             row.Stage,
		"windowStart":       row.WindowStart,
		"windowEnd":         row.WindowEnd,
		"maxMessages":       row.MaxMessages,
		"totalMsgCount":     row.TotalMsgCount,
		"processedMsgCount": row.ProcessedMsgCount,
		"segmentJobCount":   row.SegmentJobCount,
		"errorMessage":      nullString(row.ErrorMessage),
		"createdAt":         row.CreatedAt,
		"updatedAt":         row.UpdatedAt,
		"finishedAt":        nullFloat(row.FinishedAt),
	}
}

func (h *Handler) adminWechatBotLocal(c *gin.Context, path string) bool {
	if strings.HasPrefix(path, "/ai/") {
		h.AdminWechatAI(c, path)
		return true
	}
	if path == "/wxbots" || strings.HasPrefix(path, "/wxbots/") {
		h.AdminWxbots(c, path)
		return true
	}
	return false
}

func jsonObjectString(value map[string]interface{}) string {
	if value == nil {
		return "{}"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func jsonMap(raw string) map[string]interface{} {
	var result map[string]interface{}
	if raw == "" || json.Unmarshal([]byte(raw), &result) != nil || result == nil {
		return map[string]interface{}{}
	}
	return result
}

func mergeAIStatus(target gin.H, source map[string]interface{}) {
	for _, key := range []string{"enabled", "configured", "running", "autoMemoryEnabled", "queues", "models"} {
		if value, ok := source[key]; ok {
			target[key] = value
		}
	}
}

func groupWhitelist(config map[string]interface{}) []string {
	bot, _ := config["bot"].(map[string]interface{})
	values, _ := bot["group_whitelist"].([]interface{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		if roomID := wxbotStringValue(value); roomID != "" {
			result = append(result, roomID)
		}
	}
	return result
}

func parseWechatAITime(value string, fallback float64) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return float64(parsed.UnixNano()) / 1e9, nil
	}
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return float64(parsed.UnixNano()) / 1e9, nil
		}
	}
	return 0, strconv.ErrSyntax
}

func unixNow() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

func queryLimit(c *gin.Context, fallback, maximum int) int {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		return fallback
	}
	if limit > maximum {
		return maximum
	}
	return limit
}

func wxbotStringValue(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func int64Value(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func floatValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func nullString(value sql.NullString) interface{} {
	if value.Valid {
		return value.String
	}
	return nil
}

func nullInt(value sql.NullInt64) interface{} {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func nullFloat(value sql.NullFloat64) interface{} {
	if value.Valid {
		return value.Float64
	}
	return nil
}

func wxbotNullTimeString(value sql.NullTime) interface{} {
	if value.Valid {
		return value.Time.Format(time.RFC3339)
	}
	return nil
}
