package wechatadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type groupWhitelistUpdateRequest struct {
	BotID   string `json:"botId"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type managedGroupRow struct {
	RoomID      string
	RoomName    string
	MemberCount int
	OwnerWxid   string
	UpdatedAt   float64
}

type managedGroupMessageStats struct {
	MessageCount  int
	ActiveMembers int
	LastMessageAt sql.NullFloat64
}

const managedGroupVisibleWhere = "room_id <> '' AND TRIM(COALESCE(room_name, '')) <> ''"

func (s *Server) groupManagementList(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureWechatGroupIndexes(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "ensure group indexes failed")
		return
	}
	botID := cleanBotID(r.URL.Query().Get("botId"))
	whitelists, effectiveBotID, err := s.groupWhitelists(r.Context(), botID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query group whitelist failed")
		return
	}
	page := positiveInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := positiveInt(r.URL.Query().Get("pageSize"), 20, 1, 200)
	offset := (page - 1) * pageSize

	var total int
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM group_info
		WHERE `+managedGroupVisibleWhere+`
	`).Scan(&total); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count managed groups failed")
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT room_id, room_name, COALESCE(member_count, 0), COALESCE(owner_wxid, ''), COALESCE(updated_at, 0)
		FROM group_info
		WHERE `+managedGroupVisibleWhere+`
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, pageSize, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query managed groups failed")
		return
	}
	defer rows.Close()

	groups := make([]managedGroupRow, 0, pageSize)
	roomIDs := make([]string, 0, pageSize)
	for rows.Next() {
		var group managedGroupRow
		if err := rows.Scan(&group.RoomID, &group.RoomName, &group.MemberCount, &group.OwnerWxid, &group.UpdatedAt); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan managed groups failed")
			return
		}
		groups = append(groups, group)
		roomIDs = append(roomIDs, group.RoomID)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query managed groups failed")
		return
	}

	stats, err := s.groupMessageStats(r.Context(), roomIDs)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query managed group stats failed")
		return
	}

	items := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		groupStats := stats[group.RoomID]
		item := map[string]interface{}{
			"roomId":         group.RoomID,
			"roomName":       group.RoomName,
			"memberCount":    group.MemberCount,
			"ownerWxid":      group.OwnerWxid,
			"updatedAt":      unixJSON(group.UpdatedAt, s.cfg.Location),
			"messageCount":   groupStats.MessageCount,
			"activeMembers":  groupStats.ActiveMembers,
			"botWhitelisted": stringSetContains(whitelists["bot"], group.RoomID),
			"aiWhitelisted":  stringSetContains(whitelists["ai"], group.RoomID),
		}
		if groupStats.LastMessageAt.Valid {
			item["lastMessageAt"] = unixJSON(groupStats.LastMessageAt.Float64, s.cfg.Location)
		}
		items = append(items, item)
	}
	ok(w, map[string]interface{}{
		"botId": effectiveBotID,
		"items": items,
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + pageSize - 1) / pageSize,
		},
	})
}

func (s *Server) groupMessageStats(ctx context.Context, roomIDs []string) (map[string]managedGroupMessageStats, error) {
	result := make(map[string]managedGroupMessageStats, len(roomIDs))
	if len(roomIDs) == 0 {
		return result, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(roomIDs)), ",")
	args := make([]interface{}, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		args = append(args, roomID)
	}
	rows, err := s.db.QueryContext(ctx, managedGroupMessageStatsQuery(placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var roomID string
		var stats managedGroupMessageStats
		if err := rows.Scan(&roomID, &stats.MessageCount, &stats.ActiveMembers, &stats.LastMessageAt); err != nil {
			return nil, err
		}
		result[roomID] = stats
	}
	return result, rows.Err()
}

func managedGroupMessageStatsQuery(placeholders string) string {
	return `
		SELECT room_id, COUNT(*) AS message_count, COUNT(DISTINCT NULLIF(sender_wxid, '')) AS active_members, MAX(created_at) AS last_message_at
		FROM group_messages
		WHERE room_id IN (` + placeholders + `)
		GROUP BY room_id
	`
}

func (s *Server) groupManagementMembers(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureWechatGroupIndexes(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "ensure group indexes failed")
		return
	}
	roomID := strings.TrimSpace(r.PathValue("roomID"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	page := positiveInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := positiveInt(r.URL.Query().Get("pageSize"), 50, 1, 200)
	offset := (page - 1) * pageSize

	var total int
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(DISTINCT sender_wxid)
		FROM group_messages
		WHERE room_id = ? AND sender_wxid <> ''
	`, roomID).Scan(&total); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count members failed")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT sender_wxid, SUBSTRING_INDEX(GROUP_CONCAT(sender_name ORDER BY created_at DESC SEPARATOR '\n'), '\n', 1) AS sender_name,
		       COUNT(*) AS message_count, MIN(created_at) AS first_message_at, MAX(created_at) AS last_message_at
		FROM group_messages
		WHERE room_id = ? AND sender_wxid <> ''
		GROUP BY sender_wxid
		ORDER BY last_message_at DESC
		LIMIT ? OFFSET ?
	`, roomID, pageSize, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query members failed")
		return
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var wxid, name string
		var messageCount int
		var firstMessageAt, lastMessageAt float64
		if err := rows.Scan(&wxid, &name, &messageCount, &firstMessageAt, &lastMessageAt); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan members failed")
			return
		}
		items = append(items, map[string]interface{}{
			"memberWxid":     wxid,
			"displayName":    name,
			"messageCount":   messageCount,
			"firstMessageAt": unixJSON(firstMessageAt, s.cfg.Location),
			"lastMessageAt":  unixJSON(lastMessageAt, s.cfg.Location),
		})
	}
	ok(w, map[string]interface{}{
		"data": items,
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + pageSize - 1) / pageSize,
		},
	})
}

func (s *Server) groupManagementEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureWechatGroupIndexes(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "ensure group indexes failed")
		return
	}
	roomID := strings.TrimSpace(r.PathValue("roomID"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	page := positiveInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := positiveInt(r.URL.Query().Get("pageSize"), 50, 1, 200)
	offset := (page - 1) * pageSize

	var total int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM group_member_events WHERE room_id = ?`, roomID).Scan(&total); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count member events failed")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, room_id, room_name, action, member_wxid, member_name, member_count, raw_payload, created_at
		FROM group_member_events
		WHERE room_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, roomID, pageSize, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query member events failed")
		return
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var rowRoomID, roomName, action, memberWxid, memberName string
		var memberCount sql.NullInt64
		var rawPayload sql.NullString
		var createdAt float64
		if err := rows.Scan(&id, &rowRoomID, &roomName, &action, &memberWxid, &memberName, &memberCount, &rawPayload, &createdAt); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan member events failed")
			return
		}
		item := map[string]interface{}{
			"id":         id,
			"roomId":     rowRoomID,
			"roomName":   roomName,
			"action":     action,
			"memberWxid": memberWxid,
			"memberName": memberName,
			"rawPayload": nullString(rawPayload),
			"createdAt":  unixJSON(createdAt, s.cfg.Location),
		}
		if memberCount.Valid {
			item["memberCount"] = memberCount.Int64
		}
		items = append(items, item)
	}
	ok(w, map[string]interface{}{
		"data": items,
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + pageSize - 1) / pageSize,
		},
	})
}

func (s *Server) updateGroupWhitelist(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.PathValue("roomID"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	var req groupWhitelistUpdateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	req.BotID = cleanBotID(req.BotID)
	if req.BotID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "botId is required")
		return
	}
	section := strings.TrimSpace(req.Type)
	if section != "bot" && section != "ai" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "type must be bot or ai")
		return
	}
	if err := s.saveGroupWhitelist(r.Context(), req.BotID, section, roomID, req.Enabled); err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save group whitelist failed")
		return
	}
	ok(w, map[string]interface{}{"botId": req.BotID, "roomId": roomID, "type": section, "enabled": req.Enabled})
}

func (s *Server) groupWhitelists(ctx context.Context, botID string) (map[string]map[string]struct{}, string, error) {
	raw, effectiveBotID, err := s.wxbotConfigRaw(ctx, botID)
	if err != nil {
		return nil, "", err
	}
	return map[string]map[string]struct{}{
		"bot": botGroupWhitelistSet(configStringList(raw, "bot", "group_whitelist")),
		"ai":  botGroupWhitelistSet(configStringList(raw, "ai", "group_whitelist")),
	}, effectiveBotID, nil
}

func (s *Server) saveGroupWhitelist(ctx context.Context, botID, section, roomID string, enabled bool) error {
	raw, _, err := s.wxbotConfigRaw(ctx, botID)
	if err != nil {
		return err
	}
	cfg, err := unwrapWxbotConfig(raw)
	if err != nil {
		return err
	}
	values, ok := cfg[section].(map[string]interface{})
	if !ok {
		values = map[string]interface{}{}
		cfg[section] = values
	}
	current := configStringList(raw, section, "group_whitelist")
	values["group_whitelist"] = updateStringList(current, roomID, enabled)

	encoded, _ := json.Marshal(cfg)
	normalized, err := normalizeWxbotConfig(encoded)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE wxbot_agents
		SET config_json = ?, config_updated_at = NOW(), updated_at = NOW()
		WHERE bot_id = ?
	`, string(normalized), botID)
	return err
}

func (s *Server) wxbotConfigRaw(ctx context.Context, botID string) (json.RawMessage, string, error) {
	if err := s.ensureWxbotSchema(ctx); err != nil {
		return nil, "", err
	}
	query := `
		SELECT bot_id, COALESCE(config_json, JSON_OBJECT()), COALESCE(current_config_json, JSON_OBJECT())
		FROM wxbot_agents
	`
	args := []interface{}{}
	if botID != "" {
		query += " WHERE bot_id = ?"
		args = append(args, botID)
	}
	query += " ORDER BY config_updated_at DESC, last_seen_at DESC, bot_id LIMIT 1"

	var effectiveBotID string
	var configRaw, currentRaw []byte
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&effectiveBotID, &configRaw, &currentRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return emptyWxbotConfig(), botID, nil
	}
	if err != nil {
		return nil, "", err
	}
	if isEmptyConfig(configRaw) && !isEmptyConfig(currentRaw) {
		configRaw = currentRaw
	}
	return json.RawMessage(configRaw), effectiveBotID, nil
}

func configStringList(raw json.RawMessage, section, key string) []string {
	cfg, err := unwrapWxbotConfig(raw)
	if err != nil {
		return nil
	}
	values, ok := cfg[section].(map[string]interface{})
	if !ok {
		return nil
	}
	items, ok := values[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(toString(item))
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func updateStringList(values []string, value string, enabled bool) []string {
	value = strings.TrimSpace(value)
	result := make([]string, 0, len(values)+1)
	seen := map[string]struct{}{}
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" || item == value {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	if enabled && value != "" {
		result = append(result, value)
	}
	return result
}

func stringSetContains(set map[string]struct{}, value string) bool {
	_, ok := set[strings.TrimSpace(value)]
	return ok
}

func (s *Server) ensureWechatGroupIndexes(ctx context.Context) error {
	for _, item := range []struct{ table, name, ddl string }{
		{"group_info", "idx_updated_at", "ALTER TABLE group_info ADD INDEX idx_updated_at (updated_at)"},
		{"group_messages", "idx_room_created", "ALTER TABLE group_messages ADD INDEX idx_room_created (room_id, created_at)"},
		{"group_messages", "idx_room_sender_created", "ALTER TABLE group_messages ADD INDEX idx_room_sender_created (room_id, sender_wxid, created_at)"},
		{"group_member_events", "idx_room_created", "ALTER TABLE group_member_events ADD INDEX idx_room_created (room_id, created_at)"},
	} {
		var count int
		var err error
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.tables
			WHERE table_schema = DATABASE()
			  AND table_name = ?
		`, item.table).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			continue
		}
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.statistics
			WHERE table_schema = DATABASE()
			  AND table_name = ?
			  AND index_name = ?
		`, item.table, item.name).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			if _, err := s.db.ExecContext(ctx, item.ddl); err != nil {
				return err
			}
		}
	}
	return nil
}

func isEmptyConfig(raw json.RawMessage) bool {
	cfg, err := unwrapWxbotConfig(raw)
	return err != nil || len(cfg) == 0
}
