package wechatadmin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

var aiJSONColumns = map[string]bool{
	"bot_persona_json":    true,
	"candidate_json":      true,
	"decision_json":       true,
	"evidence_msg_ids":    true,
	"evidence_run_ids":    true,
	"payload_json":        true,
	"profile_json":        true,
	"request_meta_json":   true,
	"result_json":         true,
	"room_culture_json":   true,
	"config_json":         true,
	"current_config_json": true,
}

var manualAIJobTypes = map[string]bool{
	"vector_backfill": true,
}

const defaultAIRoleCard = "你是群里的知心大姐姐，温柔、细腻、有耐心，像一个熟悉大家但不过度介入的朋友。你会认真听人说话，能察觉情绪，也会在需要时给出实际、有分寸的建议。别人问具体事情时先直接回答；别人表达委屈、犹豫或难过时，先回应感受，再说建议。说话自然、生活化、简短，不端着，不说教，不用心理咨询或客服腔。不吐槽、不阴阳怪气、不装傻、不玩梗、不恶搞，也不靠固定口癖制造人设。不刻意撒娇，不滥用亲昵称呼；亲近感来自认真回应细节，而不是甜话。不为了延续对话而追问；没有可靠记录时如实但委婉地说明没查到，不脑补，不把猜测说成事实。"

var aiPromptInstructionKeys = []string{
	"reply",
}

var defaultAIPromptInstructions = map[string]string{
	"reply": "直接回答当前问题。历史聊天只能作为“谁在何时说过什么”的原始引用，找不到原话就说明没有查到，不要推断成员画像、关系或群结论。",
}

func EnsureAIStyleDefaults(ctx context.Context, db *sql.DB) error {
	return (&Server{db: db}).ensureAIStyleTables(ctx)
}

func (s *Server) aiStatus(w http.ResponseWriter, r *http.Request) {
	queues := map[string]int{}
	if tableExists(r.Context(), s.db, "ai_jobs") {
		rows, err := s.db.QueryContext(r.Context(), "SELECT status, COUNT(*) FROM ai_jobs GROUP BY status")
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai queues failed")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err != nil {
				fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan ai queues failed")
				return
			}
			queues[status] = count
		}
	}
	aiCfg := s.latestAIConfig(r.Context())
	botOnline := s.wxbotOnline(r.Context())
	runtimeVector := s.latestAIVectorStatus(r.Context())
	vectorEnabled := boolValue(aiCfg["vector_enabled"])
	vectorConfigured := false
	vectorReason := "waiting_for_bot_heartbeat"
	vectorEmbeddingModel := firstNonEmpty(stringValue(aiCfg["vector_embedding_model"]), "qwen3.7-text-embedding")
	if runtimeVector != nil {
		vectorEnabled = boolValue(runtimeVector["enabled"])
		vectorConfigured = boolValue(runtimeVector["configured"])
		vectorReason = strings.TrimSpace(stringValue(runtimeVector["reason"]))
		vectorEmbeddingModel = firstNonEmpty(stringValue(runtimeVector["embeddingModel"]), vectorEmbeddingModel)
	}
	if !botOnline {
		vectorConfigured = false
		vectorReason = "bot_offline"
	} else if !vectorConfigured && vectorReason == "" {
		vectorReason = "vector_unavailable"
	}
	vectorState := []map[string]interface{}{}
	if tableExists(r.Context(), s.db, "ai_vector_sync_state") {
		var stateErr error
		vectorState, stateErr = s.listAIMaps(r.Context(), "SELECT * FROM ai_vector_sync_state ORDER BY updated_at DESC LIMIT 200")
		if stateErr != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query vector sync state failed")
			return
		}
		if vectorState == nil {
			vectorState = []map[string]interface{}{}
		}
	}
	rooms, err := s.aiRooms(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai rooms failed")
		return
	}
	recentJobs := []map[string]interface{}{}
	if tableExists(r.Context(), s.db, "ai_jobs") {
		var err error
		recentJobs, err = s.listAIJobs(r.Context(), "", "", 20)
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query recent ai jobs failed")
			return
		}
	}
	ok(w, map[string]interface{}{
		"enabled":    boolValue(aiCfg["enabled"]),
		"configured": strings.TrimSpace(stringValue(aiCfg["api_key"])) != "" || strings.TrimSpace(s.cfg.AIAPIKey) != "",
		"running":    botOnline,
		"queues":     queues,
		"models": map[string]string{
			"reply": firstNonEmpty(stringValue(aiCfg["reply_model"]), s.cfg.AIModel),
		},
		"vector": map[string]interface{}{
			"enabled":        vectorEnabled,
			"configured":     vectorConfigured,
			"reason":         vectorReason,
			"embeddingModel": vectorEmbeddingModel,
			"syncStates":     vectorState,
		},
		"rooms":      rooms,
		"recentJobs": recentJobs,
	})
}

func (s *Server) aiJobs(w http.ResponseWriter, r *http.Request) {
	items, err := s.listAIJobs(
		r.Context(),
		strings.TrimSpace(r.URL.Query().Get("roomId")),
		strings.TrimSpace(r.URL.Query().Get("status")),
		positiveInt(r.URL.Query().Get("limit"), 100, 1, 200),
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai jobs failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiJobDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid job id")
		return
	}
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_jobs WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "ai job not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai job failed")
		return
	}
	ok(w, row)
}

func (s *Server) createAIJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoomID  string `json:"roomId"`
		JobType string `json:"jobType"`
		Start   string `json:"start"`
		End     string `json:"end"`
		Reason  string `json:"reason"`
		Model   string `json:"model"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	roomID := strings.TrimSpace(req.RoomID)
	jobType := strings.TrimSpace(req.JobType)
	if !strings.HasSuffix(roomID, "@chatroom") || !manualAIJobTypes[jobType] {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai job")
		return
	}
	var start, end *float64
	var err error
	if aiJobRequiresWindow(jobType) {
		start, err = parseAITime(req.Start)
		if err != nil || start == nil {
			fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid start time")
			return
		}
		end, err = parseAITime(req.End)
		if err != nil || end == nil || *start >= *end {
			fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid end time")
			return
		}
	}
	model := strings.TrimSpace(req.Model)
	if jobType == "vector_backfill" && model != "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "vector backfill does not allow model override")
		return
	}
	if model == "" {
		model = s.modelForJob(r.Context(), jobType)
	}
	job, err := s.insertAIJob(r.Context(), aiJobInsert{
		RoomID:      roomID,
		JobType:     jobType,
		Model:       model,
		Reason:      firstNonEmpty(strings.TrimSpace(req.Reason), "manual_backfill"),
		WindowStart: start,
		WindowEnd:   end,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "create ai job failed")
		return
	}
	ok(w, job)
}

func (s *Server) aiHistoryLearningTasks(w http.ResponseWriter, r *http.Request) {
	where, args := "1=1", []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		where = "room_id = ?"
		args = append(args, roomID)
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_history_learning_tasks WHERE "+where+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query history learning failed")
		return
	}
	for _, item := range items {
		jobID := int64Value(item["currentJobId"])
		if jobID <= 0 {
			continue
		}
		job, err := s.oneAIMap(r.Context(), "SELECT status, input_msg_count FROM ai_jobs WHERE id = ?", jobID)
		if err != nil || stringValue(job["status"]) != "running" {
			continue
		}
		projectHistoryLearningProgress(item, job)
	}
	ok(w, map[string]interface{}{"items": items})
}

func projectHistoryLearningProgress(task, job map[string]interface{}) {
	if processed := int64Value(job["inputMsgCount"]); processed > int64Value(task["processedMsgCount"]) {
		task["processedMsgCount"] = processed
	}
}

func (s *Server) createAIHistoryLearningTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoomID      string `json:"roomId"`
		Start       string `json:"start"`
		End         string `json:"end"`
		MaxMessages int    `json:"maxMessages"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	roomID := strings.TrimSpace(req.RoomID)
	if !strings.HasSuffix(roomID, "@chatroom") {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId must be a chatroom id")
		return
	}
	start, err := parseAITime(req.Start)
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid start time")
		return
	}
	end, err := parseAITime(req.End)
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid end time")
		return
	}
	startValue := 0.0
	if start != nil {
		startValue = *start
	}
	endValue := float64(time.Now().UnixNano()) / 1e9
	if end != nil {
		endValue = *end
	}
	if startValue >= endValue {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid time range")
		return
	}
	var active int
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM ai_history_learning_tasks
		WHERE room_id = ? AND status IN ('queued', 'running', 'paused')
	`, roomID).Scan(&active); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query history learning failed")
		return
	}
	if active > 0 {
		fail(w, http.StatusConflict, "PARAM_INVALID", "history learning already running")
		return
	}
	maxMessages := req.MaxMessages
	if maxMessages < 0 {
		maxMessages = 0
	}
	total, err := s.countTextMessages(r.Context(), roomID, startValue, endValue)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count history messages failed")
		return
	}
	if maxMessages > 0 && total > maxMessages {
		total = maxMessages
	}
	if total <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "no text messages in range")
		return
	}
	now := float64(time.Now().UnixNano()) / 1e9
	result, err := s.db.ExecContext(r.Context(), `
		INSERT INTO ai_history_learning_tasks
			(room_id, status, stage, window_start, window_end, max_messages, total_msg_count,
			 processed_msg_count, created_at, updated_at)
		VALUES (?, 'queued', 'vector_backfill', ?, ?, ?, ?, 0, ?, ?)
	`, roomID, startValue, endValue, maxMessages, total, now, now)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "create history learning failed")
		return
	}
	id, _ := result.LastInsertId()
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_history_learning_tasks WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query history learning failed")
		return
	}
	ok(w, row)
}

func (s *Server) updateAIHistoryLearningTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	action := strings.TrimSpace(r.PathValue("action"))
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid history learning id")
		return
	}
	task, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_history_learning_tasks WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "history learning not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query history learning failed")
		return
	}
	now := float64(time.Now().UnixNano()) / 1e9
	status := stringValue(task["status"])
	switch action {
	case "pause":
		if status != "queued" && status != "running" {
			fail(w, http.StatusConflict, "PARAM_INVALID", "history learning cannot be paused")
			return
		}
		_, err = s.db.ExecContext(r.Context(), "UPDATE ai_history_learning_tasks SET status = 'paused', updated_at = ? WHERE id = ?", now, id)
	case "resume":
		if status != "paused" {
			fail(w, http.StatusConflict, "PARAM_INVALID", "history learning is not paused")
			return
		}
		_, err = s.db.ExecContext(r.Context(), "UPDATE ai_history_learning_tasks SET status = 'queued', updated_at = ? WHERE id = ?", now, id)
	case "cancel":
		if status == "succeeded" || status == "canceled" {
			fail(w, http.StatusConflict, "PARAM_INVALID", "history learning cannot be canceled")
			return
		}
		_, err = s.db.ExecContext(r.Context(), "UPDATE ai_history_learning_tasks SET status = 'canceled', current_job_id = NULL, finished_at = ?, updated_at = ? WHERE id = ?", now, now, id)
	case "retry":
		if status != "failed" {
			fail(w, http.StatusConflict, "PARAM_INVALID", "history learning is not failed")
			return
		}
		if jobID := int64Value(task["currentJobId"]); jobID > 0 {
			if _, err = s.db.ExecContext(r.Context(), "UPDATE ai_jobs SET status = 'queued', started_at = NULL, finished_at = NULL, error_id = NULL WHERE id = ? AND status = 'failed'", jobID); err != nil {
				fail(w, http.StatusInternalServerError, "SAVE_FAILED", "retry history learning job failed")
				return
			}
		}
		_, err = s.db.ExecContext(r.Context(), "UPDATE ai_history_learning_tasks SET status = 'queued', error_message = NULL, finished_at = NULL, updated_at = ? WHERE id = ?", now, id)
	default:
		fail(w, http.StatusNotFound, "NOT_FOUND", "history learning action not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "update history learning failed")
		return
	}
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_history_learning_tasks WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query history learning failed")
		return
	}
	ok(w, row)
}

func (s *Server) aiErrors(w http.ResponseWriter, r *http.Request) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		conditions = append(conditions, "room_id = ?")
		args = append(args, roomID)
	}
	if parseBool(r.URL.Query().Get("unresolvedOnly")) {
		conditions = append(conditions, "resolved = 0")
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_job_errors WHERE "+strings.Join(conditions, " AND ")+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai errors failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) retryAIError(w http.ResponseWriter, r *http.Request) {
	errorID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || errorID <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid error id")
		return
	}
	var exists int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM ai_jobs WHERE retry_of_error_id = ?", errorID).Scan(&exists); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query retry job failed")
		return
	}
	if exists > 0 {
		fail(w, http.StatusConflict, "PARAM_INVALID", "retry job already exists")
		return
	}
	job, err := s.jobForError(r.Context(), errorID)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "ai error not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai error failed")
		return
	}
	retry, err := s.insertAIJob(r.Context(), aiJobInsert{
		RoomID:         stringValue(job["roomId"]),
		JobType:        stringValue(job["jobType"]),
		Model:          stringValue(job["model"]),
		Reason:         "retry",
		WindowStart:    floatPtr(job["windowStart"]),
		WindowEnd:      floatPtr(job["windowEnd"]),
		SourceMsgID:    stringValue(job["sourceMsgId"]),
		RetryOfErrorID: &errorID,
		DedupeSuffix:   "retry:" + strconv.FormatInt(errorID, 10),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "retry ai job failed")
		return
	}
	ok(w, retry)
}

func (s *Server) resolveAIError(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid error id")
		return
	}
	result, err := s.db.ExecContext(r.Context(), "UPDATE ai_job_errors SET resolved = 1 WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "resolve ai error failed")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		fail(w, http.StatusNotFound, "NOT_FOUND", "ai error not found")
		return
	}
	ok(w, map[string]interface{}{"resolved": true})
}

func (s *Server) aiObservation(w http.ResponseWriter, r *http.Request) {
	days := positiveInt(r.URL.Query().Get("days"), 7, 1, 90)
	since := float64(time.Now().Add(-time.Duration(days)*24*time.Hour).UnixNano()) / 1e9
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	roomClause, args := "", []interface{}{since}
	if roomID != "" {
		roomClause = " AND room_id = ?"
		args = append(args, roomID)
	}
	jobStats := []map[string]interface{}{}
	if tableExists(r.Context(), s.db, "ai_jobs") {
		var err error
		jobStats, err = s.listAIMaps(r.Context(), `
			SELECT job_type, status, COUNT(*) AS count, AVG(input_msg_count) AS avg_input_msg_count
			FROM ai_jobs
			WHERE created_at >= ?`+roomClause+`
			GROUP BY job_type, status
			ORDER BY job_type ASC, status ASC
		`, args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai job observation failed")
			return
		}
	}
	memoryStats := map[string]interface{}{"segmentCount": 0, "avgQualityScore": nil, "lowQualityCount": 0}
	if tableExists(r.Context(), s.db, "ai_memory_runs") {
		qualityExpr := "CAST(JSON_UNQUOTE(JSON_EXTRACT(result_json, '$.qualityScore')) AS DECIMAL(10,2))"
		row, err := s.oneAIMap(r.Context(), `
			SELECT COUNT(*) AS segment_count,
				AVG(`+qualityExpr+`) AS avg_quality_score,
				SUM(CASE WHEN `+qualityExpr+` < 60 THEN 1 ELSE 0 END) AS low_quality_count
			FROM ai_memory_runs
			WHERE granularity = 'segment' AND created_at >= ?`+roomClause, args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai memory observation failed")
			return
		}
		memoryStats = row
	}
	activeLearning := []map[string]interface{}{}
	if tableExists(r.Context(), s.db, "ai_history_learning_tasks") {
		queryArgs := []interface{}{}
		where := "status IN ('queued', 'running', 'paused')"
		if roomID != "" {
			where += " AND room_id = ?"
			queryArgs = append(queryArgs, roomID)
		}
		var err error
		activeLearning, err = s.listAIMaps(r.Context(), "SELECT * FROM ai_history_learning_tasks WHERE "+where+" ORDER BY id DESC LIMIT 20", queryArgs...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query active learning failed")
			return
		}
	}
	recentErrors := []map[string]interface{}{}
	if tableExists(r.Context(), s.db, "ai_job_errors") {
		queryArgs := []interface{}{}
		where := "resolved = 0"
		if roomID != "" {
			where += " AND room_id = ?"
			queryArgs = append(queryArgs, roomID)
		}
		var err error
		recentErrors, err = s.listAIMaps(r.Context(), "SELECT * FROM ai_job_errors WHERE "+where+" ORDER BY id DESC LIMIT 10", queryArgs...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query recent errors failed")
			return
		}
	}
	recentVersions := []map[string]interface{}{}
	if err := s.ensureAIPersonaVersionTable(r.Context()); err == nil {
		queryArgs := []interface{}{}
		where := "1=1"
		if roomID != "" {
			where = "room_id = ?"
			queryArgs = append(queryArgs, roomID)
		}
		recentVersions, _ = s.listAIMaps(r.Context(), "SELECT * FROM ai_persona_versions WHERE "+where+" ORDER BY id DESC LIMIT 10", queryArgs...)
	}
	ok(w, map[string]interface{}{
		"days":           days,
		"roomId":         roomID,
		"jobStats":       jobStats,
		"memoryStats":    memoryStats,
		"activeLearning": activeLearning,
		"recentErrors":   recentErrors,
		"recentVersions": recentVersions,
	})
}

func (s *Server) aiRoleCard(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	row, err := s.oneAIMap(r.Context(), "SELECT content, updated_at FROM ai_role_cards WHERE id = 1")
	if errors.Is(err, sql.ErrNoRows) {
		ok(w, map[string]interface{}{"content": defaultAIRoleCard, "isDefault": true})
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai role card failed")
		return
	}
	ok(w, row)
}

func (s *Server) updateAIRoleCard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	content := strings.TrimSpace(req.Content)
	if len(content) > 8000 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "role card is too long")
		return
	}
	if content == "" {
		if _, err := s.db.ExecContext(r.Context(), "DELETE FROM ai_role_cards WHERE id = 1"); err != nil {
			fail(w, http.StatusInternalServerError, "SAVE_FAILED", "reset ai role card failed")
			return
		}
		ok(w, map[string]interface{}{"content": defaultAIRoleCard, "isDefault": true})
		return
	}
	now := float64(time.Now().UnixNano()) / 1e9
	if _, err := s.db.ExecContext(r.Context(), `
		INSERT INTO ai_role_cards (id, content, updated_at)
		VALUES (1, ?, ?)
		ON DUPLICATE KEY UPDATE content = VALUES(content), updated_at = VALUES(updated_at)
	`, content, now); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save ai role card failed")
		return
	}
	ok(w, map[string]interface{}{"content": content, "updatedAt": now, "isDefault": false})
}

func (s *Server) aiPromptInstructions(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	rows, err := s.listAIMaps(r.Context(), "SELECT instruction_key, content, updated_at FROM ai_prompt_instructions")
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai prompt instructions failed")
		return
	}
	byKey := make(map[string]map[string]interface{}, len(rows))
	for _, row := range rows {
		byKey[stringValue(row["instructionKey"])] = row
	}
	items := make([]map[string]interface{}, 0, len(aiPromptInstructionKeys))
	for _, key := range aiPromptInstructionKeys {
		item := map[string]interface{}{"key": key, "content": ""}
		if row := byKey[key]; row != nil {
			item["content"] = stringValue(row["content"])
			item["updatedAt"] = row["updatedAt"]
		}
		items = append(items, item)
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) updateAIPromptInstruction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key     string `json:"key"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	key := strings.TrimSpace(req.Key)
	if !validAIPromptInstructionKey(key) {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai prompt instruction key")
		return
	}
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		content = defaultAIPromptInstructions[key]
	}
	if len(content) > 4000 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "ai prompt instruction is too long")
		return
	}
	now := float64(time.Now().UnixNano()) / 1e9
	if _, err := s.db.ExecContext(r.Context(), `
		INSERT INTO ai_prompt_instructions (instruction_key, content, updated_at)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE content = VALUES(content), updated_at = VALUES(updated_at)
	`, key, content, now); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save ai prompt instruction failed")
		return
	}
	ok(w, map[string]interface{}{"key": key, "content": content, "updatedAt": now})
}

func validAIPromptInstructionKey(key string) bool {
	for _, allowed := range aiPromptInstructionKeys {
		if key == allowed {
			return true
		}
	}
	return false
}

func (s *Server) aiReplyStyleSamples(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	where, args := "1=1", []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		where = "room_id = ?"
		args = append(args, roomID)
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_reply_style_samples WHERE "+where+" ORDER BY updated_at DESC, id DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai reply samples failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) createAIReplyStyleSample(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoomID      string `json:"roomId"`
		Scenario    string `json:"scenario"`
		TriggerText string `json:"triggerText"`
		ReplyText   string `json:"replyText"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	roomID := strings.TrimSpace(req.RoomID)
	scenario := strings.TrimSpace(req.Scenario)
	triggerText := strings.TrimSpace(req.TriggerText)
	replyText := strings.TrimSpace(req.ReplyText)
	if (roomID != "" && !strings.HasSuffix(roomID, "@chatroom")) || triggerText == "" || replyText == "" || len(scenario) > 32 || len(triggerText) > 2000 || len(replyText) > 1000 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai reply sample")
		return
	}
	now := float64(time.Now().UnixNano()) / 1e9
	result, err := s.db.ExecContext(r.Context(), `
		INSERT INTO ai_reply_style_samples (room_id, scenario, trigger_text, reply_text, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, roomID, scenario, triggerText, replyText, now, now)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "create ai reply sample failed")
		return
	}
	id, _ := result.LastInsertId()
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_reply_style_samples WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai reply sample failed")
		return
	}
	ok(w, row)
}

func (s *Server) deleteAIReplyStyleSample(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai reply sample id")
		return
	}
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	result, err := s.db.ExecContext(r.Context(), "DELETE FROM ai_reply_style_samples WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "delete ai reply sample failed")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		fail(w, http.StatusNotFound, "NOT_FOUND", "ai reply sample not found")
		return
	}
	ok(w, map[string]interface{}{"deleted": true})
}

func (s *Server) aiReplyConversationSamples(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	where, args := "1=1", []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		where = "room_id = ?"
		args = append(args, roomID)
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_reply_conversation_samples WHERE "+where+" ORDER BY updated_at DESC, id DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai reply conversation samples failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) createAIReplyConversationSample(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoomID      string `json:"roomId"`
		Scenario    string `json:"scenario"`
		ContextText string `json:"contextText"`
		ReplyText   string `json:"replyText"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	roomID := strings.TrimSpace(req.RoomID)
	scenario := strings.TrimSpace(req.Scenario)
	contextText := strings.TrimSpace(req.ContextText)
	replyText := strings.TrimSpace(req.ReplyText)
	if (roomID != "" && !strings.HasSuffix(roomID, "@chatroom")) || contextText == "" || replyText == "" || len(scenario) > 32 || len(contextText) > 8000 || len(replyText) > 1000 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai reply conversation sample")
		return
	}
	now := float64(time.Now().UnixNano()) / 1e9
	result, err := s.db.ExecContext(r.Context(), `
		INSERT INTO ai_reply_conversation_samples (room_id, scenario, context_text, reply_text, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, roomID, scenario, contextText, replyText, now, now)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "create ai reply conversation sample failed")
		return
	}
	id, _ := result.LastInsertId()
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_reply_conversation_samples WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai reply conversation sample failed")
		return
	}
	ok(w, row)
}

func (s *Server) deleteAIReplyConversationSample(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai reply conversation sample id")
		return
	}
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	result, err := s.db.ExecContext(r.Context(), "DELETE FROM ai_reply_conversation_samples WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "delete ai reply conversation sample failed")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		fail(w, http.StatusNotFound, "NOT_FOUND", "ai reply conversation sample not found")
		return
	}
	ok(w, map[string]interface{}{"deleted": true})
}

func (s *Server) aiReplyLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	if !tableExists(r.Context(), s.db, "ai_reply_logs") {
		ok(w, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}
	where, args := "l.reply_text <> ''", []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		where += " AND l.room_id = ?"
		args = append(args, roomID)
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), `
		SELECT l.*, COALESCE(m.content, '') AS trigger_content,
			COALESCE(f.feedback, '') AS feedback, f.updated_at AS feedback_at
		FROM ai_reply_logs l
		LEFT JOIN group_messages m ON m.msg_id = l.trigger_msg_id
		LEFT JOIN ai_reply_log_feedbacks f ON f.reply_log_id = l.id
		WHERE `+where+` ORDER BY l.id DESC LIMIT ?`, args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai reply logs failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) reviewAIReplyLog(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai reply log id")
		return
	}
	var req struct {
		Feedback string `json:"feedback"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	feedback := strings.TrimSpace(req.Feedback)
	if !validAIReplyFeedback(feedback) {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid ai reply feedback")
		return
	}
	if err := s.ensureAIStyleTables(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure ai style tables failed")
		return
	}
	log, err := s.oneAIMap(r.Context(), `
		SELECT l.id, l.room_id, l.reply_text, COALESCE(m.content, '') AS trigger_content
		FROM ai_reply_logs l
		LEFT JOIN group_messages m ON m.msg_id = l.trigger_msg_id
		WHERE l.id = ?
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "ai reply log not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai reply log failed")
		return
	}
	now := float64(time.Now().UnixNano()) / 1e9
	if _, err := s.db.ExecContext(r.Context(), `
		INSERT INTO ai_reply_log_feedbacks (reply_log_id, feedback, updated_at)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE feedback = VALUES(feedback), updated_at = VALUES(updated_at)
	`, id, feedback, now); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save ai reply feedback failed")
		return
	}
	sampleActive := false
	if feedback == "human" {
		triggerText := strings.TrimSpace(stringValue(log["triggerContent"]))
		replyText := strings.TrimSpace(stringValue(log["replyText"]))
		if triggerText != "" && replyText != "" {
			_, err = s.db.ExecContext(r.Context(), `
				INSERT INTO ai_reply_style_samples
					(room_id, scenario, trigger_text, reply_text, source_reply_log_id, created_at, updated_at)
				VALUES (?, 'general', ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE room_id = VALUES(room_id), trigger_text = VALUES(trigger_text),
					reply_text = VALUES(reply_text), updated_at = VALUES(updated_at)
			`, stringValue(log["roomId"]), triggerText, replyText, id, now, now)
			if err != nil {
				fail(w, http.StatusInternalServerError, "SAVE_FAILED", "promote ai reply sample failed")
				return
			}
			sampleActive = true
		}
	} else if _, err := s.db.ExecContext(r.Context(), "DELETE FROM ai_reply_style_samples WHERE source_reply_log_id = ?", id); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "remove ai reply sample failed")
		return
	}
	ok(w, map[string]interface{}{"feedback": feedback, "sampleActive": sampleActive})
}

func validAIReplyFeedback(value string) bool {
	return value == "human" || value == "too_ai" || value == "too_much"
}

func (s *Server) aiMemoryRuns(w http.ResponseWriter, r *http.Request) {
	where, args := "1=1", []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		where = "room_id = ?"
		args = append(args, roomID)
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_memory_runs WHERE "+where+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai memory runs failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiGroupFacts(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	if !tableExists(r.Context(), s.db, "ai_group_facts") {
		ok(w, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}

	conditions, args := []string{"room_id = ?"}, []interface{}{roomID}
	if state := strings.TrimSpace(r.URL.Query().Get("state")); validFactState(state) {
		conditions = append(conditions, "state = ?")
		args = append(args, state)
	} else {
		conditions = append(conditions, "state IN ('active', 'disputed')")
	}
	if members := queryValues(r, "memberWxids"); len(members) > 0 {
		conditions = append(conditions, "subject_key IN ("+placeholders(len(members))+")")
		for _, member := range members {
			args = append(args, member)
		}
	}
	if query := strings.TrimSpace(r.URL.Query().Get("query")); query != "" {
		conditions = append(conditions, "(content LIKE ? ESCAPE '\\\\' OR predicate LIKE ? ESCAPE '\\\\')")
		args = append(args, likePattern(query), likePattern(query))
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_group_facts WHERE "+strings.Join(conditions, " AND ")+" ORDER BY CASE state WHEN 'active' THEN 0 ELSE 1 END, updated_at DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai facts failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiRelationships(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	if !tableExists(r.Context(), s.db, "ai_relationship_edges") {
		ok(w, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}

	conditions, args := []string{"room_id = ?"}, []interface{}{roomID}
	members := queryValues(r, "memberWxids")
	if len(members) == 1 {
		conditions = append(conditions, "(left_member_wxid = ? OR right_member_wxid = ?)")
		args = append(args, members[0], members[0])
	} else if len(members) > 1 {
		memberPlaceholders := placeholders(len(members))
		conditions = append(conditions, "left_member_wxid IN ("+memberPlaceholders+")", "right_member_wxid IN ("+memberPlaceholders+")")
		for _, member := range members {
			args = append(args, member)
		}
		for _, member := range members {
			args = append(args, member)
		}
	}
	if state := strings.TrimSpace(r.URL.Query().Get("state")); validFactState(state) {
		conditions = append(conditions, "state = ?")
		args = append(args, state)
	} else {
		conditions = append(conditions, "state IN ('active', 'disputed')")
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_relationship_edges WHERE "+strings.Join(conditions, " AND ")+" ORDER BY updated_at DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai relationships failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiGroupEvents(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	if !tableExists(r.Context(), s.db, "ai_group_events") {
		ok(w, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}

	conditions, args := []string{"room_id = ?"}, []interface{}{roomID}
	if state := strings.TrimSpace(r.URL.Query().Get("state")); validEventState(state) {
		conditions = append(conditions, "state = ?")
		args = append(args, state)
	} else {
		conditions = append(conditions, "state = 'active'")
	}
	if query := strings.TrimSpace(r.URL.Query().Get("query")); query != "" {
		conditions = append(conditions, "(summary LIKE ? ESCAPE '\\\\' OR event_type LIKE ? ESCAPE '\\\\')")
		args = append(args, likePattern(query), likePattern(query))
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_group_events WHERE "+strings.Join(conditions, " AND ")+" ORDER BY last_evidence_at DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai events failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiInterventions(w http.ResponseWriter, r *http.Request) {
	if !tableExists(r.Context(), s.db, "ai_interventions") {
		ok(w, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}
	conditions, args := []string{}, []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		conditions = append(conditions, "room_id = ?")
		args = append(args, roomID)
	}
	if state := strings.TrimSpace(r.URL.Query().Get("state")); validInterventionState(state) {
		conditions = append(conditions, "state = ?")
		args = append(args, state)
	}
	where := "1=1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_interventions WHERE "+where+" ORDER BY updated_at DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai interventions failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiMemoryFeedbacks(w http.ResponseWriter, r *http.Request) {
	if !tableExists(r.Context(), s.db, "ai_memory_feedbacks") {
		ok(w, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}
	conditions, args := []string{}, []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		conditions = append(conditions, "room_id = ?")
		args = append(args, roomID)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); validFeedbackStatus(status) {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	where := "1=1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_memory_feedbacks WHERE "+where+" ORDER BY created_at DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai memory feedbacks failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiProactiveConfig(w http.ResponseWriter, r *http.Request) {
	ai, err := s.latestDesiredAIConfig(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query ai config failed")
		return
	}
	ok(w, proactiveConfigPayload(ai))
}

func (s *Server) updateAIProactiveConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&updates); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	if !validProactiveConfigUpdates(updates) {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid proactive ai config")
		return
	}
	if err := s.ensureWxbotSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure wxbot schema failed")
		return
	}

	var botID string
	var raw json.RawMessage
	err := s.db.QueryRowContext(r.Context(), `
		SELECT bot_id, COALESCE(config_json, JSON_OBJECT())
		FROM wxbot_agents
		ORDER BY last_seen_at DESC, bot_id
		LIMIT 1
	`).Scan(&botID, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "wxbot not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query wxbot config failed")
		return
	}
	config, err := unwrapWxbotConfig(raw)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "parse wxbot config failed")
		return
	}
	ai, _ := config["ai"].(map[string]interface{})
	if ai == nil {
		ai = map[string]interface{}{}
	}
	for key, value := range updates {
		ai[key] = value
	}
	config["ai"] = ai
	merged, _ := json.Marshal(config)
	normalized, err := normalizeWxbotConfig(merged)
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
		return
	}
	if _, err := s.db.ExecContext(r.Context(), `
		UPDATE wxbot_agents
		SET config_json = ?, config_updated_at = NOW(), updated_at = NOW()
		WHERE bot_id = ?
	`, string(normalized), botID); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save ai config failed")
		return
	}
	updated, err := unwrapWxbotConfig(normalized)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "parse saved ai config failed")
		return
	}
	updatedAI, _ := updated["ai"].(map[string]interface{})
	ok(w, proactiveConfigPayload(updatedAI))
}

func (s *Server) latestDesiredAIConfig(ctx context.Context) (map[string]interface{}, error) {
	if !tableExists(ctx, s.db, "wxbot_agents") {
		return map[string]interface{}{}, nil
	}
	var raw json.RawMessage
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(config_json, JSON_OBJECT())
		FROM wxbot_agents
		ORDER BY last_seen_at DESC, bot_id
		LIMIT 1
	`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	config, err := unwrapWxbotConfig(raw)
	if err != nil {
		return nil, err
	}
	ai, _ := config["ai"].(map[string]interface{})
	return ai, nil
}

func proactiveConfigPayload(ai map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"proactiveEnabled":                 boolValue(ai["proactive_enabled"]),
		"proactiveObserverIntervalSeconds": proactiveConfigInt(ai["proactive_observer_interval_seconds"], 60, 30),
		"proactiveSettleSeconds":           proactiveConfigInt(ai["proactive_settle_seconds"], 90, 30),
		"proactiveTimeoutSeconds":          proactiveConfigInt(ai["proactive_timeout_seconds"], 45, 5),
	}
}

func proactiveConfigInt(value interface{}, fallback, minimum int) int {
	number := int64Value(value)
	if number < int64(minimum) {
		return fallback
	}
	return int(number)
}

func validProactiveConfigUpdates(updates map[string]interface{}) bool {
	if len(updates) == 0 {
		return false
	}
	if enabled, ok := updates["proactive_enabled"]; ok {
		if _, ok := enabled.(bool); !ok {
			return false
		}
	}
	minimums := map[string]int{
		"proactive_observer_interval_seconds": 30,
		"proactive_settle_seconds":            30,
		"proactive_timeout_seconds":           5,
	}
	for key, value := range updates {
		minimum, allowed := minimums[key]
		if key == "proactive_enabled" {
			continue
		}
		if !allowed {
			return false
		}
		number, ok := value.(float64)
		if !ok || number != float64(int64Value(value)) || int64Value(value) < int64(minimum) {
			return false
		}
	}
	return true
}

func validFactState(value string) bool {
	return value == "active" || value == "disputed" || value == "revised" || value == "retracted"
}

func validEventState(value string) bool {
	return value == "active" || value == "resolved" || value == "expired"
}

func validInterventionState(value string) bool {
	return value == "new" || value == "addressed" || value == "reopened"
}

func validFeedbackStatus(value string) bool {
	return value == "pending" || value == "applied" || value == "rejected"
}

func queryValues(r *http.Request, key string) []string {
	values, seen := []string{}, map[string]struct{}{}
	for _, raw := range r.URL.Query()[key] {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	return values
}

func placeholders(count int) string {
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func (s *Server) aiRoomPersona(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	if roomID == "" {
		rows, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_room_persona ORDER BY updated_at DESC LIMIT 200")
		if err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query room persona failed")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, row := range rows {
			items = append(items, map[string]interface{}{"roomId": row["roomId"], "persona": row})
		}
		ok(w, map[string]interface{}{"items": items})
		return
	}
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_room_persona WHERE room_id = ?", roomID)
	if errors.Is(err, sql.ErrNoRows) {
		ok(w, map[string]interface{}{"roomId": roomID, "roomCultureJson": map[string]interface{}{}, "botPersonaJson": map[string]interface{}{}})
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query room persona failed")
		return
	}
	ok(w, row)
}

func (s *Server) aiMemberProfiles(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_member_profiles WHERE room_id = ? ORDER BY updated_at DESC", roomID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query member profiles failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiPersonaCandidates(w http.ResponseWriter, r *http.Request) {
	where, args := "1=1", []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		where = "room_id = ?"
		args = append(args, roomID)
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_persona_candidates WHERE "+where+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona candidates failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) aiPersonaCandidateEvidence(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid candidate id")
		return
	}
	candidate, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_persona_candidates WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "persona candidate not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona candidate failed")
		return
	}
	runs, err := s.aiMemoryRunsByIDs(r.Context(), int64Slice(candidate["evidenceRunIds"]))
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona evidence runs failed")
		return
	}
	messages, err := s.messagesByIDs(r.Context(), evidenceMessageIDs(runs))
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona evidence messages failed")
		return
	}
	ok(w, map[string]interface{}{"candidate": candidate, "runs": runs, "messages": messages})
}

func (s *Server) aiPersonaVersions(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureAIPersonaVersionTable(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure persona versions failed")
		return
	}
	where, args := "1=1", []interface{}{}
	if roomID := strings.TrimSpace(r.URL.Query().Get("roomId")); roomID != "" {
		where = "room_id = ?"
		args = append(args, roomID)
	}
	args = append(args, positiveInt(r.URL.Query().Get("limit"), 100, 1, 200))
	items, err := s.listAIMaps(r.Context(), "SELECT * FROM ai_persona_versions WHERE "+where+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona versions failed")
		return
	}
	ok(w, map[string]interface{}{"items": items})
}

func (s *Server) rollbackAIPersonaVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid persona version id")
		return
	}
	if err := s.ensureAIPersonaVersionTable(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "ensure persona versions failed")
		return
	}
	version, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_persona_versions WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "persona version not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona version failed")
		return
	}
	roomID := stringValue(version["roomId"])
	current, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_room_persona WHERE room_id = ?", roomID)
	if err == nil {
		_ = s.insertPersonaVersion(r.Context(), roomID, current["roomCultureJson"], current["botPersonaJson"], "rollback_backup", &id, "rollback previous active persona")
	} else if !errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query current persona failed")
		return
	}
	cultureJSON, _ := json.Marshal(version["roomCultureJson"])
	personaJSON, _ := json.Marshal(version["botPersonaJson"])
	now := float64(time.Now().UnixNano()) / 1e9
	if _, err := s.db.ExecContext(r.Context(), `
		INSERT INTO ai_room_persona (room_id, room_culture_json, bot_persona_json, updated_at)
		VALUES (?, CAST(? AS JSON), CAST(? AS JSON), ?)
		ON DUPLICATE KEY UPDATE room_culture_json = VALUES(room_culture_json),
			bot_persona_json = VALUES(bot_persona_json), updated_at = VALUES(updated_at)
	`, roomID, string(cultureJSON), string(personaJSON), now); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "rollback persona failed")
		return
	}
	if err := s.insertPersonaVersion(r.Context(), roomID, version["roomCultureJson"], version["botPersonaJson"], "rollback", &id, "rollback applied"); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "record rollback version failed")
		return
	}
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_room_persona WHERE room_id = ?", roomID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query room persona failed")
		return
	}
	ok(w, map[string]interface{}{"persona": row, "rolledBackFrom": version})
}

func (s *Server) promoteAIPersonaCandidate(w http.ResponseWriter, r *http.Request) {
	s.reviewAIPersonaCandidate(w, r, "promoted")
}

func (s *Server) rejectAIPersonaCandidate(w http.ResponseWriter, r *http.Request) {
	s.reviewAIPersonaCandidate(w, r, "rejected")
}

func (s *Server) reviewAIPersonaCandidate(w http.ResponseWriter, r *http.Request, status string) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid candidate id")
		return
	}
	candidate, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_persona_candidates WHERE id = ? AND status = 'pending'", id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "persona candidate not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona candidate failed")
		return
	}
	if status == "promoted" {
		if err := s.promoteCandidatePersona(r.Context(), candidate); err != nil {
			fail(w, http.StatusInternalServerError, "SAVE_FAILED", "promote persona candidate failed")
			return
		}
	}
	now := float64(time.Now().UnixNano()) / 1e9
	if _, err := s.db.ExecContext(r.Context(), "UPDATE ai_persona_candidates SET status = ?, reviewed_at = ? WHERE id = ?", status, now, id); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "review persona candidate failed")
		return
	}
	row, err := s.oneAIMap(r.Context(), "SELECT * FROM ai_persona_candidates WHERE id = ?", id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query persona candidate failed")
		return
	}
	ok(w, row)
}

func (s *Server) wxbotNextHistoryLearningTask(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]interface{}{"task": nil})
}

func (s *Server) wxbotHistoryLearningProgress(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]interface{}{"updated": true})
}

type aiJobInsert struct {
	RoomID         string
	JobType        string
	Model          string
	Reason         string
	WindowStart    *float64
	WindowEnd      *float64
	SourceMsgID    string
	RetryOfErrorID *int64
	DedupeSuffix   string
}

func (s *Server) insertAIJob(ctx context.Context, input aiJobInsert) (map[string]interface{}, error) {
	dedupe := aiDedupeKey(input)
	_, err := s.db.ExecContext(ctx, `
		INSERT IGNORE INTO ai_jobs
			(room_id, job_type, status, model, window_start, window_end,
			 window_start_msg_id, window_end_msg_id, source_msg_id,
			 retry_of_error_id, reason, dedupe_key, created_at)
		VALUES (?, ?, 'queued', ?, ?, ?, '', '', ?, ?, ?, ?, ?)
	`, input.RoomID, input.JobType, input.Model, nullableFloat(input.WindowStart), nullableFloat(input.WindowEnd), input.SourceMsgID,
		nullableInt(input.RetryOfErrorID), input.Reason, dedupe, float64(time.Now().UnixNano())/1e9)
	if err != nil {
		return nil, err
	}
	return s.oneAIMap(ctx, "SELECT * FROM ai_jobs WHERE dedupe_key = ?", dedupe)
}

func aiDedupeKey(input aiJobInsert) string {
	type payload struct {
		JobType     string   `json:"job_type"`
		RoomID      string   `json:"room_id"`
		WindowStart *float64 `json:"window_start"`
		WindowEnd   *float64 `json:"window_end"`
		SourceMsgID string   `json:"source_msg_id"`
		Suffix      string   `json:"suffix"`
	}
	raw, _ := json.Marshal(payload{
		JobType:     input.JobType,
		RoomID:      input.RoomID,
		WindowStart: input.WindowStart,
		WindowEnd:   input.WindowEnd,
		SourceMsgID: input.SourceMsgID,
		Suffix:      "::" + input.DedupeSuffix,
	})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Server) listAIJobs(ctx context.Context, roomID, status string, limit int) ([]map[string]interface{}, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	if roomID != "" {
		conditions = append(conditions, "room_id = ?")
		args = append(args, roomID)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	args = append(args, limit)
	return s.listAIMaps(ctx, "SELECT * FROM ai_jobs WHERE "+strings.Join(conditions, " AND ")+" ORDER BY id DESC LIMIT ?", args...)
}

func (s *Server) listAIMaps(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAIRows(rows)
}

func (s *Server) oneAIMap(ctx context.Context, query string, args ...interface{}) (map[string]interface{}, error) {
	rows, err := s.listAIMaps(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rows[0], nil
}

func scanAIRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{}, len(columns))
		for i, column := range columns {
			row[snakeToCamel(column)] = normalizeAIValue(column, values[i])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func normalizeAIValue(column string, value interface{}) interface{} {
	if value == nil {
		return nil
	}
	if column == "resolved" {
		return parseBool(stringValue(value))
	}
	if raw, ok := value.([]byte); ok {
		text := string(raw)
		if aiJSONColumns[column] {
			return parseJSONValue(text, defaultJSONValue(column))
		}
		return text
	}
	if aiJSONColumns[column] {
		return parseJSONValue(stringValue(value), defaultJSONValue(column))
	}
	return value
}

func parseJSONValue(raw string, fallback interface{}) interface{} {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fallback
	}
	return value
}

func defaultJSONValue(column string) interface{} {
	if column == "evidence_msg_ids" || column == "evidence_run_ids" {
		return []interface{}{}
	}
	return map[string]interface{}{}
}

func snakeToCamel(value string) string {
	parts := strings.Split(value, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func (s *Server) latestAIConfig(ctx context.Context) map[string]interface{} {
	if !tableExists(ctx, s.db, "wxbot_agents") {
		return map[string]interface{}{}
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(current_config_json, config_json, JSON_OBJECT())
		FROM wxbot_agents
		ORDER BY last_seen_at DESC, bot_id
		LIMIT 1
	`).Scan(&raw)
	if err != nil {
		return map[string]interface{}{}
	}
	root, err := unwrapWxbotConfig(raw)
	if err != nil {
		return map[string]interface{}{}
	}
	ai, _ := root["ai"].(map[string]interface{})
	if ai == nil {
		return map[string]interface{}{}
	}
	if value := normalizeAIModelName(stringValue(ai["reply_model"]), ""); value != "" {
		ai["reply_model"] = value
	}
	return ai
}

func (s *Server) latestAIVectorStatus(ctx context.Context) map[string]interface{} {
	if !tableExists(ctx, s.db, "wxbot_agents") {
		return nil
	}
	var raw json.RawMessage
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(ai_status_json, JSON_OBJECT())
		FROM wxbot_agents
		ORDER BY last_seen_at DESC, bot_id
		LIMIT 1
	`).Scan(&raw)
	if err != nil {
		return nil
	}
	var status map[string]interface{}
	if json.Unmarshal(raw, &status) != nil {
		return nil
	}
	vector, _ := status["vector"].(map[string]interface{})
	return vector
}

func (s *Server) aiRooms(ctx context.Context) ([]map[string]interface{}, error) {
	whitelist := aiGroupWhitelist(s.latestAIConfig(ctx))
	if len(whitelist) == 0 {
		return []map[string]interface{}{}, nil
	}
	allowed := botGroupWhitelistSet(whitelist)
	roomIDs := map[string]struct{}{}
	roomNames := map[string]string{}
	for _, roomID := range whitelist {
		roomIDs[roomID] = struct{}{}
	}
	addRooms := func(query string) error {
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var roomID string
			if err := rows.Scan(&roomID); err != nil {
				return err
			}
			roomID = strings.TrimSpace(roomID)
			if botGroupAllowed(roomID, allowed) {
				roomIDs[roomID] = struct{}{}
			}
		}
		return rows.Err()
	}
	if tableExists(ctx, s.db, "group_info") {
		rows, err := s.db.QueryContext(ctx, "SELECT room_id, room_name FROM group_info ORDER BY updated_at DESC LIMIT 500")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var roomID, roomName string
			if err := rows.Scan(&roomID, &roomName); err != nil {
				return nil, err
			}
			roomID = strings.TrimSpace(roomID)
			if roomID == "" {
				continue
			}
			if !botGroupAllowed(roomID, allowed) {
				continue
			}
			roomIDs[roomID] = struct{}{}
			if strings.TrimSpace(roomName) != "" {
				roomNames[roomID] = roomName
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for _, item := range []struct {
		table string
		query string
	}{
		{"ai_jobs", "SELECT DISTINCT room_id FROM ai_jobs ORDER BY room_id LIMIT 200"},
		{"ai_history_learning_tasks", "SELECT DISTINCT room_id FROM ai_history_learning_tasks ORDER BY room_id LIMIT 200"},
	} {
		if tableExists(ctx, s.db, item.table) {
			if err := addRooms(item.query); err != nil {
				return nil, err
			}
		}
	}
	ids := make([]string, 0, len(roomIDs))
	for roomID := range roomIDs {
		ids = append(ids, roomID)
	}
	sort.Slice(ids, func(i, j int) bool {
		return firstNonEmpty(roomNames[ids[i]], ids[i]) < firstNonEmpty(roomNames[ids[j]], ids[j])
	})
	result := make([]map[string]interface{}, 0, len(roomIDs))
	for _, roomID := range ids {
		result = append(result, map[string]interface{}{
			"roomId":     roomID,
			"roomName":   firstNonEmpty(roomNames[roomID], roomID),
			"activeJobs": []interface{}{},
		})
	}
	return result, nil
}

func aiGroupWhitelist(ai map[string]interface{}) []string {
	values, ok := ai["group_whitelist"].([]interface{})
	if !ok {
		if strings, ok := ai["group_whitelist"].([]string); ok {
			values = make([]interface{}, len(strings))
			for index, value := range strings {
				values[index] = value
			}
		}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		roomID := strings.TrimSpace(toString(value))
		if !strings.HasSuffix(roomID, "@chatroom") {
			continue
		}
		if _, exists := seen[roomID]; !exists {
			seen[roomID] = struct{}{}
			result = append(result, roomID)
		}
	}
	return result
}

func (s *Server) wxbotOnline(ctx context.Context) bool {
	if !tableExists(ctx, s.db, "wxbot_agents") {
		return false
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wxbot_agents WHERE last_seen_at >= DATE_SUB(NOW(), INTERVAL 90 SECOND)").Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func tableExists(ctx context.Context, db *sql.DB, table string) bool {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, table).Scan(&count)
	return err == nil && count > 0
}

func (s *Server) modelForJob(ctx context.Context, jobType string) string {
	cfg := s.latestAIConfig(ctx)
	if jobType == "vector_backfill" {
		return firstNonEmpty(stringValue(cfg["vector_embedding_model"]), "qwen3.7-text-embedding")
	}
	return normalizeAIModelName(firstNonEmpty(stringValue(cfg["reply_model"]), s.cfg.AIModel), "")
}

func aiJobRequiresWindow(jobType string) bool {
	return jobType == "vector_backfill"
}

func (s *Server) countTextMessages(ctx context.Context, roomID string, start, end float64) (int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM group_messages
		WHERE room_id = ? AND msg_type = 1 AND content IS NOT NULL AND content <> ''
		  AND created_at > ? AND created_at <= ?
	`, roomID, start, end).Scan(&total)
	return total, err
}

func (s *Server) aiMemoryRunsByIDs(ctx context.Context, ids []int64) ([]map[string]interface{}, error) {
	if len(ids) == 0 {
		return []map[string]interface{}{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, 0, len(ids)*2)
	for _, id := range ids {
		args = append(args, id)
	}
	for _, id := range ids {
		args = append(args, id)
	}
	return s.listAIMaps(ctx, "SELECT * FROM ai_memory_runs WHERE id IN ("+placeholders+") ORDER BY FIELD(id, "+placeholders+")", args...)
}

func (s *Server) jobForError(ctx context.Context, errorID int64) (map[string]interface{}, error) {
	return s.oneAIMap(ctx, `
		SELECT j.*
		FROM ai_job_errors e
		JOIN ai_jobs j ON j.id = e.job_id
		WHERE e.id = ?
	`, errorID)
}

func (s *Server) promoteCandidatePersona(ctx context.Context, candidate map[string]interface{}) error {
	roomID := stringValue(candidate["roomId"])
	candidateJSON, _ := candidate["candidateJson"].(map[string]interface{})
	persona, _ := candidateJSON["bot_persona"].(map[string]interface{})
	if persona == nil {
		persona = candidateJSON
	}
	if err := s.ensureAIPersonaVersionTable(ctx); err != nil {
		return err
	}
	current, err := s.oneAIMap(ctx, "SELECT * FROM ai_room_persona WHERE room_id = ?", roomID)
	if errors.Is(err, sql.ErrNoRows) {
		current = map[string]interface{}{"roomCultureJson": map[string]interface{}{}}
	} else if err != nil {
		return err
	}
	cultureJSON, _ := json.Marshal(current["roomCultureJson"])
	personaJSON, _ := json.Marshal(persona)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ai_room_persona (room_id, room_culture_json, bot_persona_json, updated_at)
		VALUES (?, CAST(? AS JSON), CAST(? AS JSON), ?)
		ON DUPLICATE KEY UPDATE bot_persona_json = VALUES(bot_persona_json), updated_at = VALUES(updated_at)
	`, roomID, string(cultureJSON), string(personaJSON), float64(time.Now().UnixNano())/1e9)
	if err != nil {
		return err
	}
	candidateID := int64Value(candidate["id"])
	return s.insertPersonaVersion(ctx, roomID, current["roomCultureJson"], persona, "candidate_promote", &candidateID, "promoted persona candidate")
}

func (s *Server) ensureAIStyleTables(ctx context.Context) error {
	if tableExists(ctx, s.db, "ai_role_cards") && tableExists(ctx, s.db, "ai_prompt_instructions") && tableExists(ctx, s.db, "ai_reply_style_samples") && tableExists(ctx, s.db, "ai_reply_conversation_samples") && tableExists(ctx, s.db, "ai_reply_log_feedbacks") {
		return s.ensureAIPromptInstructionDefaults(ctx)
	}
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS ai_role_cards (
			id TINYINT NOT NULL,
			content TEXT NOT NULL,
			updated_at DOUBLE NOT NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS ai_prompt_instructions (
			instruction_key VARCHAR(64) NOT NULL,
			content TEXT NOT NULL,
			updated_at DOUBLE NOT NULL,
			PRIMARY KEY (instruction_key)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS ai_reply_style_samples (
			id BIGINT NOT NULL AUTO_INCREMENT,
			room_id VARCHAR(128) NOT NULL DEFAULT '',
			scenario VARCHAR(32) NOT NULL DEFAULT '',
			trigger_text TEXT NOT NULL,
			reply_text TEXT NOT NULL,
			source_reply_log_id BIGINT NULL,
			created_at DOUBLE NOT NULL,
			updated_at DOUBLE NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_ai_reply_style_samples_source (source_reply_log_id),
			KEY idx_ai_reply_style_samples_room_time (room_id, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS ai_reply_conversation_samples (
			id BIGINT NOT NULL AUTO_INCREMENT,
			room_id VARCHAR(128) NOT NULL DEFAULT '',
			scenario VARCHAR(32) NOT NULL DEFAULT '',
			context_text TEXT NOT NULL,
			reply_text TEXT NOT NULL,
			source_reply_log_id BIGINT NULL,
			created_at DOUBLE NOT NULL,
			updated_at DOUBLE NOT NULL,
			PRIMARY KEY (id),
			KEY idx_ai_reply_conversation_samples_room_time (room_id, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS ai_reply_log_feedbacks (
			reply_log_id BIGINT NOT NULL,
			feedback VARCHAR(16) NOT NULL,
			updated_at DOUBLE NOT NULL,
			PRIMARY KEY (reply_log_id),
			KEY idx_ai_reply_log_feedbacks_feedback_time (feedback, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return s.ensureAIPromptInstructionDefaults(ctx)
}

func (s *Server) ensureAIPromptInstructionDefaults(ctx context.Context) error {
	now := float64(time.Now().UnixNano()) / 1e9
	for _, key := range aiPromptInstructionKeys {
		if _, err := s.db.ExecContext(ctx, `
			INSERT IGNORE INTO ai_prompt_instructions (instruction_key, content, updated_at)
			VALUES (?, ?, ?)
		`, key, defaultAIPromptInstructions[key], now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) ensureAIPersonaVersionTable(ctx context.Context) error {
	if tableExists(ctx, s.db, "ai_persona_versions") {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ai_persona_versions (
			id BIGINT NOT NULL AUTO_INCREMENT,
			room_id VARCHAR(128) NOT NULL,
			version_no INT NOT NULL,
			room_culture_json JSON NOT NULL,
			bot_persona_json JSON NOT NULL,
			source_type VARCHAR(32) NOT NULL DEFAULT '',
			source_id BIGINT NULL,
			note VARCHAR(255) NOT NULL DEFAULT '',
			created_at DOUBLE NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_ai_persona_versions_room_version (room_id, version_no),
			KEY idx_ai_persona_versions_room_time (room_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	return err
}

func (s *Server) insertPersonaVersion(ctx context.Context, roomID string, culture, persona interface{}, sourceType string, sourceID *int64, note string) error {
	if err := s.ensureAIPersonaVersionTable(ctx); err != nil {
		return err
	}
	var versionNo int
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version_no), 0) + 1 FROM ai_persona_versions WHERE room_id = ?", roomID).Scan(&versionNo); err != nil {
		return err
	}
	cultureJSON, _ := json.Marshal(culture)
	personaJSON, _ := json.Marshal(persona)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_persona_versions
			(room_id, version_no, room_culture_json, bot_persona_json, source_type, source_id, note, created_at)
		VALUES (?, ?, CAST(? AS JSON), CAST(? AS JSON), ?, ?, ?, ?)
	`, roomID, versionNo, string(cultureJSON), string(personaJSON), strings.TrimSpace(sourceType), nullableInt(sourceID), strings.TrimSpace(note), float64(time.Now().UnixNano())/1e9)
	return err
}

func parseAITime(raw string) (*float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return &parsed, nil
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			unix := float64(parsed.UnixNano()) / 1e9
			return &unix, nil
		}
	}
	return nil, errors.New("invalid time")
}

func floatPtr(value interface{}) *float64 {
	switch typed := value.(type) {
	case nil:
		return nil
	case float64:
		return &typed
	case int64:
		next := float64(typed)
		return &next
	case int:
		next := float64(typed)
		return &next
	case []byte:
		return floatPtr(string(typed))
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		next, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return nil
		}
		return &next
	default:
		return nil
	}
}

func nullableFloat(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func int64Value(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		next, _ := typed.Int64()
		return next
	case []byte:
		return int64Value(string(typed))
	case string:
		next, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return next
	default:
		return 0
	}
}

func int64Slice(value interface{}) []int64 {
	values, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]int64, 0, len(values))
	seen := map[int64]struct{}{}
	for _, item := range values {
		id := int64Value(item)
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func evidenceMessageIDs(runs []map[string]interface{}) []string {
	result := []string{}
	seen := map[string]struct{}{}
	var walk func(interface{})
	walk = func(value interface{}) {
		switch typed := value.(type) {
		case map[string]interface{}:
			for _, key := range []string{"evidence_msg_ids", "evidenceMsgIds"} {
				if raw, ok := typed[key].([]interface{}); ok {
					for _, item := range raw {
						id := strings.TrimSpace(stringValue(item))
						if id == "" {
							continue
						}
						if _, exists := seen[id]; exists {
							continue
						}
						seen[id] = struct{}{}
						result = append(result, id)
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	for _, run := range runs {
		walk(run["resultJson"])
	}
	return result
}

func boolValue(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return parseBool(typed)
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
