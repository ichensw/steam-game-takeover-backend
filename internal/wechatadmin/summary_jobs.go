package wechatadmin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	summaryJobPending   = "pending"
	summaryJobRunning   = "running"
	summaryJobSucceeded = "succeeded"
	summaryJobFailed    = "failed"
)

// ponytail: process-local lock; use a DB unique key if this service runs multiple instances.
var summaryJobCreateMu sync.Mutex

type summaryJobRequest struct {
	RoomID        string   `json:"roomId"`
	Date          string   `json:"date"`
	Period        string   `json:"period"`
	Start         string   `json:"start"`
	End           string   `json:"end"`
	Prompt        string   `json:"prompt"`
	Style         string   `json:"style"`
	Model         string   `json:"model"`
	CompareModels []string `json:"compareModels"`
	SendToGroup   bool     `json:"sendToGroup"`
}

type summaryOptions struct {
	Prompt        string   `json:"prompt,omitempty"`
	Style         string   `json:"style,omitempty"`
	Model         string   `json:"model,omitempty"`
	CompareModels []string `json:"compareModels,omitempty"`
	SendToGroup   bool     `json:"sendToGroup,omitempty"`
}

type summaryJobRecord struct {
	ID                  int64          `json:"id"`
	Status              string         `json:"status"`
	RoomID              string         `json:"roomId,omitempty"`
	RoomName            string         `json:"roomName,omitempty"`
	Start               string         `json:"start"`
	End                 string         `json:"end"`
	Period              string         `json:"period"`
	MessageCount        int            `json:"messageCount"`
	ChunkCount          int            `json:"chunkCount"`
	ProcessedChunkCount int            `json:"processedChunkCount"`
	SummaryID           int64          `json:"summaryId,omitempty"`
	Summary             *summaryRecord `json:"summary,omitempty"`
	Error               string         `json:"error,omitempty"`
	SendStatus          string         `json:"sendStatus,omitempty"`
	SendError           string         `json:"sendError,omitempty"`
	CreatedBy           string         `json:"createdBy,omitempty"`
	CreatedAt           string         `json:"createdAt"`
	StartedAt           string         `json:"startedAt,omitempty"`
	FinishedAt          string         `json:"finishedAt,omitempty"`
	Request             summaryRequest `json:"request"`
	Options             summaryOptions `json:"options"`
}

func (s *Server) createSummaryJob(w http.ResponseWriter, r *http.Request) {
	var req summaryJobRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	baseReq := summaryRequest{RoomID: req.RoomID, Date: req.Date, Period: req.Period, Start: req.Start, End: req.End}
	start, end, err := resolveSummaryRange(baseReq, s.cfg.Location, time.Now())
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
		return
	}
	summaryJobCreateMu.Lock()
	defer summaryJobCreateMu.Unlock()

	existingJob, hasJob, err := s.existingSummaryJob(r.Context(), req.RoomID, start, end)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "check summary job failed")
		return
	}
	if hasJob {
		ok(w, existingJob)
		return
	}
	if exists, err := s.summaryExists(r.Context(), req.RoomID, start, end); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "check summary history failed")
		return
	} else if exists {
		fail(w, http.StatusConflict, "SUMMARY_EXISTS", "该时间段已经生成过 AI 总结")
		return
	}
	job, err := s.insertSummaryJob(r.Context(), baseReq, s.summaryOptions(r, req), start, end, r.Header.Get(gatewayAdminUsernameHeader))
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "create summary job failed")
		return
	}
	go s.runSummaryJob(job.ID)
	ok(w, job)
}

func (s *Server) summaryJobDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid job id")
		return
	}
	job, err := s.summaryJobByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "NOT_FOUND", "summary job not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query summary job failed")
		return
	}
	ok(w, job)
}

func (s *Server) summaryOptions(r *http.Request, req summaryJobRequest) summaryOptions {
	options := summaryOptions{
		Prompt:        firstHeaderOrValue(r, summaryPromptHeader, req.Prompt),
		Style:         firstHeaderOrValue(r, summaryStyleHeader, req.Style),
		Model:         firstHeaderOrValue(r, summaryModelHeader, req.Model),
		CompareModels: cleanStringList(append(splitCSV(r.Header.Get(summaryCompareModelsHeader)), req.CompareModels...)),
		SendToGroup:   parseBool(firstHeaderOrValue(r, summaryAutoSendHeader, strconv.FormatBool(req.SendToGroup))),
	}
	if options.Model == "" {
		options.Model = s.cfg.AIModel
	}
	return options
}

func firstHeaderOrValue(r *http.Request, header, value string) string {
	if headerValue := strings.TrimSpace(r.Header.Get(header)); headerValue != "" {
		return headerValue
	}
	return strings.TrimSpace(value)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *Server) ensureSummaryJobSchema(ctx context.Context) error {
	if err := s.ensureSummarySchema(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, summaryJobSchemaSQL); err != nil {
		return err
	}
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'wechat_ai_summary_jobs'
		  AND column_name = 'processed_chunk_count'
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = s.db.ExecContext(ctx, `ALTER TABLE wechat_ai_summary_jobs ADD COLUMN processed_chunk_count INT UNSIGNED NOT NULL DEFAULT 0 AFTER chunk_count`)
	}
	return err
}

const summaryJobSchemaSQL = `
CREATE TABLE IF NOT EXISTS wechat_ai_summary_jobs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  status VARCHAR(32) NOT NULL,
  room_id VARCHAR(128) NULL,
  room_name VARCHAR(255) NULL,
  range_start DATETIME NOT NULL,
  range_end DATETIME NOT NULL,
  period VARCHAR(32) NOT NULL,
  request_json JSON NOT NULL,
  options_json JSON NOT NULL,
  message_count INT UNSIGNED NOT NULL DEFAULT 0,
  chunk_count INT UNSIGNED NOT NULL DEFAULT 0,
  processed_chunk_count INT UNSIGNED NOT NULL DEFAULT 0,
  summary_id BIGINT UNSIGNED NULL,
  error_message TEXT NULL,
  send_status VARCHAR(32) NULL,
  send_error TEXT NULL,
  created_by VARCHAR(128) NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at DATETIME NULL,
  finished_at DATETIME NULL,
  KEY idx_status_created (status, created_at),
  KEY idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='微信群 AI 总结任务';
`

func (s *Server) insertSummaryJob(ctx context.Context, req summaryRequest, options summaryOptions, start, end time.Time, createdBy string) (summaryJobRecord, error) {
	if err := s.ensureSummaryJobSchema(ctx); err != nil {
		return summaryJobRecord{}, err
	}
	roomID := strings.TrimSpace(req.RoomID)
	roomName := s.summaryRoomName(ctx, roomID)
	reqJSON, _ := json.Marshal(req)
	optionsJSON, _ := json.Marshal(options)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO wechat_ai_summary_jobs
			(status, room_id, room_name, range_start, range_end, period, request_json, options_json, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, summaryJobPending, nullEmpty(roomID), nullEmpty(roomName), start, end, summaryFirstNonEmpty(req.Period, "day"), string(reqJSON), string(optionsJSON), nullEmpty(createdBy))
	if err != nil {
		return summaryJobRecord{}, err
	}
	id, _ := result.LastInsertId()
	return s.summaryJobByID(ctx, id)
}

func (s *Server) existingSummaryJob(ctx context.Context, roomID string, start, end time.Time) (summaryJobRecord, bool, error) {
	if err := s.ensureSummaryJobSchema(ctx); err != nil {
		return summaryJobRecord{}, false, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM wechat_ai_summary_jobs
		WHERE room_id <=> ? AND range_start = ? AND range_end = ? AND status IN (?, ?, ?)
		ORDER BY id DESC
		LIMIT 1
	`, nullEmpty(strings.TrimSpace(roomID)), start, end, summaryJobPending, summaryJobRunning, summaryJobSucceeded)
	var id int64
	if err := row.Scan(&id); errors.Is(err, sql.ErrNoRows) {
		return summaryJobRecord{}, false, nil
	} else if err != nil {
		return summaryJobRecord{}, false, err
	}
	job, err := s.summaryJobByID(ctx, id)
	return job, err == nil, err
}

func (s *Server) summaryExists(ctx context.Context, roomID string, start, end time.Time) (bool, error) {
	if err := s.ensureSummarySchema(ctx); err != nil {
		return false, err
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM wechat_ai_summaries
		WHERE room_id <=> ? AND range_start = ? AND range_end = ?
		LIMIT 1
	`, nullEmpty(strings.TrimSpace(roomID)), start, end).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Server) runSummaryJob(jobID int64) {
	ctx := context.Background()
	job, err := s.summaryJobByID(ctx, jobID)
	if err != nil {
		return
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE wechat_ai_summary_jobs SET status = ?, started_at = ? WHERE id = ?", summaryJobRunning, time.Now(), jobID)
	record, messageCount, chunkCount, err := s.generateSegmentedSummary(ctx, jobID, job.Request, job.Options, job.CreatedBy)
	if err != nil {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE wechat_ai_summary_jobs
			SET status = ?, error_message = ?, message_count = ?, chunk_count = ?, finished_at = ?
			WHERE id = ?
		`, summaryJobFailed, err.Error(), messageCount, chunkCount, time.Now(), jobID)
		return
	}
	sendStatus, sendError := "", ""
	if job.Options.SendToGroup {
		if err := s.sendSummaryToGroup(ctx, record); err != nil {
			sendStatus, sendError = "failed", err.Error()
		} else {
			sendStatus = "sent"
		}
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE wechat_ai_summary_jobs
		SET status = ?, message_count = ?, chunk_count = ?, summary_id = ?, send_status = ?, send_error = ?, finished_at = ?
		WHERE id = ?
	`, summaryJobSucceeded, messageCount, chunkCount, record.ID, nullEmpty(sendStatus), nullEmpty(sendError), time.Now(), jobID)
}

func (s *Server) generateSegmentedSummary(ctx context.Context, jobID int64, req summaryRequest, options summaryOptions, createdBy string) (summaryRecord, int, int, error) {
	start, end, err := resolveSummaryRange(req, s.cfg.Location, time.Now())
	if err != nil {
		return summaryRecord{}, 0, 0, err
	}
	messages, err := s.summaryMessagesAll(ctx, req.RoomID, start, end)
	if err != nil {
		return summaryRecord{}, 0, 0, err
	}
	maxMessages := s.configuredSummaryMaxMessages()
	chunks := chunkSummaryMessages(messages, maxMessages)
	s.updateSummaryJobProgress(ctx, jobID, len(messages), len(chunks), 0)
	if len(messages) == 0 {
		report := emptySummaryReport()
		record, err := s.saveSummary(ctx, req, report, report.Overview, messages, false, start, end, createdBy, maxMessages, options.Model)
		return record, 0, 0, err
	}
	if s.cfg.AIAPIKey == "" {
		return summaryRecord{}, len(messages), len(chunks), errors.New("AI service is not configured")
	}
	reports := make([]summaryReport, 0, len(chunks))
	for index, chunk := range chunks {
		text, err := s.callAIForMessages(ctx, start, end, chunk, options)
		if err != nil {
			return summaryRecord{}, len(messages), len(chunks), err
		}
		reports = append(reports, parseSummaryReport(text, chunk, s.cfg.Location))
		s.updateSummaryJobProgress(ctx, jobID, len(messages), len(chunks), index+1)
	}
	report, rawText, err := s.mergeSummaryReports(ctx, start, end, reports, options)
	if err != nil {
		return summaryRecord{}, len(messages), len(chunks), err
	}
	for _, model := range options.CompareModels {
		model = strings.TrimSpace(model)
		if model == "" || model == options.Model {
			continue
		}
		compareOptions := options
		compareOptions.Model = model
		compareOptions.CompareModels = nil
		compareReport, _, err := s.mergeSummaryReports(ctx, start, end, reports, compareOptions)
		if err == nil {
			report.ModelComparisons = append(report.ModelComparisons, summaryModelComparison{Model: model, Overview: compareReport.Overview, Topics: compareReport.Topics})
		}
	}
	record, err := s.saveSummary(ctx, req, report, rawText, messages, false, start, end, createdBy, maxMessages, options.Model)
	return record, len(messages), len(chunks), err
}

func (s *Server) updateSummaryJobProgress(ctx context.Context, jobID int64, messageCount, chunkCount, processedChunkCount int) {
	if jobID <= 0 {
		return
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE wechat_ai_summary_jobs
		SET message_count = ?, chunk_count = ?, processed_chunk_count = ?
		WHERE id = ?
	`, messageCount, chunkCount, processedChunkCount, jobID)
}

func (s *Server) summaryMessagesAll(ctx context.Context, roomID string, start, end time.Time) ([]summaryMessage, error) {
	args := []interface{}{float64(start.Unix()), float64(end.Unix())}
	where := "msg_type = 1 AND content IS NOT NULL AND content <> '' AND created_at >= ? AND created_at < ?"
	if strings.TrimSpace(roomID) != "" {
		where += " AND room_id = ?"
		args = append(args, strings.TrimSpace(roomID))
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT msg_id, room_id, sender_name, content, created_at
		FROM group_messages
		WHERE `+where+`
		ORDER BY created_at ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []summaryMessage
	for rows.Next() {
		var msg summaryMessage
		var content sql.NullString
		var createdAt float64
		if err := rows.Scan(&msg.MsgID, &msg.RoomID, &msg.Sender, &content, &createdAt); err != nil {
			return nil, err
		}
		if content.Valid {
			msg.LocalID = len(messages) + 1
			msg.Content = content.String
			msg.CreatedAt = time.Unix(int64(createdAt), 0).In(s.cfg.Location)
			messages = append(messages, msg)
		}
	}
	return messages, rows.Err()
}

func chunkSummaryMessages(messages []summaryMessage, size int) [][]summaryMessage {
	if size <= 0 {
		size = 1000
	}
	var chunks [][]summaryMessage
	for start := 0; start < len(messages); start += size {
		end := start + size
		if end > len(messages) {
			end = len(messages)
		}
		chunks = append(chunks, messages[start:end])
	}
	return chunks
}

func (s *Server) callAIForMessages(ctx context.Context, start, end time.Time, messages []summaryMessage, options summaryOptions) (string, error) {
	return s.callAIContent(ctx, options.Model, summaryPrompt(options), fmt.Sprintf("时间段：%s 到 %s\n\n聊天记录：\n%s", start.In(s.cfg.Location).Format(time.RFC3339), end.In(s.cfg.Location).Format(time.RFC3339), formatSummaryMessages(messages, s.cfg.Location)))
}

func (s *Server) mergeSummaryReports(ctx context.Context, start, end time.Time, reports []summaryReport, options summaryOptions) (summaryReport, string, error) {
	if len(reports) == 1 {
		return reports[0], reports[0].Overview, nil
	}
	raw, err := s.callAIContent(ctx, options.Model, summaryPrompt(options), fmt.Sprintf("时间段：%s 到 %s\n\n以下是按时间分段生成的群聊摘要，请合并为一个完整总结，去重、按话题聚类，仍然输出指定 JSON：\n%s", start.In(s.cfg.Location).Format(time.RFC3339), end.In(s.cfg.Location).Format(time.RFC3339), formatSegmentReports(reports)))
	if err != nil {
		return summaryReport{}, "", err
	}
	report := parseSummaryReport(raw, nil, s.cfg.Location)
	attachMergedTopicMessages(&report, reports)
	return report, raw, nil
}

func summaryPrompt(options summaryOptions) string {
	var b strings.Builder
	b.WriteString(summarySystemPrompt)
	switch strings.ToLower(strings.TrimSpace(options.Style)) {
	case "brief":
		b.WriteString("\n\n风格：简洁版，主要话题控制在 3 个以内，每个话题一句话。")
	case "detailed":
		b.WriteString("\n\n风格：详细版，保留更多上下文、昵称、游戏名和关键细节。")
	case "fun":
		b.WriteString("\n\n风格：轻松版，可以保留群内调侃语气，但不得编造。")
	}
	if prompt := strings.TrimSpace(options.Prompt); prompt != "" {
		b.WriteString("\n\n额外要求：")
		b.WriteString(prompt)
	}
	return b.String()
}

func formatSegmentReports(reports []summaryReport) string {
	var b strings.Builder
	for index, report := range reports {
		fmt.Fprintf(&b, "【分段 %d】%s\n", index+1, report.Overview)
		for _, topic := range report.Topics {
			fmt.Fprintf(&b, "- %s：%s（原文 %d 条）\n", topic.Title, topic.Summary, len(topic.MessageIDs))
		}
		for _, item := range report.ImportantInfo {
			fmt.Fprintf(&b, "重要：%s\n", item)
		}
	}
	return b.String()
}

func attachMergedTopicMessages(report *summaryReport, segments []summaryReport) {
	if report == nil {
		return
	}
	var sources []summaryTopic
	for _, segment := range segments {
		sources = append(sources, segment.Topics...)
	}
	for index := range report.Topics {
		topic := &report.Topics[index]
		if len(topic.MessageIDs) > 0 {
			continue
		}
		matches := matchingSourceTopics(*topic, sources)
		if len(matches) == 0 && len(sources) == 1 {
			matches = sources
		}
		mergeTopicMessages(topic, matches)
	}
}

func matchingSourceTopics(target summaryTopic, sources []summaryTopic) []summaryTopic {
	needle := strings.ToLower(strings.Join(append([]string{target.Title, target.Summary}, target.Keywords...), " "))
	var matches []summaryTopic
	bestScore := 0
	for _, source := range sources {
		score := topicMatchScore(needle, source)
		if score == 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			matches = matches[:0]
		}
		if score == bestScore {
			matches = append(matches, source)
		}
	}
	return matches
}

func topicMatchScore(needle string, source summaryTopic) int {
	score := 0
	if source.Title != "" && strings.Contains(needle, strings.ToLower(source.Title)) {
		score += 4
	}
	for _, keyword := range source.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" && strings.Contains(needle, keyword) {
			score += 2
		}
	}
	return score
}

func mergeTopicMessages(target *summaryTopic, sources []summaryTopic) {
	seen := map[string]struct{}{}
	speakers := 0
	for _, source := range sources {
		speakers += source.SpeakerCount
		for _, id := range source.MessageIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			target.MessageIDs = append(target.MessageIDs, id)
		}
		for _, sample := range source.Samples {
			if len(target.Samples) >= 3 {
				break
			}
			target.Samples = append(target.Samples, sample)
		}
	}
	target.MessageCount = len(target.MessageIDs)
	target.SpeakerCount = speakers
}

func (s *Server) callAIContent(ctx context.Context, model, systemPrompt, userContent string) (string, error) {
	body := map[string]interface{}{
		"model": summaryFirstNonEmpty(model, s.cfg.AIModel),
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"temperature": 0.2,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.AIBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.AIAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai status %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("empty ai response")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func (s *Server) summaryJobByID(ctx context.Context, id int64) (summaryJobRecord, error) {
	if err := s.ensureSummaryJobSchema(ctx); err != nil {
		return summaryJobRecord{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, status, room_id, room_name, range_start, range_end, period, request_json, options_json,
		       message_count, chunk_count, processed_chunk_count, COALESCE(summary_id, 0), error_message, send_status, send_error,
		       created_by, created_at, started_at, finished_at
		FROM wechat_ai_summary_jobs
		WHERE id = ?
	`, id)
	var job summaryJobRecord
	var roomID, roomName, errorMessage, sendStatus, sendError, createdBy sql.NullString
	var summaryID int64
	var start, end, createdAt time.Time
	var startedAt, finishedAt sql.NullTime
	var reqJSON, optionsJSON []byte
	if err := row.Scan(&job.ID, &job.Status, &roomID, &roomName, &start, &end, &job.Period, &reqJSON, &optionsJSON, &job.MessageCount, &job.ChunkCount, &job.ProcessedChunkCount, &summaryID, &errorMessage, &sendStatus, &sendError, &createdBy, &createdAt, &startedAt, &finishedAt); err != nil {
		return summaryJobRecord{}, err
	}
	job.RoomID = sqlNullStringValue(roomID)
	job.RoomName = sqlNullStringValue(roomName)
	job.Error = sqlNullStringValue(errorMessage)
	job.SendStatus = sqlNullStringValue(sendStatus)
	job.SendError = sqlNullStringValue(sendError)
	job.CreatedBy = sqlNullStringValue(createdBy)
	job.Start = start.In(s.cfg.Location).Format(time.RFC3339)
	job.End = end.In(s.cfg.Location).Format(time.RFC3339)
	job.CreatedAt = createdAt.In(s.cfg.Location).Format(time.RFC3339)
	if startedAt.Valid {
		job.StartedAt = startedAt.Time.In(s.cfg.Location).Format(time.RFC3339)
	}
	if finishedAt.Valid {
		job.FinishedAt = finishedAt.Time.In(s.cfg.Location).Format(time.RFC3339)
	}
	_ = json.Unmarshal(reqJSON, &job.Request)
	_ = json.Unmarshal(optionsJSON, &job.Options)
	if summaryID > 0 {
		job.SummaryID = summaryID
		if record, err := s.summaryByID(ctx, summaryID); err == nil {
			job.Summary = &record
		}
	}
	return job, nil
}

func (s *Server) sendSummaryToGroup(ctx context.Context, record summaryRecord) error {
	if strings.TrimSpace(record.RoomID) == "" {
		return errors.New("roomId is required")
	}
	if s.cfg.WechatHookAPIURL == "" || s.cfg.WechatHookAPIToken == "" {
		return errors.New("wechat hook api is not configured")
	}
	body, _ := json.Marshal(map[string]string{
		"room_id": record.RoomID,
		"content": formatSummaryForWechat(record),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.WechatHookAPIURL+"/api/send_group", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.WechatHookAPIToken)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wechat hook status %d", resp.StatusCode)
	}
	return nil
}

func formatSummaryForWechat(record summaryRecord) string {
	report := record.Report
	var b strings.Builder
	fmt.Fprintf(&b, "【一句话概览】\n%s\n\n【主要话题】\n", report.Overview)
	for _, topic := range report.Topics {
		fmt.Fprintf(&b, "- %s：%s\n", topic.Title, topic.Summary)
	}
	if len(report.ImportantInfo) > 0 {
		b.WriteString("\n【值得注意的信息】\n")
		for _, item := range report.ImportantInfo {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if strings.TrimSpace(report.Disputes) != "" {
		fmt.Fprintf(&b, "\n【争议或情绪】\n%s", report.Disputes)
	}
	return b.String()
}
