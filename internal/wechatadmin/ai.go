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
	"profile_json":        true,
	"request_meta_json":   true,
	"result_json":         true,
	"room_culture_json":   true,
	"config_json":         true,
	"current_config_json": true,
}

var manualAIJobTypes = map[string]bool{
	"segment_summary":   true,
	"profile_merge":     true,
	"culture_update":    true,
	"persona_candidate": true,
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
		"enabled":           boolValue(aiCfg["enabled"]),
		"configured":        strings.TrimSpace(stringValue(aiCfg["api_key"])) != "" || strings.TrimSpace(s.cfg.AIAPIKey) != "",
		"running":           s.wxbotOnline(r.Context()),
		"autoMemoryEnabled": boolValue(aiCfg["auto_memory_enabled"]),
		"queues":            queues,
		"models": map[string]string{
			"reply":      firstNonEmpty(stringValue(aiCfg["reply_model"]), s.cfg.AIModel),
			"summary":    firstNonEmpty(stringValue(aiCfg["summary_model"]), s.cfg.AIModel),
			"merge":      firstNonEmpty(stringValue(aiCfg["merge_model"]), s.cfg.AIModel),
			"manualDeep": firstNonEmpty(stringValue(aiCfg["manual_deep_model"]), s.cfg.AIModel),
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
	if jobType == "segment_summary" {
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
	ok(w, map[string]interface{}{"items": items})
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
			 processed_msg_count, segment_job_count, cursor_time, cursor_msg_id, created_at, updated_at)
		VALUES (?, 'queued', 'segment', ?, ?, ?, ?, 0, 0, ?, '', ?, ?)
	`, roomID, startValue, endValue, maxMessages, total, startValue, now, now)
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
	for _, key := range []string{"reply_model", "summary_model", "merge_model", "manual_deep_model"} {
		if value := normalizeAIModelName(stringValue(ai[key]), ""); value != "" {
			ai[key] = value
		}
	}
	return ai
}

func (s *Server) aiRooms(ctx context.Context) ([]map[string]interface{}, error) {
	whitelist, err := s.botGroupWhitelist(ctx)
	if err != nil {
		return nil, err
	}
	if len(whitelist) == 0 {
		return []map[string]interface{}{}, nil
	}
	allowed := botGroupWhitelistSet(whitelist)
	roomIDs := map[string]struct{}{}
	roomNames := map[string]string{}
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
		{"ai_memory_runs", "SELECT DISTINCT room_id FROM ai_memory_runs ORDER BY room_id LIMIT 200"},
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
	if jobType == "segment_summary" {
		return normalizeAIModelName(firstNonEmpty(stringValue(cfg["summary_model"]), s.cfg.AIModel), "")
	}
	return normalizeAIModelName(firstNonEmpty(stringValue(cfg["merge_model"]), s.cfg.AIModel), "")
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
