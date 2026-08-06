package wechatadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
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

type groupMemberRow struct {
	MemberWxid      string
	DisplayName     string
	Nickname        string
	Alias           string
	Remark          string
	Sex             sql.NullInt64
	Country         string
	Province        string
	City            string
	Signature       string
	BigHeadImgURL   string
	SmallHeadImgURL string
	HeadImgMD5      string
	IsInChatRoom    sql.NullBool
	ProfileSyncedAt sql.NullString
	GroupSyncedAt   sql.NullString
	ProfileError    string
}

type groupMemberEventRow struct {
	ID              int64
	RoomID          string
	RoomName        string
	Action          string
	MemberWxid      string
	MemberName      string
	MemberRoomName  string
	Alias           string
	Remark          string
	Sex             sql.NullInt64
	Country         string
	Province        string
	City            string
	BigHeadImgURL   string
	SmallHeadImgURL string
	ProfileSyncedAt sql.NullString
	MemberCount     sql.NullInt64
	RawPayload      sql.NullString
	CreatedAt       float64
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

	orderBy, orderArgs := managedGroupOrderBy(whitelists["bot"], whitelists["ai"])
	listArgs := append(orderArgs, pageSize, offset)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT room_id, room_name, COALESCE(member_count, 0), COALESCE(owner_wxid, ''), COALESCE(updated_at, 0)
		FROM group_info
		WHERE `+managedGroupVisibleWhere+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, listArgs...)
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

func managedGroupOrderBy(botWhitelist, aiWhitelist map[string]struct{}) (string, []interface{}) {
	botRoomIDs := sortedStringSetValues(botWhitelist)
	aiRoomIDs := sortedStringSetValues(aiWhitelist)
	if len(botRoomIDs) == 0 && len(aiRoomIDs) == 0 {
		return "updated_at DESC, room_id ASC", nil
	}

	orderParts := []string{"CASE"}
	args := make([]interface{}, 0, len(botRoomIDs)*2+len(aiRoomIDs)*2)
	botClause, botArgs := roomIDInClause(botRoomIDs)
	aiClause, aiArgs := roomIDInClause(aiRoomIDs)

	if botClause != "" && aiClause != "" {
		orderParts = append(orderParts, "WHEN "+botClause+" AND "+aiClause+" THEN 0")
		args = append(args, botArgs...)
		args = append(args, aiArgs...)
	}
	if botClause != "" {
		orderParts = append(orderParts, "WHEN "+botClause+" THEN 1")
		args = append(args, botArgs...)
	}
	if aiClause != "" {
		orderParts = append(orderParts, "WHEN "+aiClause+" THEN 2")
		args = append(args, aiArgs...)
	}
	orderParts = append(orderParts, "ELSE 3 END, updated_at DESC, room_id ASC")
	return strings.Join(orderParts, " "), args
}

func sortedStringSetValues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func roomIDInClause(roomIDs []string) (string, []interface{}) {
	if len(roomIDs) == 0 {
		return "", nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(roomIDs)), ",")
	args := make([]interface{}, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		args = append(args, roomID)
	}
	return "room_id IN (" + placeholders + ")", args
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
	if err := s.ensureWechatGroupProfileSchema(r.Context()); err != nil {
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
	keyword := groupMemberSearchKeyword(r)

	var total int
	countQuery, countArgs := groupMemberCountQuery(roomID, keyword)
	if err := s.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count members failed")
		return
	}
	listQuery, listArgs := groupMemberRowsQuery(roomID, keyword, pageSize, offset)
	rows, err := s.db.QueryContext(r.Context(), listQuery, listArgs...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query members failed")
		return
	}
	defer rows.Close()

	memberRows := make([]groupMemberRow, 0)
	for rows.Next() {
		var row groupMemberRow
		if err := rows.Scan(
			&row.MemberWxid, &row.DisplayName, &row.Nickname, &row.Alias, &row.Remark, &row.Sex,
			&row.Country, &row.Province, &row.City, &row.Signature, &row.BigHeadImgURL, &row.SmallHeadImgURL, &row.HeadImgMD5,
			&row.IsInChatRoom, &row.ProfileSyncedAt, &row.GroupSyncedAt, &row.ProfileError,
		); err != nil {
			fail(w, http.StatusInternalServerError, "QUERY_FAILED", "scan members failed")
			return
		}
		memberRows = append(memberRows, row)
	}
	ok(w, map[string]interface{}{
		"data": groupMemberItems(memberRows, s.cfg.Location),
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + pageSize - 1) / pageSize,
		},
	})
}

func groupMemberItems(rows []groupMemberRow, _ *time.Location) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		item := map[string]interface{}{
			"memberWxid":       row.MemberWxid,
			"displayName":      row.DisplayName,
			"nickname":         row.Nickname,
			"alias":            row.Alias,
			"remark":           row.Remark,
			"country":          row.Country,
			"province":         row.Province,
			"city":             row.City,
			"signature":        row.Signature,
			"bigHeadImgUrl":    row.BigHeadImgURL,
			"smallHeadImgUrl":  row.SmallHeadImgURL,
			"headImgMd5":       row.HeadImgMD5,
			"profileSyncError": row.ProfileError,
		}
		if row.Sex.Valid {
			item["sex"] = row.Sex.Int64
		}
		if row.IsInChatRoom.Valid {
			item["isInChatRoom"] = row.IsInChatRoom.Bool
		}
		if row.ProfileSyncedAt.Valid {
			item["profileSyncedAt"] = row.ProfileSyncedAt.String
		}
		if row.GroupSyncedAt.Valid {
			item["groupInfoSyncedAt"] = row.GroupSyncedAt.String
		}
		items = append(items, item)
	}
	return items
}

func groupMemberSearchKeyword(r *http.Request) string {
	for _, key := range []string{"keyword", "search", "q"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func groupMemberCountQuery(roomID, keyword string) (string, []interface{}) {
	query := `
		SELECT COUNT(*)
		FROM (
			SELECT member_wxid
			FROM wechat_group_member_profiles
			WHERE room_id = ? AND member_wxid <> ''
			UNION
			SELECT sender_wxid AS member_wxid
			FROM group_messages
			WHERE room_id = ? AND sender_wxid <> ''
		) members
		LEFT JOIN wechat_group_member_profiles p ON p.room_id = ? AND p.member_wxid = members.member_wxid
	`
	args := []interface{}{roomID, roomID, roomID}
	if keyword != "" {
		query += " WHERE " + groupMemberSearchCondition("members", "p")
		args = append(args, groupMemberSearchArgs(roomID, keyword)...)
	}
	return query, args
}

func groupMemberRowsQuery(roomID, keyword string, limit, offset int) (string, []interface{}) {
	where := ""
	args := []interface{}{roomID, roomID, roomID, roomID}
	if keyword != "" {
		where = " WHERE " + groupMemberSearchCondition("members", "p")
		args = append(args, groupMemberSearchArgs(roomID, keyword)...)
	}
	args = append(args, limit, offset)
	return `
		SELECT members.member_wxid,
		       COALESCE(NULLIF(TRIM(p.display_name), ''), (
			       SELECT NULLIF(TRIM(gm.sender_name), '')
			       FROM group_messages gm
			       WHERE gm.room_id = ? AND gm.sender_wxid = members.member_wxid AND TRIM(gm.sender_name) <> ''
			       ORDER BY gm.created_at DESC
			       LIMIT 1
		       ), '') AS display_name,
		       COALESCE(p.nickname, ''), COALESCE(p.alias, ''), COALESCE(p.remark, ''), p.sex,
		       COALESCE(p.country, ''), COALESCE(p.province, ''), COALESCE(p.city, ''), COALESCE(p.signature, ''),
		       COALESCE(p.big_head_img_url, ''), COALESCE(p.small_head_img_url, ''), COALESCE(p.head_img_md5, ''),
		       p.is_in_chat_room, DATE_FORMAT(p.profile_synced_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(p.group_info_synced_at, '%Y-%m-%d %H:%i:%s'), COALESCE(p.profile_sync_error, '')
		FROM (
			SELECT member_wxid
			FROM wechat_group_member_profiles
			WHERE room_id = ? AND member_wxid <> ''
			UNION
			SELECT sender_wxid AS member_wxid
			FROM group_messages
			WHERE room_id = ? AND sender_wxid <> ''
		) members
		LEFT JOIN wechat_group_member_profiles p ON p.room_id = ? AND p.member_wxid = members.member_wxid
		` + where + `
		ORDER BY
			CASE WHEN NULLIF(TRIM(p.display_name), '') IS NULL THEN 1 ELSE 0 END,
			CASE WHEN NULLIF(TRIM(p.nickname), '') IS NULL THEN 1 ELSE 0 END,
			COALESCE(p.updated_at, FROM_UNIXTIME(0)) DESC,
			members.member_wxid ASC
		LIMIT ? OFFSET ?
	`, args
}

func latestGroupMemberNameExpr(alias string) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	return "COALESCE(SUBSTRING_INDEX(GROUP_CONCAT(NULLIF(TRIM(" + prefix + "sender_name), '') ORDER BY " + prefix + "created_at DESC SEPARATOR '|#|'), '|#|', 1), '')"
}

func groupMemberMessageSearchCondition(alias string) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	return "(" + prefix + "sender_wxid LIKE ? ESCAPE '\\\\' OR " + prefix + "sender_name LIKE ? ESCAPE '\\\\')"
}

func groupMemberMessageSearchArgs(keyword string) []interface{} {
	pattern := likePattern(keyword)
	return []interface{}{pattern, pattern}
}

func groupMemberSearchCondition(memberAlias, profileAlias string) string {
	memberPrefix := ""
	if memberAlias != "" {
		memberPrefix = memberAlias + "."
	}
	profilePrefix := ""
	if profileAlias != "" {
		profilePrefix = profileAlias + "."
	}
	return "(" + memberPrefix + "member_wxid LIKE ? ESCAPE '\\\\'" +
		" OR " + profilePrefix + "display_name LIKE ? ESCAPE '\\\\'" +
		" OR " + profilePrefix + "nickname LIKE ? ESCAPE '\\\\'" +
		" OR " + profilePrefix + "alias LIKE ? ESCAPE '\\\\'" +
		" OR " + profilePrefix + "remark LIKE ? ESCAPE '\\\\'" +
		" OR EXISTS (" +
		"SELECT 1 FROM group_messages gm WHERE gm.room_id = ? AND gm.sender_wxid = " + memberPrefix + "member_wxid AND gm.sender_name LIKE ? ESCAPE '\\\\'" +
		"))"
}

func groupMemberSearchArgs(roomID, keyword string) []interface{} {
	pattern := likePattern(keyword)
	return []interface{}{pattern, pattern, pattern, pattern, pattern, roomID, pattern}
}

func (s *Server) groupManagementEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureWechatGroupProfileSchema(r.Context()); err != nil {
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
	keyword := groupMemberSearchKeyword(r)

	var total int
	countQuery, countArgs := groupMemberEventCountQuery(roomID, keyword)
	if err := s.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "count member events failed")
		return
	}
	eventRows, err := s.groupMemberEventRows(r.Context(), roomID, keyword, pageSize, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query member events failed")
		return
	}
	ok(w, map[string]interface{}{
		"data": groupMemberEventItems(eventRows, s.cfg.Location),
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + pageSize - 1) / pageSize,
		},
	})
}

func (s *Server) groupMemberEventRows(ctx context.Context, roomID, keyword string, limit, offset int) ([]groupMemberEventRow, error) {
	query, args := groupMemberEventRowsQuery(roomID, keyword, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]groupMemberEventRow, 0, limit)
	for rows.Next() {
		var item groupMemberEventRow
		if err := rows.Scan(
			&item.ID, &item.RoomID, &item.RoomName, &item.Action, &item.MemberWxid, &item.MemberName, &item.MemberRoomName,
			&item.Alias, &item.Remark, &item.Sex, &item.Country, &item.Province, &item.City, &item.BigHeadImgURL, &item.SmallHeadImgURL, &item.ProfileSyncedAt,
			&item.MemberCount, &item.RawPayload, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func groupMemberEventCountQuery(roomID, keyword string) (string, []interface{}) {
	where, args := groupMemberEventWhere(roomID, keyword)
	return `
		SELECT COUNT(*)
		FROM group_member_events e
		WHERE ` + where + `
	`, args
}

func groupMemberEventRowsQuery(roomID, keyword string, limit, offset int) (string, []interface{}) {
	where, args := groupMemberEventWhere(roomID, keyword)
	args = append(args, limit, offset)
	return `
		SELECT e.id, e.room_id, e.room_name, e.action, e.member_wxid, e.member_name, COALESCE(e.member_room_name, ''),
		       COALESCE(p.alias, ''), COALESCE(p.remark, ''), p.sex,
		       COALESCE(p.country, ''), COALESCE(p.province, ''), COALESCE(p.city, ''),
		       COALESCE(p.big_head_img_url, ''), COALESCE(p.small_head_img_url, ''),
		       DATE_FORMAT(p.profile_synced_at, '%Y-%m-%d %H:%i:%s'),
		       e.member_count, e.raw_payload, e.created_at
		FROM group_member_events e
		LEFT JOIN wechat_group_member_profiles p ON p.room_id = e.room_id AND p.member_wxid = e.member_wxid
		WHERE ` + where + `
		ORDER BY e.created_at DESC
		LIMIT ? OFFSET ?
	`, args
}

func groupMemberEventWhere(roomID, keyword string) (string, []interface{}) {
	where := "e.room_id = ?"
	args := []interface{}{roomID}
	if keyword == "" {
		return where, args
	}
	pattern := likePattern(keyword)
	where += ` AND (
		e.member_wxid LIKE ? ESCAPE '\\'
		OR e.member_name LIKE ? ESCAPE '\\'
		OR e.member_room_name LIKE ? ESCAPE '\\'
		OR e.member_wxid IN (
			SELECT DISTINCT sender_wxid
			FROM group_messages
			WHERE room_id = ? AND sender_wxid <> '' AND sender_name LIKE ? ESCAPE '\\'
		)
	)`
	args = append(args, pattern, pattern, pattern, roomID, pattern)
	return where, args
}

func groupMemberEventItems(rows []groupMemberEventRow, loc *time.Location) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		rawPayload := ""
		if row.RawPayload.Valid {
			rawPayload = row.RawPayload.String
		}
		details := groupMemberEventRawDetails(rawPayload)
		item := map[string]interface{}{
			"id":              row.ID,
			"roomId":          row.RoomID,
			"roomName":        row.RoomName,
			"action":          row.Action,
			"memberWxid":      row.MemberWxid,
			"memberName":      row.MemberName,
			"memberRoomName":  row.MemberRoomName,
			"alias":           row.Alias,
			"remark":          row.Remark,
			"country":         row.Country,
			"province":        row.Province,
			"city":            row.City,
			"bigHeadImgUrl":   row.BigHeadImgURL,
			"smallHeadImgUrl": row.SmallHeadImgURL,
			"rawPayload":      rawPayload,
			"rawDetails":      details,
			"createdAt":       unixJSON(row.CreatedAt, loc),
		}
		if row.Sex.Valid {
			item["sex"] = row.Sex.Int64
		}
		if row.ProfileSyncedAt.Valid {
			item["profileSyncedAt"] = row.ProfileSyncedAt.String
		}
		if row.MemberCount.Valid {
			item["memberCount"] = row.MemberCount.Int64
		}
		items = append(items, item)
	}
	return items
}

func groupMemberEventRawDetails(raw string) map[string]string {
	result := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return result
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return result
	}
	data := payload
	if nested, ok := payload["data"].(map[string]interface{}); ok {
		data = nested
	}
	member := firstMap(data, "memberlist", "memberList", "members", "member")
	pairs := []struct {
		out  string
		src  map[string]interface{}
		keys []string
	}{
		{"rawRoomId", data, []string{"roomid", "room_id", "roomId", "chatroom_id", "chatroomId"}},
		{"rawRoomName", data, []string{"roomname", "room_name", "roomName", "chatroom_name"}},
		{"rawMemberWxid", member, []string{"userName", "username", "wxid", "member_wxid", "memberWxid"}},
		{"rawMemberName", member, []string{"nickName", "nickname", "displayName", "remark"}},
		{"rawMemberRoomName", member, []string{"displayName", "chatroomNickName", "chatroom_nick_name", "roomNickName", "room_nick_name"}},
		{"inviterWxid", data, []string{"inviterUserName", "inviter_username", "inviterWxid", "inviter_wxid", "inviteUserName", "invite_user_name"}},
		{"inviterName", data, []string{"inviterNickName", "inviter_nickname", "inviterName", "inviter_name", "inviteNickName", "invite_nick_name"}},
		{"operatorWxid", data, []string{"operatorUserName", "operator_username", "operatorWxid", "operator_wxid", "adminUserName", "admin_user_name"}},
		{"operatorName", data, []string{"operatorNickName", "operator_nickname", "operatorName", "operator_name", "adminNickName", "admin_nick_name"}},
		{"eventType", data, []string{"type", "event", "eventType", "event_type"}},
	}
	for _, pair := range pairs {
		if value := firstText(pair.src, pair.keys...); value != "" {
			result[pair.out] = value
		}
	}
	return result
}

func firstMap(data map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		value := data[key]
		if items, ok := value.([]interface{}); ok {
			if len(items) == 0 {
				continue
			}
			value = items[0]
		}
		if item, ok := value.(map[string]interface{}); ok {
			return item
		}
	}
	return map[string]interface{}{}
}

func firstText(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := data[key]
		if !ok || value == nil {
			continue
		}
		if item, ok := value.(map[string]interface{}); ok {
			value = item["String"]
			if value == nil {
				value = item["string"]
			}
		}
		text := strings.TrimSpace(toString(value))
		if text != "" {
			return text
		}
	}
	return ""
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
	if err := s.ensureColumnIfTableExists(ctx, "group_member_events", "member_room_name", "ALTER TABLE group_member_events ADD COLUMN member_room_name VARCHAR(255) NOT NULL DEFAULT '' AFTER member_name"); err != nil {
		return err
	}
	for _, item := range []struct{ table, name, ddl string }{
		{"group_info", "idx_updated_at", "ALTER TABLE group_info ADD INDEX idx_updated_at (updated_at)"},
		{"group_messages", "idx_room_created", "ALTER TABLE group_messages ADD INDEX idx_room_created (room_id, created_at)"},
		{"group_messages", "idx_room_sender_created", "ALTER TABLE group_messages ADD INDEX idx_room_sender_created (room_id, sender_wxid, created_at)"},
		{"group_member_events", "idx_room_created", "ALTER TABLE group_member_events ADD INDEX idx_room_created (room_id, created_at)"},
		{"group_member_events", "idx_member_events_room_name", "ALTER TABLE group_member_events ADD INDEX idx_member_events_room_name (room_id, member_room_name)"},
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

func (s *Server) ensureColumnIfTableExists(ctx context.Context, table, column, ddl string) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name = ?
	`, table).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		  AND column_name = ?
	`, table, column).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, ddl)
	return err
}

func (s *Server) ensureWechatGroupProfileSchema(ctx context.Context) error {
	if err := s.ensureWechatGroupIndexes(ctx); err != nil {
		return err
	}
	for _, ddl := range []string{wechatGroupMemberProfilesSchemaSQL, wechatGroupMemberProfileSyncStateSchemaSQL} {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	return nil
}

const wechatGroupMemberProfilesSchemaSQL = `
CREATE TABLE IF NOT EXISTS wechat_group_member_profiles (
	room_id VARCHAR(128) NOT NULL,
	member_wxid VARCHAR(128) NOT NULL,
	nickname VARCHAR(255) NOT NULL DEFAULT '',
	display_name VARCHAR(255) NOT NULL DEFAULT '',
	remark VARCHAR(255) NOT NULL DEFAULT '',
	alias VARCHAR(255) NOT NULL DEFAULT '',
	sex INT NULL,
	country VARCHAR(128) NOT NULL DEFAULT '',
	province VARCHAR(128) NOT NULL DEFAULT '',
	city VARCHAR(128) NOT NULL DEFAULT '',
	signature TEXT NULL,
	big_head_img_url TEXT NULL,
	small_head_img_url TEXT NULL,
	head_img_md5 VARCHAR(128) NOT NULL DEFAULT '',
	chatroom_member_flag INT NULL,
	status INT NULL,
	inviter_user_name VARCHAR(128) NOT NULL DEFAULT '',
	add_chatroom_scene_xml TEXT NULL,
	is_in_chat_room BOOLEAN NULL,
	last_seen_message_at DOUBLE NULL,
	group_info_synced_at DATETIME NULL,
	profile_synced_at DATETIME NULL,
	profile_sync_error TEXT NULL,
	raw_profile_json JSON NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (room_id, member_wxid),
	KEY idx_room_profile_synced_at (room_id, profile_synced_at),
	KEY idx_room_last_seen_message_at (room_id, last_seen_message_at),
	KEY idx_room_updated_at (room_id, updated_at),
	KEY idx_room_display_name (room_id, display_name),
	KEY idx_room_nickname (room_id, nickname),
	KEY idx_room_alias (room_id, alias),
	KEY idx_alias (alias),
	KEY idx_nickname (nickname),
	KEY idx_display_name (display_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`

const wechatGroupMemberProfileSyncStateSchemaSQL = `
CREATE TABLE IF NOT EXISTS wechat_group_member_profile_sync_state (
	room_id VARCHAR(128) NOT NULL,
	status VARCHAR(32) NOT NULL DEFAULT 'idle',
	sync_type VARCHAR(32) NOT NULL DEFAULT 'incremental',
	cursor_member_wxid VARCHAR(128) NOT NULL DEFAULT '',
	last_full_synced_at DATETIME NULL,
	last_incremental_synced_at DATETIME NULL,
	processed_count INT NOT NULL DEFAULT 0,
	failed_count INT NOT NULL DEFAULT 0,
	last_error TEXT NULL,
	locked_until DATETIME NULL,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (room_id),
	KEY idx_status_locked_until (status, locked_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`

func isEmptyConfig(raw json.RawMessage) bool {
	cfg, err := unwrapWxbotConfig(raw)
	return err != nil || len(cfg) == 0
}
