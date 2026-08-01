package wechatadmin

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	GatewaySecretHeader        = "X-Wechat-Bot-Admin-Secret"
	GatewayAdminIDHeader       = "X-Wechat-Bot-Admin-ID"
	GatewayAdminUsernameHeader = "X-Wechat-Bot-Admin-Username"
	WxbotTokenHeader           = "X-Wxbot-Token"
	SummaryMaxMessagesHeader   = "X-Wechat-Bot-Summary-Max-Messages"
	SummaryPromptHeader        = "X-Wechat-Bot-Summary-Prompt"
	SummaryStyleHeader         = "X-Wechat-Bot-Summary-Style"
	SummaryModelHeader         = "X-Wechat-Bot-Summary-Model"
	SummaryCompareModelsHeader = "X-Wechat-Bot-Summary-Compare-Models"
	SummaryAutoSendHeader      = "X-Wechat-Bot-Summary-Auto-Send"

	gatewayAdminUsernameHeader = GatewayAdminUsernameHeader
	summaryMaxMessagesHeader   = SummaryMaxMessagesHeader
	summaryPromptHeader        = SummaryPromptHeader
	summaryStyleHeader         = SummaryStyleHeader
	summaryModelHeader         = SummaryModelHeader
	summaryCompareModelsHeader = SummaryCompareModelsHeader
	summaryAutoSendHeader      = SummaryAutoSendHeader
)

type Config struct {
	GatewaySharedSecret string
	AIAPIKey            string
	AIBaseURL           string
	AIModel             string
	AITimeout           time.Duration
	SummaryMaxMessages  int
	WechatHookAPIURL    string
	WechatHookAPIToken  string
	WxbotAPIToken       string
	Location            *time.Location
}

type Server struct {
	cfg    Config
	db     *sql.DB
	client *http.Client
}

func NewServer(cfg Config, db *sql.DB) http.Handler {
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.AIBaseURL == "" {
		cfg.AIBaseURL = "https://api.openai.com/v1"
	}
	if cfg.AIModel == "" {
		cfg.AIModel = "gpt-4o-mini"
	}
	if cfg.AITimeout <= 0 {
		cfg.AITimeout = 180 * time.Second
	}
	s := &Server{cfg: cfg, db: db, client: &http.Client{Timeout: cfg.AITimeout}}

	mux := http.NewServeMux()
	mux.Handle("GET /api/groups", s.trustedAdmin(http.HandlerFunc(s.groups)))
	mux.Handle("GET /api/messages", s.trustedAdmin(http.HandlerFunc(s.messages)))
	mux.Handle("POST /api/messages/summary", s.trustedAdmin(http.HandlerFunc(s.summary)))
	mux.Handle("POST /api/messages/summary-jobs", s.trustedAdmin(http.HandlerFunc(s.createSummaryJob)))
	mux.Handle("GET /api/messages/summary-jobs/{id}", s.trustedAdmin(http.HandlerFunc(s.summaryJobDetail)))
	mux.Handle("GET /api/messages/summary/history", s.trustedAdmin(http.HandlerFunc(s.summaryHistory)))
	mux.Handle("GET /api/messages/summary/{id}", s.trustedAdmin(http.HandlerFunc(s.summaryDetail)))
	mux.Handle("GET /api/messages/summary/{id}/messages", s.trustedAdmin(http.HandlerFunc(s.summaryOriginalMessages)))
	mux.Handle("GET /api/stats/daily", s.trustedAdmin(http.HandlerFunc(s.dailyStats)))
	mux.Handle("GET /api/tables", s.trustedAdmin(http.HandlerFunc(s.tables)))
	mux.Handle("GET /api/tables/{table}", s.trustedAdmin(http.HandlerFunc(s.tableDetail)))
	mux.Handle("GET /api/tables/{table}/rows", s.trustedAdmin(http.HandlerFunc(s.tableRows)))
	mux.Handle("GET /api/wxbots", s.trustedAdmin(http.HandlerFunc(s.wxbotList)))
	mux.Handle("GET /api/wxbots/{botID}/config", s.trustedAdmin(http.HandlerFunc(s.wxbotConfigDetail)))
	mux.Handle("PUT /api/wxbots/{botID}/config", s.trustedAdmin(http.HandlerFunc(s.wxbotUpdateConfig)))
	mux.Handle("GET /api/ai/status", s.trustedAdmin(http.HandlerFunc(s.aiStatus)))
	mux.Handle("GET /api/ai/jobs", s.trustedAdmin(http.HandlerFunc(s.aiJobs)))
	mux.Handle("POST /api/ai/jobs", s.trustedAdmin(http.HandlerFunc(s.createAIJob)))
	mux.Handle("GET /api/ai/jobs/{id}", s.trustedAdmin(http.HandlerFunc(s.aiJobDetail)))
	mux.Handle("GET /api/ai/history-learning", s.trustedAdmin(http.HandlerFunc(s.aiHistoryLearningTasks)))
	mux.Handle("POST /api/ai/history-learning", s.trustedAdmin(http.HandlerFunc(s.createAIHistoryLearningTask)))
	mux.Handle("POST /api/ai/history-learning/{id}/{action}", s.trustedAdmin(http.HandlerFunc(s.updateAIHistoryLearningTask)))
	mux.Handle("GET /api/ai/errors", s.trustedAdmin(http.HandlerFunc(s.aiErrors)))
	mux.Handle("POST /api/ai/errors/{id}/retry", s.trustedAdmin(http.HandlerFunc(s.retryAIError)))
	mux.Handle("POST /api/ai/errors/{id}/resolve", s.trustedAdmin(http.HandlerFunc(s.resolveAIError)))
	mux.Handle("GET /api/ai/observation", s.trustedAdmin(http.HandlerFunc(s.aiObservation)))
	mux.Handle("GET /api/ai/role-card", s.trustedAdmin(http.HandlerFunc(s.aiRoleCard)))
	mux.Handle("PUT /api/ai/role-card", s.trustedAdmin(http.HandlerFunc(s.updateAIRoleCard)))
	mux.Handle("GET /api/ai/prompt-instructions", s.trustedAdmin(http.HandlerFunc(s.aiPromptInstructions)))
	mux.Handle("PUT /api/ai/prompt-instructions", s.trustedAdmin(http.HandlerFunc(s.updateAIPromptInstruction)))
	mux.Handle("GET /api/ai/reply-samples", s.trustedAdmin(http.HandlerFunc(s.aiReplyStyleSamples)))
	mux.Handle("POST /api/ai/reply-samples", s.trustedAdmin(http.HandlerFunc(s.createAIReplyStyleSample)))
	mux.Handle("DELETE /api/ai/reply-samples/{id}", s.trustedAdmin(http.HandlerFunc(s.deleteAIReplyStyleSample)))
	mux.Handle("GET /api/ai/reply-conversation-samples", s.trustedAdmin(http.HandlerFunc(s.aiReplyConversationSamples)))
	mux.Handle("POST /api/ai/reply-conversation-samples", s.trustedAdmin(http.HandlerFunc(s.createAIReplyConversationSample)))
	mux.Handle("DELETE /api/ai/reply-conversation-samples/{id}", s.trustedAdmin(http.HandlerFunc(s.deleteAIReplyConversationSample)))
	mux.Handle("GET /api/ai/reply-logs", s.trustedAdmin(http.HandlerFunc(s.aiReplyLogs)))
	mux.Handle("POST /api/ai/reply-logs/{id}/feedback", s.trustedAdmin(http.HandlerFunc(s.reviewAIReplyLog)))
	mux.Handle("GET /api/ai/memory/runs", s.trustedAdmin(http.HandlerFunc(s.aiMemoryRuns)))
	mux.Handle("GET /api/ai/memory/facts", s.trustedAdmin(http.HandlerFunc(s.aiGroupFacts)))
	mux.Handle("GET /api/ai/memory/relationships", s.trustedAdmin(http.HandlerFunc(s.aiRelationships)))
	mux.Handle("GET /api/ai/memory/events", s.trustedAdmin(http.HandlerFunc(s.aiGroupEvents)))
	mux.Handle("GET /api/ai/interventions", s.trustedAdmin(http.HandlerFunc(s.aiInterventions)))
	mux.Handle("GET /api/ai/memory/feedbacks", s.trustedAdmin(http.HandlerFunc(s.aiMemoryFeedbacks)))
	mux.Handle("GET /api/ai/config", s.trustedAdmin(http.HandlerFunc(s.aiProactiveConfig)))
	mux.Handle("PUT /api/ai/config", s.trustedAdmin(http.HandlerFunc(s.updateAIProactiveConfig)))
	mux.Handle("GET /api/ai/memory/room-persona", s.trustedAdmin(http.HandlerFunc(s.aiRoomPersona)))
	mux.Handle("GET /api/ai/memory/member-profiles", s.trustedAdmin(http.HandlerFunc(s.aiMemberProfiles)))
	mux.Handle("GET /api/ai/memory/persona-candidates", s.trustedAdmin(http.HandlerFunc(s.aiPersonaCandidates)))
	mux.Handle("GET /api/ai/memory/persona-candidates/{id}/evidence", s.trustedAdmin(http.HandlerFunc(s.aiPersonaCandidateEvidence)))
	mux.Handle("POST /api/ai/memory/persona-candidates/{id}/promote", s.trustedAdmin(http.HandlerFunc(s.promoteAIPersonaCandidate)))
	mux.Handle("POST /api/ai/memory/persona-candidates/{id}/reject", s.trustedAdmin(http.HandlerFunc(s.rejectAIPersonaCandidate)))
	mux.Handle("GET /api/ai/memory/persona-versions", s.trustedAdmin(http.HandlerFunc(s.aiPersonaVersions)))
	mux.Handle("POST /api/ai/memory/persona-versions/{id}/rollback", s.trustedAdmin(http.HandlerFunc(s.rollbackAIPersonaVersion)))
	mux.Handle("POST /api/wxbot/heartbeat", s.wxbotAuth(http.HandlerFunc(s.wxbotHeartbeat)))
	mux.Handle("GET /api/wxbot/config", s.wxbotAuth(http.HandlerFunc(s.wxbotConfigForBot)))
	mux.Handle("POST /api/wxbot/config/applied", s.wxbotAuth(http.HandlerFunc(s.wxbotConfigApplied)))
	mux.Handle("GET /api/wxbot/ai/history-learning/next", s.wxbotAuth(http.HandlerFunc(s.wxbotNextHistoryLearningTask)))
	mux.Handle("POST /api/wxbot/ai/history-learning/{id}/progress", s.wxbotAuth(http.HandlerFunc(s.wxbotHistoryLearningProgress)))
	return mux
}

func (s *Server) trustedAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supplied := r.Header.Get(GatewaySecretHeader)
		expected := s.cfg.GatewaySharedSecret
		if supplied == "" || expected == "" || subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) != 1 {
			fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}
		if r.Header.Get(GatewayAdminIDHeader) == "" || r.Header.Get(GatewayAdminUsernameHeader) == "" {
			fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "administrator identity is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) wxbotAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := s.cfg.WxbotAPIToken
		supplied := r.Header.Get(WxbotTokenHeader)
		if supplied == "" {
			const prefix = "Bearer "
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, prefix) {
				supplied = strings.TrimSpace(strings.TrimPrefix(auth, prefix))
			}
		}
		if supplied == "" || expected == "" || subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) != 1 {
			fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) groups(w http.ResponseWriter, r *http.Request) {
	whitelist, err := s.botGroupWhitelist(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query group whitelist failed")
		return
	}
	if len(whitelist) == 0 {
		ok(w, []map[string]interface{}{})
		return
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(whitelist)), ",")
	args := make([]interface{}, 0, len(whitelist))
	for _, roomID := range whitelist {
		args = append(args, roomID)
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT room_id, room_name, member_count, owner_wxid, updated_at
		FROM group_info
		WHERE room_id IN (`+placeholders+`)
		ORDER BY updated_at DESC
	`, args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query groups failed")
		return
	}
	defer rows.Close()

	groups := make([]map[string]interface{}, 0)
	for rows.Next() {
		var roomID, roomName, ownerWxid string
		var memberCount int
		var updatedAt float64
		if err := rows.Scan(&roomID, &roomName, &memberCount, &ownerWxid, &updatedAt); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan groups failed")
			return
		}
		groups = append(groups, map[string]interface{}{
			"roomId":      roomID,
			"roomName":    roomName,
			"memberCount": memberCount,
			"ownerWxid":   ownerWxid,
			"updatedAt":   unixJSON(updatedAt, s.cfg.Location),
		})
	}
	ok(w, groups)
}

func (s *Server) botGroupWhitelist(ctx context.Context) ([]string, error) {
	if err := s.ensureWxbotSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(config_json, JSON_OBJECT()), COALESCE(current_config_json, JSON_OBJECT())
		FROM wxbot_agents
		ORDER BY config_updated_at DESC, last_seen_at DESC, bot_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	whitelist := make([]string, 0)
	for rows.Next() {
		var configRaw, currentConfigRaw []byte
		if err := rows.Scan(&configRaw, &currentConfigRaw); err != nil {
			return nil, err
		}
		values, ok := parseBotGroupWhitelist(configRaw)
		if !ok {
			values, _ = parseBotGroupWhitelist(currentConfigRaw)
		}
		for _, value := range values {
			roomID := strings.TrimSpace(value)
			if roomID == "" {
				continue
			}
			if _, exists := seen[roomID]; exists {
				continue
			}
			seen[roomID] = struct{}{}
			whitelist = append(whitelist, roomID)
		}
	}
	return whitelist, rows.Err()
}

func botGroupWhitelistSet(whitelist []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(whitelist))
	for _, roomID := range whitelist {
		roomID = strings.TrimSpace(roomID)
		if roomID != "" {
			allowed[roomID] = struct{}{}
		}
	}
	return allowed
}

func botGroupAllowed(roomID string, allowed map[string]struct{}) bool {
	_, ok := allowed[strings.TrimSpace(roomID)]
	return ok
}

func parseBotGroupWhitelist(raw []byte) ([]string, bool) {
	root, err := unwrapWxbotConfig(raw)
	if err != nil {
		return nil, false
	}
	bot, ok := root["bot"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	items, ok := bot["group_whitelist"].([]interface{})
	if !ok {
		if values, ok := bot["group_whitelist"].([]string); ok {
			return values, true
		}
		return nil, false
	}
	whitelist := make([]string, 0, len(items))
	for _, item := range items {
		whitelist = append(whitelist, strings.TrimSpace(toString(item)))
	}
	return whitelist, true
}

func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	query, args, err := s.messageWhere(r)
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
		return
	}
	page := positiveInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := positiveInt(r.URL.Query().Get("pageSize"), 50, 1, 200)
	offset := (page - 1) * pageSize

	countSQL := "SELECT COUNT(*) FROM group_messages WHERE " + query
	var total int
	if err := s.db.QueryRowContext(r.Context(), countSQL, args...).Scan(&total); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count messages failed")
		return
	}

	listSQL := `
		SELECT msg_id, room_id, sender_wxid, sender_name, msg_type, content, xml_content,
		       media_url, media_local_path, media_oss_key, created_at
		FROM group_messages
		WHERE ` + query + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)
	rows, err := s.db.QueryContext(r.Context(), listSQL, args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query messages failed")
		return
	}
	defer rows.Close()

	var messages []map[string]interface{}
	for rows.Next() {
		var msgID, roomID, senderWxid, senderName, mediaURL, mediaLocalPath, mediaOSSKey string
		var msgType int
		var content, xmlContent sql.NullString
		var createdAt float64
		if err := rows.Scan(&msgID, &roomID, &senderWxid, &senderName, &msgType, &content, &xmlContent, &mediaURL, &mediaLocalPath, &mediaOSSKey, &createdAt); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan messages failed")
			return
		}
		messages = append(messages, map[string]interface{}{
			"msgId":          msgID,
			"roomId":         roomID,
			"senderWxid":     senderWxid,
			"senderName":     senderName,
			"msgType":        msgType,
			"subType":        messageSubType(msgType, content.String, xmlContent.String),
			"content":        nullString(content),
			"xmlContent":     nullString(xmlContent),
			"mediaUrl":       mediaURL,
			"mediaLocalPath": mediaLocalPath,
			"mediaOssKey":    mediaOSSKey,
			"createdAt":      unixJSON(createdAt, s.cfg.Location),
		})
	}
	ok(w, map[string]interface{}{
		"data": messages,
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + pageSize - 1) / pageSize,
		},
	})
}

func (s *Server) messageWhere(r *http.Request) (string, []interface{}, error) {
	q := r.URL.Query()
	conditions := []string{"1=1"}
	var args []interface{}

	if roomID := strings.TrimSpace(q.Get("roomId")); roomID != "" {
		conditions = append(conditions, "room_id = ?")
		args = append(args, roomID)
	}
	if sender := strings.TrimSpace(q.Get("sender")); sender != "" {
		conditions = append(conditions, "(sender_wxid = ? OR sender_name LIKE ? ESCAPE '\\\\')")
		args = append(args, sender, likePattern(sender))
	}
	if msgType := strings.TrimSpace(q.Get("msgType")); msgType != "" {
		value, err := strconv.Atoi(msgType)
		if err != nil {
			return "", nil, errors.New("msgType must be a number")
		}
		conditions = append(conditions, "msg_type = ?")
		args = append(args, value)
	}
	if subType := strings.TrimSpace(q.Get("subType")); subType != "" {
		condition, subTypeArgs, err := messageSubTypeCondition(subType)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, condition)
		args = append(args, subTypeArgs...)
	}
	if keyword := strings.TrimSpace(q.Get("keyword")); keyword != "" {
		conditions = append(conditions, "content LIKE ? ESCAPE '\\\\'")
		args = append(args, likePattern(keyword))
	}
	if start := strings.TrimSpace(q.Get("start")); start != "" {
		ts, err := parseTimeParam(start, s.cfg.Location)
		if err != nil {
			return "", nil, errors.New("start must be RFC3339, YYYY-MM-DD HH:mm:ss, or unix seconds")
		}
		conditions = append(conditions, "created_at >= ?")
		args = append(args, float64(ts.Unix()))
	}
	if end := strings.TrimSpace(q.Get("end")); end != "" {
		ts, err := parseTimeParam(end, s.cfg.Location)
		if err != nil {
			return "", nil, errors.New("end must be RFC3339, YYYY-MM-DD HH:mm:ss, or unix seconds")
		}
		conditions = append(conditions, "created_at < ?")
		args = append(args, float64(ts.Unix()))
	}

	return strings.Join(conditions, " AND "), args, nil
}

func messageSubType(msgType int, content, xmlContent string) string {
	payload := content + "\n" + xmlContent
	switch msgType {
	case 10000:
		if strings.Contains(payload, "加入") && strings.Contains(payload, "群聊") {
			return "group_join"
		}
		if strings.Contains(payload, "退出") || strings.Contains(payload, "移出") {
			return "group_leave"
		}
		return "group_system"
	case 10002:
		if strings.Contains(payload, `type="pat"`) {
			return "pat"
		}
		if strings.Contains(payload, `type="revokemsg"`) {
			return "revoke"
		}
		if strings.Contains(payload, "mmchatroombarannouncememt") {
			return "room_announcement"
		}
		return "system_notice"
	case 49:
		if strings.Contains(payload, "<type>33</type>") {
			return "mini_program"
		}
		if strings.Contains(payload, "<type>6</type>") {
			return "file"
		}
		if strings.Contains(payload, "<type>5</type>") {
			return "link"
		}
		if strings.Contains(payload, "<type>2000</type>") {
			return "transfer"
		}
		if strings.Contains(payload, "<type>2001</type>") {
			return "red_packet"
		}
		return "card_file"
	}
	return ""
}

func messageSubTypeCondition(subType string) (string, []interface{}, error) {
	likePayload := func(msgType int, marker string) (string, []interface{}) {
		return "msg_type = ? AND (content LIKE ? ESCAPE '\\\\' OR xml_content LIKE ? ESCAPE '\\\\')", []interface{}{msgType, "%" + marker + "%", "%" + marker + "%"}
	}
	switch subType {
	case "group_join":
		return "msg_type = ? AND content LIKE ? ESCAPE '\\\\'", []interface{}{10000, "%加入%群聊%"}, nil
	case "group_leave":
		return "msg_type = ? AND (content LIKE ? ESCAPE '\\\\' OR content LIKE ? ESCAPE '\\\\')", []interface{}{10000, "%退出%群聊%", "%移出%群聊%"}, nil
	case "group_system":
		return "msg_type = ? AND COALESCE(content, '') NOT LIKE ? ESCAPE '\\\\' AND COALESCE(content, '') NOT LIKE ? ESCAPE '\\\\' AND COALESCE(content, '') NOT LIKE ? ESCAPE '\\\\'", []interface{}{10000, "%加入%群聊%", "%退出%群聊%", "%移出%群聊%"}, nil
	case "pat":
		condition, args := likePayload(10002, `type="pat"`)
		return condition, args, nil
	case "revoke":
		condition, args := likePayload(10002, `type="revokemsg"`)
		return condition, args, nil
	case "room_announcement":
		condition, args := likePayload(10002, "mmchatroombarannouncememt")
		return condition, args, nil
	case "system_notice":
		return "msg_type = ? AND (COALESCE(content, '') NOT LIKE ? ESCAPE '\\\\' AND COALESCE(xml_content, '') NOT LIKE ? ESCAPE '\\\\') AND (COALESCE(content, '') NOT LIKE ? ESCAPE '\\\\' AND COALESCE(xml_content, '') NOT LIKE ? ESCAPE '\\\\') AND (COALESCE(content, '') NOT LIKE ? ESCAPE '\\\\' AND COALESCE(xml_content, '') NOT LIKE ? ESCAPE '\\\\')", []interface{}{10002, `%type="pat"%`, `%type="pat"%`, `%type="revokemsg"%`, `%type="revokemsg"%`, "%mmchatroombarannouncememt%", "%mmchatroombarannouncememt%"}, nil
	case "mini_program":
		condition, args := likePayload(49, "<type>33</type>")
		return condition, args, nil
	case "file":
		condition, args := likePayload(49, "<type>6</type>")
		return condition, args, nil
	case "link":
		condition, args := likePayload(49, "<type>5</type>")
		return condition, args, nil
	case "transfer":
		condition, args := likePayload(49, "<type>2000</type>")
		return condition, args, nil
	case "red_packet":
		condition, args := likePayload(49, "<type>2001</type>")
		return condition, args, nil
	case "card_file":
		markers := []string{"<type>33</type>", "<type>6</type>", "<type>5</type>", "<type>2000</type>", "<type>2001</type>"}
		conditions := []string{"msg_type = ?"}
		args := []interface{}{49}
		for _, marker := range markers {
			conditions = append(conditions, "(COALESCE(content, '') NOT LIKE ? ESCAPE '\\\\' AND COALESCE(xml_content, '') NOT LIKE ? ESCAPE '\\\\')")
			args = append(args, "%"+marker+"%", "%"+marker+"%")
		}
		return strings.Join(conditions, " AND "), args, nil
	default:
		return "", nil, errors.New("subType is not supported")
	}
}

func (s *Server) tables(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT TABLE_NAME, TABLE_COMMENT, ENGINE, COALESCE(TABLE_ROWS, 0)
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		ORDER BY TABLE_NAME
	`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query tables failed")
		return
	}
	defer rows.Close()

	var tables []map[string]interface{}
	for rows.Next() {
		var name, comment, engine string
		var approxRows int64
		if err := rows.Scan(&name, &comment, &engine, &approxRows); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan tables failed")
			return
		}
		tables = append(tables, map[string]interface{}{"name": name, "comment": comment, "engine": engine, "approxRows": approxRows})
	}
	ok(w, tables)
}

func (s *Server) tableDetail(w http.ResponseWriter, r *http.Request) {
	table, err := s.checkedTable(r.Context(), r.PathValue("table"))
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COALESCE(COLUMN_DEFAULT, ''), EXTRA, COLUMN_COMMENT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, table)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query columns failed")
		return
	}
	defer rows.Close()

	var columns []map[string]interface{}
	for rows.Next() {
		var name, typ, nullable, key, def, extra, comment string
		if err := rows.Scan(&name, &typ, &nullable, &key, &def, &extra, &comment); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan columns failed")
			return
		}
		columns = append(columns, map[string]interface{}{
			"name": name, "type": typ, "nullable": nullable == "YES", "key": key, "default": def, "extra": extra, "comment": comment,
		})
	}
	ok(w, map[string]interface{}{"table": table, "columns": columns})
}

func (s *Server) tableRows(w http.ResponseWriter, r *http.Request) {
	table, err := s.checkedTable(r.Context(), r.PathValue("table"))
	if err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", err.Error())
		return
	}
	page := positiveInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := positiveInt(r.URL.Query().Get("pageSize"), 50, 1, 200)
	offset := (page - 1) * pageSize

	var total int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM `"+table+"`").Scan(&total); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count table rows failed")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), "SELECT * FROM `"+table+"` LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query table rows failed")
		return
	}
	defer rows.Close()

	data, err := scanRows(rows)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan table rows failed")
		return
	}
	ok(w, map[string]interface{}{
		"data": data,
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + pageSize - 1) / pageSize,
		},
	})
}

func (s *Server) checkedTable(ctx context.Context, table string) (string, error) {
	if !validIdentifier(table) {
		return "", errors.New("invalid table name")
	}
	var found string
	err := s.db.QueryRowContext(ctx, `
		SELECT TABLE_NAME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, table).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("table not found")
	}
	return found, err
}

func parseTimeParam(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty time")
	}
	if unix, err := strconv.ParseFloat(value, 64); err == nil {
		sec := int64(unix)
		nsec := int64((unix - float64(sec)) * 1e9)
		return time.Unix(sec, nsec).In(loc), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid time")
}

func scanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
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
		for i, col := range columns {
			if b, ok := values[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = values[i]
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func positiveInt(raw string, fallback, min, max int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r != '_' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}

func likePattern(value string) string {
	var b strings.Builder
	b.WriteByte('%')
	for _, r := range value {
		if r == '\\' || r == '%' || r == '_' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('%')
	return b.String()
}

func nullString(value sql.NullString) interface{} {
	if value.Valid {
		return value.String
	}
	return nil
}

func unixJSON(seconds float64, loc *time.Location) map[string]interface{} {
	sec := int64(seconds)
	nsec := int64((seconds - float64(sec)) * 1e9)
	t := time.Unix(sec, nsec).In(loc)
	return map[string]interface{}{"unix": seconds, "text": t.Format("2006-01-02 15:04:05")}
}
