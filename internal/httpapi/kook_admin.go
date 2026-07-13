package httpapi

import (
	"fmt"
	"net/http"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

const kookAPIBaseURL = "https://www.kookapp.cn/api/v3"

type kookProxyResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (h *Handler) AdminListKookChannels(c *gin.Context) {
	channels, meta, err := h.fetchKookChannels()
	if err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, "kook channel query failed")
		return
	}
	ok(c, "success", gin.H{"list": channels, "meta": meta})
}

func (h *Handler) AdminGetKookChannel(c *gin.Context) {
	h.proxyKookGET(c, "/channel/view", gin.H{"target_id": c.Param("channelId")})
}

func (h *Handler) AdminCreateKookChannel(c *gin.Context) {
	h.proxyKookPOST(c, "/channel/create", gin.H{"guild_id": h.kookGuildID()})
}

func (h *Handler) AdminUpdateKookChannel(c *gin.Context) {
	h.proxyKookPOST(c, "/channel/update", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminDeleteKookChannel(c *gin.Context) {
	h.proxyKookPOST(c, "/channel/delete", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminListKookChannelUsers(c *gin.Context) {
	h.proxyKookGET(c, "/channel/user-list", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminMoveKookChannelUsers(c *gin.Context) {
	h.proxyKookPOST(c, "/channel/move-user", gin.H{"target_id": c.Param("channelId")})
}

func (h *Handler) AdminKickoutKookChannelUser(c *gin.Context) {
	h.proxyKookPOST(c, "/channel/kickout", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminGetKookChannelRoles(c *gin.Context) {
	data, okData := h.kookProxyData(c, http.MethodGet, "/channel-role/index", kookGETParams(c, gin.H{"channel_id": c.Param("channelId")}))
	if !okData {
		return
	}
	h.fillKookChannelRoleUserNames(data)
	ok(c, "success", data)
}

func (h *Handler) AdminCreateKookChannelRole(c *gin.Context) {
	h.proxyKookPOST(c, "/channel-role/create", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminUpdateKookChannelRole(c *gin.Context) {
	h.proxyKookPOST(c, "/channel-role/update", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminSyncKookChannelRoles(c *gin.Context) {
	h.proxyKookPOST(c, "/channel-role/sync", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminDeleteKookChannelRole(c *gin.Context) {
	h.proxyKookPOST(c, "/channel-role/delete", gin.H{"channel_id": c.Param("channelId")})
}

func (h *Handler) AdminListKookGuildRoles(c *gin.Context) {
	h.proxyKookGET(c, "/guild-role/list", gin.H{"guild_id": h.kookGuildID(), "page_size": 100})
}

func (h *Handler) AdminCreateKookGuildRole(c *gin.Context) {
	h.proxyKookPOST(c, "/guild-role/create", gin.H{"guild_id": h.kookGuildID()})
}

func (h *Handler) AdminUpdateKookGuildRole(c *gin.Context) {
	h.proxyKookPOST(c, "/guild-role/update", gin.H{"guild_id": h.kookGuildID(), "role_id": c.Param("roleId")})
}

func (h *Handler) AdminDeleteKookGuildRole(c *gin.Context) {
	h.proxyKookPOST(c, "/guild-role/delete", gin.H{"guild_id": h.kookGuildID(), "role_id": c.Param("roleId")})
}

func (h *Handler) AdminGrantKookGuildRole(c *gin.Context) {
	h.proxyKookPOST(c, "/guild-role/grant", gin.H{"guild_id": h.kookGuildID(), "role_id": c.Param("roleId")})
}

func (h *Handler) AdminRevokeKookGuildRole(c *gin.Context) {
	h.proxyKookPOST(c, "/guild-role/revoke", gin.H{"guild_id": h.kookGuildID(), "role_id": c.Param("roleId")})
}

func (h *Handler) AdminGetKookUserMe(c *gin.Context) {
	h.proxyKookGET(c, "/user/me", gin.H{})
}

func (h *Handler) AdminGetKookUser(c *gin.Context) {
	h.proxyKookGET(c, "/user/view", gin.H{"guild_id": h.kookGuildID(), "user_id": c.Param("userId")})
}

func (h *Handler) AdminOfflineKookBot(c *gin.Context) {
	h.proxyKookPOST(c, "/user/offline", gin.H{})
}

func (h *Handler) AdminOnlineKookBot(c *gin.Context) {
	h.proxyKookPOST(c, "/user/online", gin.H{})
}

func (h *Handler) AdminGetKookBotOnlineStatus(c *gin.Context) {
	h.proxyKookGET(c, "/user/get-online-status", gin.H{})
}

func (h *Handler) proxyKookGET(c *gin.Context, path string, defaults gin.H) {
	h.sendKookProxy(c, http.MethodGet, path, kookGETParams(c, defaults))
}

func kookGETParams(c *gin.Context, defaults gin.H) gin.H {
	params := gin.H{}
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			params[toKookParamName(key)] = values[0]
		}
	}
	for key, value := range defaults {
		if _, ok := params[key]; !ok && value != "" {
			params[key] = value
		}
	}
	return params
}

func (h *Handler) proxyKookPOST(c *gin.Context, path string, defaults gin.H) {
	body := gin.H{}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
			return
		}
	}
	body = toKookBody(body)
	for key, value := range defaults {
		if _, ok := body[key]; !ok && value != "" {
			body[key] = value
		}
	}
	h.sendKookProxy(c, http.MethodPost, path, body)
}

func (h *Handler) sendKookProxy(c *gin.Context, method string, path string, payload gin.H) {
	data, okData := h.kookProxyData(c, method, path, payload)
	if !okData {
		return
	}
	ok(c, "success", data)
}

func (h *Handler) kookProxyData(c *gin.Context, method string, path string, payload gin.H) (interface{}, bool) {
	token := h.kookBotToken()
	if token == "" {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, "KOOK Bot Token is not configured")
		return nil, false
	}

	var result kookProxyResponse
	req := resty.New().R().
		SetHeader("Authorization", "Bot "+token).
		SetHeader("Content-Type", "application/json").
		SetResult(&result)
	var resp *resty.Response
	var err error
	if method == http.MethodGet {
		resp, err = req.SetQueryParams(kookStringParams(payload)).Get(kookAPIBaseURL + path)
	} else {
		resp, err = req.SetBody(payload).Post(kookAPIBaseURL + path)
	}
	if err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, err.Error())
		return nil, false
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, kookProxyErrorMessage(resp.StatusCode(), result))
		return nil, false
	}
	return result.Data, true
}

func (h *Handler) fillKookChannelRoleUserNames(data interface{}) {
	rows := kookRoleRows(data)
	userIDs := make([]string, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		userID := kookRoleUserID(row)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	if len(userIDs) == 0 {
		return
	}

	var members []model.KookMember
	if err := h.db.Where("guild_id = ? AND kook_user_id IN ?", h.kookGuildID(), userIDs).Find(&members).Error; err != nil {
		return
	}
	names := make(map[string]string, len(members))
	for _, member := range members {
		names[member.KookUserID] = firstNonEmpty(stringValue(member.Nickname), stringValue(member.Username), member.KookUserID)
	}
	for _, row := range rows {
		userID := kookRoleUserID(row)
		name := firstNonEmpty(names[userID], kookRoleInlineUserName(row))
		if name == "" {
			continue
		}
		row["displayName"] = name
		row["objectName"] = name
		if user, ok := row["user"].(map[string]interface{}); ok {
			user["displayName"] = name
		}
	}
}

func kookRoleRows(data interface{}) []map[string]interface{} {
	root, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	var rows []map[string]interface{}
	for _, key := range []string{"permission_users", "permission_overwrites"} {
		items, _ := root[key].([]interface{})
		for _, item := range items {
			if row, ok := item.(map[string]interface{}); ok {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func kookRoleUserID(row map[string]interface{}) string {
	if value := stringFromAny(row["user_id"]); value != "" {
		return value
	}
	if user, ok := row["user"].(map[string]interface{}); ok {
		return stringFromAny(user["id"])
	}
	return ""
}

func kookRoleInlineUserName(row map[string]interface{}) string {
	if user, ok := row["user"].(map[string]interface{}); ok {
		return firstNonEmpty(stringFromAny(user["nickname"]), stringFromAny(user["username"]), stringFromAny(user["name"]))
	}
	return firstNonEmpty(stringFromAny(row["nickname"]), stringFromAny(row["username"]), stringFromAny(row["name"]))
}

func toKookBody(body gin.H) gin.H {
	next := gin.H{}
	for key, value := range body {
		next[toKookParamName(key)] = value
	}
	return next
}

func toKookParamName(key string) string {
	switch key {
	case "guildId":
		return "guild_id"
	case "pageSize":
		return "page_size"
	case "parentId":
		return "parent_id"
	case "channelId":
		return "channel_id"
	case "targetId":
		return "target_id"
	case "needChildren":
		return "need_children"
	case "limitAmount":
		return "limit_amount"
	case "isCategory":
		return "is_category"
	case "voiceQuality":
		return "voice_quality"
	case "slowMode":
		return "slow_mode"
	case "userId":
		return "user_id"
	case "userIds":
		return "user_ids"
	case "roleId":
		return "role_id"
	default:
		return key
	}
}

func kookStringParams(payload gin.H) map[string]string {
	params := make(map[string]string, len(payload))
	for key, value := range payload {
		params[key] = fmt.Sprint(value)
	}
	return params
}

func kookProxyErrorMessage(status int, result kookProxyResponse) string {
	if result.Message != "" {
		return result.Message
	}
	return fmt.Sprintf("kook api failed: http=%d code=%d", status, result.Code)
}
