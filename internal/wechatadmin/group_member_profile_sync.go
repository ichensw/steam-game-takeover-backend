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
	"net/url"
	"strings"
	"sync"
)

const (
	groupMemberProfileSyncStatusRunning   = "running"
	groupMemberProfileSyncStatusFailed    = "failed"
	groupMemberProfileSyncStatusSucceeded = "succeeded"
	groupMemberProfileSyncStatusIdle      = "idle"
)

type groupMemberProfileSyncRequest struct {
	Mode string `json:"mode"`
}

type wxbotMemberProfileSyncProgressRequest struct {
	BotID            string `json:"botId"`
	RoomID           string `json:"roomId"`
	Status           string `json:"status"`
	SyncType         string `json:"syncType"`
	CursorMemberWxid string `json:"cursorMemberWxid"`
	ProcessedCount   int    `json:"processedCount"`
	FailedCount      int    `json:"failedCount"`
	ErrorMessage     string `json:"errorMessage"`
}

type hookRoomMembersResponse struct {
	ChatroomUserName      string              `json:"chatroomUserName"`
	ChatRoomOwner         string              `json:"chatRoomOwner"`
	AllMemberCount        int                 `json:"allMemberCount"`
	AdminCount            int                 `json:"adminCount"`
	AllMemberUserNameList []hookStringValue   `json:"allMemberUserNameList"`
	NewChatroomData       hookNewChatroomData `json:"newChatroomData"`
	BaseResponse          hookBaseResponse    `json:"baseResponse"`
	Ret                   int                 `json:"ret"`
	ErrMsg                string              `json:"errMsg"`
}

type hookNewChatroomData struct {
	MemberCount    int                  `json:"memberCount"`
	ChatRoomMember []hookChatroomMember `json:"chatRoomMember"`
}

type hookChatroomMember struct {
	UserName               hookStringValue `json:"userName"`
	NickName               hookStringValue `json:"nickName"`
	DisplayName            string          `json:"displayName"`
	BigHeadImgURL          string          `json:"bigHeadImgUrl"`
	SmallHeadImgURL        string          `json:"smallHeadImgUrl"`
	HeadImgMD5             string          `json:"headImgMd5"`
	InviterUserName        string          `json:"inviterUserName"`
	AddChatRoomSceneNewXML string          `json:"addChatRoomSceneNewXml"`
	ChatroomMemberFlag     int             `json:"chatroomMemberFlag"`
	Status                 int             `json:"status"`
}

type hookGroupMemberContactResponse struct {
	ContactList  []hookContact    `json:"contactList"`
	BaseResponse hookBaseResponse `json:"baseResponse"`
	Ret          int              `json:"ret"`
	ErrMsg       string           `json:"errMsg"`
}

type hookContact struct {
	UserName        hookStringValue     `json:"userName"`
	FriendUserName  string              `json:"friendUserName"`
	NickName        hookStringValue     `json:"nickName"`
	Remark          hookStringValue     `json:"remark"`
	Alias           string              `json:"alias"`
	Sex             int                 `json:"sex"`
	Country         string              `json:"country"`
	Province        string              `json:"province"`
	City            string              `json:"city"`
	Signature       string              `json:"signature"`
	BigHeadImgURL   string              `json:"bigHeadImgUrl"`
	SmallHeadImgURL string              `json:"smallHeadImgUrl"`
	HeadImgMD5      string              `json:"headImgMd5"`
	VerifyFlag      int                 `json:"verifyFlag"`
	ContactType     int                 `json:"contactType"`
	DeleteFlag      int                 `json:"deleteFlag"`
	Status          int                 `json:"status"`
	IsInChatRoom    bool                `json:"isInChatRoom"`
	NewChatroomData hookNewChatroomData `json:"newChatroomData"`
}

type hookGroupMemberInfoResponse struct {
	DisplayName string `json:"displayName"`
	Nick        string `json:"nick"`
	RoomID      string `json:"roomId"`
}

type hookStringValue struct {
	String string `json:"String"`
}

type hookBaseResponse struct {
	Ret    int             `json:"ret"`
	ErrMsg json.RawMessage `json:"errMsg"`
}

func (s *Server) startGroupMemberProfileSync(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.PathValue("roomID"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	var req groupMemberProfileSyncRequest
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req)
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "incremental"
	}
	if mode != "full" && mode != "incremental" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "mode must be full or incremental")
		return
	}
	if err := s.ensureWechatGroupProfileSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure profile schema failed")
		return
	}
	locked, err := s.acquireGroupMemberProfileSync(r.Context(), roomID, mode, "")
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "start profile sync failed")
		return
	}
	if !locked {
		fail(w, http.StatusConflict, "SYNC_RUNNING", "member profile sync is already running")
		return
	}
	state, err := s.groupMemberProfileSyncState(r.Context(), roomID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query profile sync state failed")
		return
	}
	ok(w, state)
}

func (s *Server) groupMemberProfileSyncStatus(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.PathValue("roomID"))
	if roomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	if err := s.ensureWechatGroupProfileSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure profile schema failed")
		return
	}
	state, err := s.groupMemberProfileSyncState(r.Context(), roomID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query profile sync state failed")
		return
	}
	ok(w, state)
}

func (s *Server) refreshGroupMemberProfile(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.PathValue("roomID"))
	memberWxid := strings.TrimSpace(r.PathValue("memberWxid"))
	if roomID == "" || memberWxid == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId and memberWxid are required")
		return
	}
	if err := s.ensureWechatGroupProfileSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure profile schema failed")
		return
	}
	locked, err := s.acquireGroupMemberProfileSync(r.Context(), roomID, "member", memberWxid)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "start profile refresh failed")
		return
	}
	if !locked {
		fail(w, http.StatusConflict, "SYNC_RUNNING", "member profile sync is already running")
		return
	}
	state, err := s.groupMemberProfileSyncState(r.Context(), roomID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query profile sync state failed")
		return
	}
	ok(w, state)
}

func (s *Server) acquireGroupMemberProfileSync(ctx context.Context, roomID, mode, targetMemberWxid string) (bool, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO wechat_group_member_profile_sync_state
			(room_id, status, sync_type, processed_count, failed_count, last_error, locked_until, updated_at)
		VALUES (?, ?, ?, 0, 0, NULL, DATE_ADD(NOW(), INTERVAL 15 MINUTE), NOW())
		ON DUPLICATE KEY UPDATE updated_at = updated_at
	`, roomID, groupMemberProfileSyncStatusIdle, mode); err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE wechat_group_member_profile_sync_state
		SET status = ?, sync_type = ?, cursor_member_wxid = ?, processed_count = 0, failed_count = 0,
		    last_error = NULL, locked_until = DATE_ADD(NOW(), INTERVAL 15 MINUTE), updated_at = NOW()
		WHERE room_id = ? AND (status <> ? OR locked_until IS NULL OR locked_until < NOW())
	`, groupMemberProfileSyncStatusRunning, mode, targetMemberWxid, roomID, groupMemberProfileSyncStatusRunning)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *Server) wxbotNextMemberProfileSyncTask(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureWechatGroupProfileSchema(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "SCHEMA_FAILED", "ensure profile schema failed")
		return
	}
	row, err := s.oneMemberProfileSyncTask(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		ok(w, map[string]interface{}{"task": nil})
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "QUERY_FAILED", "query profile sync task failed")
		return
	}
	ok(w, map[string]interface{}{"task": row})
}

func (s *Server) oneMemberProfileSyncTask(ctx context.Context) (map[string]interface{}, error) {
	var roomID, syncType, cursor string
	var processed, failed int
	err := s.db.QueryRowContext(ctx, `
		SELECT room_id, sync_type, cursor_member_wxid, processed_count, failed_count
		FROM wechat_group_member_profile_sync_state
		WHERE status = ? AND (locked_until IS NULL OR locked_until < DATE_ADD(NOW(), INTERVAL 20 MINUTE))
		ORDER BY updated_at ASC
		LIMIT 1
	`, groupMemberProfileSyncStatusRunning).Scan(&roomID, &syncType, &cursor, &processed, &failed)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE wechat_group_member_profile_sync_state
		SET locked_until = DATE_ADD(NOW(), INTERVAL 15 MINUTE), updated_at = NOW()
		WHERE room_id = ? AND status = ?
	`, roomID, groupMemberProfileSyncStatusRunning)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"roomId":           roomID,
		"syncType":         syncType,
		"cursorMemberWxid": cursor,
		"processedCount":   processed,
		"failedCount":      failed,
	}, nil
}

func (s *Server) wxbotMemberProfileSyncProgress(w http.ResponseWriter, r *http.Request) {
	var req wxbotMemberProfileSyncProgressRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid json body")
		return
	}
	req.RoomID = strings.TrimSpace(req.RoomID)
	if req.RoomID == "" {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "roomId is required")
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = groupMemberProfileSyncStatusRunning
	}
	if status != groupMemberProfileSyncStatusRunning && status != groupMemberProfileSyncStatusSucceeded && status != groupMemberProfileSyncStatusFailed {
		fail(w, http.StatusBadRequest, "PARAM_INVALID", "invalid status")
		return
	}
	errText := shortSyncText(req.ErrorMessage)
	lockedUntilExpr := "DATE_ADD(NOW(), INTERVAL 15 MINUTE)"
	if status != groupMemberProfileSyncStatusRunning {
		lockedUntilExpr = "NULL"
	}
	_, err := s.db.ExecContext(r.Context(), `
		UPDATE wechat_group_member_profile_sync_state
		SET status = ?, cursor_member_wxid = ?, processed_count = ?, failed_count = ?,
		    last_error = NULLIF(?, ''),
		    last_full_synced_at = IF(? = 'succeeded' AND sync_type = 'full', NOW(), last_full_synced_at),
		    last_incremental_synced_at = IF(? = 'succeeded' AND sync_type <> 'full', NOW(), last_incremental_synced_at),
		    locked_until = `+lockedUntilExpr+`, updated_at = NOW()
		WHERE room_id = ?
	`, status, strings.TrimSpace(req.CursorMemberWxid), nonNegativeInt(req.ProcessedCount), nonNegativeInt(req.FailedCount), errText, status, status, req.RoomID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "SAVE_FAILED", "save profile sync progress failed")
		return
	}
	ok(w, map[string]interface{}{"updated": true})
}

func (s *Server) groupMemberProfileSyncState(ctx context.Context, roomID string) (map[string]interface{}, error) {
	var status, syncType, cursor string
	var lastFull, lastIncremental, lockedUntil, updatedAt sql.NullString
	var processed, failed int
	var lastError sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT status, sync_type, cursor_member_wxid,
		       DATE_FORMAT(last_full_synced_at, '%Y-%m-%d %H:%i:%s'),
		       DATE_FORMAT(last_incremental_synced_at, '%Y-%m-%d %H:%i:%s'),
		       processed_count, failed_count, last_error,
		       DATE_FORMAT(locked_until, '%Y-%m-%d %H:%i:%s'),
		       DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s')
		FROM wechat_group_member_profile_sync_state
		WHERE room_id = ?
	`, roomID).Scan(&status, &syncType, &cursor, &lastFull, &lastIncremental, &processed, &failed, &lastError, &lockedUntil, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]interface{}{
			"roomId":         roomID,
			"status":         groupMemberProfileSyncStatusIdle,
			"syncType":       "",
			"processedCount": 0,
			"failedCount":    0,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	item := map[string]interface{}{
		"roomId":           roomID,
		"status":           status,
		"syncType":         syncType,
		"cursorMemberWxid": cursor,
		"processedCount":   processed,
		"failedCount":      failed,
	}
	setNullString(item, "lastFullSyncedAt", lastFull)
	setNullString(item, "lastIncrementalSyncedAt", lastIncremental)
	setNullString(item, "lastError", lastError)
	setNullString(item, "lockedUntil", lockedUntil)
	setNullString(item, "updatedAt", updatedAt)
	return item, nil
}

func setNullString(item map[string]interface{}, key string, value sql.NullString) {
	if value.Valid {
		item[key] = value.String
	}
}

func (s *Server) runGroupMemberProfileSync(ctx context.Context, roomID, mode, onlyMemberWxid string) {
	processed, failed, runErr := s.executeGroupMemberProfileSync(ctx, roomID, mode, onlyMemberWxid)
	status := groupMemberProfileSyncStatusSucceeded
	errText := ""
	if runErr != nil {
		status = groupMemberProfileSyncStatusFailed
		errText = shortSyncError(runErr)
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE wechat_group_member_profile_sync_state
		SET status = ?, processed_count = ?, failed_count = ?, last_error = NULLIF(?, ''),
		    last_full_synced_at = IF(? = 'full' AND ? = '', NOW(), last_full_synced_at),
		    last_incremental_synced_at = IF(? <> 'full' OR ? <> '', NOW(), last_incremental_synced_at),
		    locked_until = NULL, updated_at = NOW()
		WHERE room_id = ?
	`, status, processed, failed, errText, mode, onlyMemberWxid, mode, onlyMemberWxid, roomID)
}

func (s *Server) executeGroupMemberProfileSync(ctx context.Context, roomID, mode, onlyMemberWxid string) (int, int, error) {
	baseURL, err := s.hookAPIBaseURL(ctx)
	if err != nil {
		return 0, 0, err
	}
	members := []string{onlyMemberWxid}
	if onlyMemberWxid == "" {
		if mode == "full" {
			members, err = s.discoverGroupMembers(ctx, baseURL, roomID)
		} else {
			members, err = s.incrementalProfileMemberWxids(ctx, roomID, 100)
		}
		if err != nil {
			return 0, 0, err
		}
	}
	members = uniqueNonEmptyStrings(members)
	if len(members) == 0 {
		return 0, 0, nil
	}
	processed, failed := s.syncGroupMemberInfoBatch(ctx, baseURL, roomID, members)
	p2, f2 := s.syncGroupMemberContactBatch(ctx, baseURL, roomID, members)
	return processed + p2, failed + f2, nil
}

func (s *Server) hookAPIBaseURL(ctx context.Context) (string, error) {
	raw, err := s.wxbotRuntimeConfigRaw(ctx)
	if err != nil {
		return "", err
	}
	cfg, err := unwrapWxbotConfig(raw)
	if err != nil {
		return "", err
	}
	hook, _ := cfg["hook"].(map[string]interface{})
	for _, key := range []string{"api_base_url", "http_server_base_url", "base_url"} {
		if value := strings.TrimRight(strings.TrimSpace(toString(hook[key])), "/"); value != "" {
			return value, nil
		}
	}
	port := intFromConfigValue(hook["http_server_port"])
	if port <= 0 {
		return "", errors.New("hook api base url is not configured")
	}
	host := firstNonEmptyWechatString(toString(hook["http_server_host"]), toString(hook["host"]), toString(hook["tcp_ip"]), "127.0.0.1")
	if strings.TrimSpace(host) == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, port), nil
}

func (s *Server) wxbotRuntimeConfigRaw(ctx context.Context) (json.RawMessage, error) {
	if err := s.ensureWxbotSchema(ctx); err != nil {
		return nil, err
	}
	var currentRaw, configRaw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(current_config_json, JSON_OBJECT()), COALESCE(config_json, JSON_OBJECT())
		FROM wxbot_agents
		ORDER BY last_seen_at DESC, config_updated_at DESC, bot_id
		LIMIT 1
	`).Scan(&currentRaw, &configRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return emptyWxbotConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	if !isEmptyConfig(currentRaw) {
		return json.RawMessage(currentRaw), nil
	}
	return json.RawMessage(configRaw), nil
}

func intFromConfigValue(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		i, _ := typed.Int64()
		return int(i)
	default:
		return 0
	}
}

func (s *Server) discoverGroupMembers(ctx context.Context, baseURL, roomID string) ([]string, error) {
	var resp hookRoomMembersResponse
	if err := s.postHookJSON(ctx, baseURL, "/api/get_room_members", map[string]string{"room_id": roomID}, &resp); err != nil {
		return nil, err
	}
	if err := hookResponseError(resp.Ret, resp.ErrMsg, resp.BaseResponse); err != nil {
		return nil, err
	}
	memberSet := map[string]hookChatroomMember{}
	for _, value := range resp.AllMemberUserNameList {
		wxid := strings.TrimSpace(value.String)
		if wxid != "" {
			memberSet[wxid] = hookChatroomMember{}
		}
	}
	for _, member := range resp.NewChatroomData.ChatRoomMember {
		wxid := strings.TrimSpace(member.UserName.String)
		if wxid == "" {
			continue
		}
		memberSet[wxid] = member
	}
	members := make([]string, 0, len(memberSet))
	for wxid, member := range memberSet {
		members = append(members, wxid)
		if err := s.upsertDiscoveredGroupMember(ctx, roomID, wxid, member); err != nil {
			return nil, err
		}
	}
	return members, nil
}

func (s *Server) upsertDiscoveredGroupMember(ctx context.Context, roomID, wxid string, member hookChatroomMember) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO wechat_group_member_profiles
			(room_id, member_wxid, nickname, big_head_img_url, small_head_img_url, head_img_md5,
			 chatroom_member_flag, status, inviter_user_name, add_chatroom_scene_xml, is_in_chat_room, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0), ?, ?, TRUE, NOW())
		ON DUPLICATE KEY UPDATE
			nickname = IF(VALUES(nickname) <> '', VALUES(nickname), nickname),
			big_head_img_url = IF(VALUES(big_head_img_url) <> '', VALUES(big_head_img_url), big_head_img_url),
			small_head_img_url = IF(VALUES(small_head_img_url) <> '', VALUES(small_head_img_url), small_head_img_url),
			head_img_md5 = IF(VALUES(head_img_md5) <> '', VALUES(head_img_md5), head_img_md5),
			chatroom_member_flag = COALESCE(VALUES(chatroom_member_flag), chatroom_member_flag),
			status = COALESCE(VALUES(status), status),
			inviter_user_name = IF(VALUES(inviter_user_name) <> '', VALUES(inviter_user_name), inviter_user_name),
			add_chatroom_scene_xml = IF(VALUES(add_chatroom_scene_xml) <> '', VALUES(add_chatroom_scene_xml), add_chatroom_scene_xml),
			is_in_chat_room = TRUE,
			updated_at = NOW()
	`, roomID, wxid, member.NickName.String, member.BigHeadImgURL, member.SmallHeadImgURL, member.HeadImgMD5, member.ChatroomMemberFlag, member.Status, member.InviterUserName, member.AddChatRoomSceneNewXML)
	return err
}

func (s *Server) incrementalProfileMemberWxids(ctx context.Context, roomID string, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT members.member_wxid
		FROM (
			SELECT sender_wxid AS member_wxid, MAX(created_at) AS last_seen_message_at
			FROM group_messages
			WHERE room_id = ? AND sender_wxid <> '' AND created_at >= UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 24 HOUR))
			GROUP BY sender_wxid
			UNION
			SELECT member_wxid, COALESCE(last_seen_message_at, 0)
			FROM wechat_group_member_profiles
			WHERE room_id = ? AND (profile_synced_at IS NULL OR profile_synced_at < DATE_SUB(NOW(), INTERVAL 24 HOUR))
		) members
		LEFT JOIN wechat_group_member_profiles p ON p.room_id = ? AND p.member_wxid = members.member_wxid
		WHERE p.profile_synced_at IS NULL OR p.profile_synced_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)
		ORDER BY members.last_seen_message_at DESC
		LIMIT ?
	`, roomID, roomID, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []string
	for rows.Next() {
		var wxid string
		if err := rows.Scan(&wxid); err != nil {
			return nil, err
		}
		members = append(members, wxid)
	}
	return members, rows.Err()
}

func (s *Server) syncGroupMemberInfoBatch(ctx context.Context, baseURL, roomID string, members []string) (int, int) {
	return runProfileSyncWorkers(ctx, members, 8, func(memberWxid string) error {
		var resp hookGroupMemberInfoResponse
		if err := s.postHookJSON(ctx, baseURL, "/api/get_group_memeber_info", map[string]string{"roomId": roomID, "memeberId": memberWxid}, &resp); err != nil {
			return err
		}
		displayName := normalizeGroupMemberDisplayName(resp.DisplayName)
		if displayName == "" {
			return nil
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO wechat_group_member_profiles (room_id, member_wxid, display_name, group_info_synced_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE display_name = VALUES(display_name), group_info_synced_at = NOW(), updated_at = NOW()
		`, roomID, memberWxid, displayName)
		return err
	})
}

func (s *Server) syncGroupMemberContactBatch(ctx context.Context, baseURL, roomID string, members []string) (int, int) {
	return runProfileSyncWorkers(ctx, members, 3, func(memberWxid string) error {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE wechat_group_member_profile_sync_state
			SET cursor_member_wxid = ?, locked_until = DATE_ADD(NOW(), INTERVAL 15 MINUTE), updated_at = NOW()
			WHERE room_id = ?
		`, memberWxid, roomID); err != nil {
			return err
		}
		var resp hookGroupMemberContactResponse
		err := s.postHookJSON(ctx, baseURL, "/api/get_group_member_contact", map[string]string{"wxid": memberWxid, "roomId": roomID}, &resp)
		if err == nil {
			err = hookResponseError(resp.Ret, resp.ErrMsg, resp.BaseResponse)
		}
		if err == nil && len(resp.ContactList) == 0 {
			err = errors.New("empty contactList")
		}
		if err != nil {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO wechat_group_member_profiles (room_id, member_wxid, profile_sync_error, updated_at)
				VALUES (?, ?, ?, NOW())
				ON DUPLICATE KEY UPDATE profile_sync_error = VALUES(profile_sync_error), updated_at = NOW()
			`, roomID, memberWxid, shortSyncError(err))
			return err
		}
		return s.upsertGroupMemberContact(ctx, roomID, memberWxid, resp.ContactList[0])
	})
}

func (s *Server) upsertGroupMemberContact(ctx context.Context, roomID, memberWxid string, contact hookContact) error {
	raw, _ := json.Marshal(contact)
	wxid := firstNonEmptyWechatString(memberWxid, contact.UserName.String, contact.FriendUserName)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO wechat_group_member_profiles
			(room_id, member_wxid, nickname, remark, alias, sex, country, province, city, signature,
			 big_head_img_url, small_head_img_url, head_img_md5, chatroom_member_flag, status, is_in_chat_room,
			 profile_synced_at, profile_sync_error, raw_profile_json, updated_at)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0), ?, NOW(), NULL, ?, NOW())
		ON DUPLICATE KEY UPDATE
			nickname = VALUES(nickname),
			remark = VALUES(remark),
			alias = VALUES(alias),
			sex = COALESCE(VALUES(sex), sex),
			country = VALUES(country),
			province = VALUES(province),
			city = VALUES(city),
			signature = VALUES(signature),
			big_head_img_url = VALUES(big_head_img_url),
			small_head_img_url = VALUES(small_head_img_url),
			head_img_md5 = VALUES(head_img_md5),
			chatroom_member_flag = COALESCE(VALUES(chatroom_member_flag), chatroom_member_flag),
			status = COALESCE(VALUES(status), status),
			is_in_chat_room = VALUES(is_in_chat_room),
			profile_synced_at = NOW(),
			profile_sync_error = NULL,
			raw_profile_json = VALUES(raw_profile_json),
			updated_at = NOW()
	`, roomID, wxid, contact.NickName.String, contact.Remark.String, contact.Alias, contact.Sex, contact.Country, contact.Province, contact.City, contact.Signature, contact.BigHeadImgURL, contact.SmallHeadImgURL, contact.HeadImgMD5, contact.ContactType, contact.Status, contact.IsInChatRoom, string(raw))
	return err
}

func normalizeGroupMemberDisplayName(value string) string {
	text := strings.TrimSpace(value)
	switch text {
	case "未设置群昵称", "未设置群名片":
		return ""
	default:
		return text
	}
}

func (s *Server) postHookJSON(ctx context.Context, baseURL, path string, payload interface{}, out interface{}) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint, err := url.JoinPath(strings.TrimRight(baseURL, "/"), path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("hook api %s returned %d", path, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return err
	}
	return nil
}

func hookResponseError(ret int, errMsg string, base hookBaseResponse) error {
	if base.Ret != 0 {
		return fmt.Errorf("hook ret %d: %s", base.Ret, hookErrMessage(base.ErrMsg))
	}
	if ret != 0 {
		return fmt.Errorf("hook ret %d: %s", ret, strings.TrimSpace(errMsg))
	}
	return nil
}

func hookErrMessage(raw json.RawMessage) string {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "{}" || text == "null" {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return strings.TrimSpace(value)
	}
	return text
}

func runProfileSyncWorkers(ctx context.Context, members []string, concurrency int, fn func(string) error) (int, int) {
	if concurrency < 1 {
		concurrency = 1
	}
	jobs := make(chan string)
	var mu sync.Mutex
	processed := 0
	failed := 0
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for member := range jobs {
				err := fn(member)
				mu.Lock()
				processed++
				if err != nil {
					failed++
				}
				mu.Unlock()
			}
		}()
	}
dispatch:
	for _, member := range members {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- member:
		}
	}
	close(jobs)
	wg.Wait()
	return processed, failed
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func shortSyncError(err error) string {
	if err == nil {
		return ""
	}
	return shortSyncText(err.Error())
}

func shortSyncText(text string) string {
	msg := strings.TrimSpace(text)
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func firstNonEmptyWechatString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
