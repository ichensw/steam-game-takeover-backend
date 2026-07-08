package httpapi

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errKookMemberDuplicate = errors.New("kook member already exists")
	errKookMemberInvalid   = errors.New("kook member invalid")
)

type kookMemberInput struct {
	GuildID      string  `json:"guildId"`
	KookUserID   string  `json:"kookUserId"`
	Username     string  `json:"username"`
	Nickname     string  `json:"nickname"`
	IdentifyNum  string  `json:"identifyNum"`
	AvatarURL    string  `json:"avatarUrl"`
	IsBot        bool    `json:"isBot"`
	MemberStatus uint8   `json:"memberStatus"`
	JoinedAt     *string `json:"joinedAt"`
	ExitedAt     *string `json:"exitedAt"`
	Remark       string  `json:"remark"`
}

type kookMemberDTO struct {
	ID              uint64  `json:"id"`
	GuildID         string  `json:"guildId"`
	KookUserID      string  `json:"kookUserId"`
	Username        string  `json:"username"`
	Nickname        string  `json:"nickname"`
	IdentifyNum     string  `json:"identifyNum"`
	AvatarURL       string  `json:"avatarUrl"`
	IsBot           bool    `json:"isBot"`
	MemberStatus    uint8   `json:"memberStatus"`
	JoinedAt        *string `json:"joinedAt"`
	ExitedAt        *string `json:"exitedAt"`
	IsBlacklisted   bool    `json:"isBlacklisted"`
	BlacklistReason *string `json:"blacklistReason"`
	BlacklistedAt   *string `json:"blacklistedAt"`
	Remark          *string `json:"remark"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type kookMemberListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []kookMemberAPIItem `json:"items"`
		Meta  struct {
			Page      int `json:"page"`
			PageTotal int `json:"page_total"`
			PageSize  int `json:"page_size"`
			Total     int `json:"total"`
		} `json:"meta"`
	} `json:"data"`
}

type kookMemberAPIItem struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	KookUserID  string      `json:"kook_user_id"`
	Username    string      `json:"username"`
	Nickname    string      `json:"nickname"`
	IdentifyNum string      `json:"identify_num"`
	Avatar      string      `json:"avatar"`
	AvatarURL   string      `json:"avatar_url"`
	IsBot       bool        `json:"is_bot"`
	Bot         bool        `json:"bot"`
	JoinedAt    interface{} `json:"joined_at"`
}

type kookAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type kookAPIError struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (e kookAPIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("kook api failed: http=%d code=%d", e.HTTPStatus, e.Code)
}

func (h *Handler) AdminListKookMembers(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(firstNonEmpty(c.Query("page_size"), c.Query("pageSize")), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	query := h.db.Model(&model.KookMember{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("kook_user_id LIKE ? OR username LIKE ? OR nickname LIKE ? OR identify_num LIKE ?", like, like, like, like)
	}
	if raw := strings.TrimSpace(c.Query("memberStatus")); raw != "" {
		status, err := parseKookMemberStatus(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
			return
		}
		query = query.Where("member_status = ?", status)
	}
	if raw := strings.TrimSpace(c.Query("isBlacklisted")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, "isBlacklisted invalid")
			return
		}
		query = query.Where("is_blacklisted = ?", value)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	var rows []model.KookMember
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	list := make([]kookMemberDTO, 0, len(rows))
	for _, row := range rows {
		list = append(list, toKookMemberDTO(row))
	}
	ok(c, "success", gin.H{"list": list, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) AdminGetKookMember(c *gin.Context) {
	member, okMember := h.adminKookMember(c)
	if !okMember {
		return
	}
	ok(c, "success", toKookMemberDTO(member))
}

func (h *Handler) AdminCreateKookMember(c *gin.Context) {
	var req kookMemberInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	member, err := normalizeKookMemberInput(req, true, model.KookMemberStatusJoined)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	var count int64
	if err := h.db.Model(&model.KookMember{}).Where("guild_id = ? AND kook_user_id = ?", member.GuildID, member.KookUserID).Count(&count).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if count > 0 {
		fail(c, http.StatusConflict, CodeParamInvalid, errKookMemberDuplicate.Error())
		return
	}
	if err := h.db.Create(&member).Error; err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			fail(c, http.StatusConflict, CodeParamInvalid, errKookMemberDuplicate.Error())
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "created", toKookMemberDTO(member))
}

func (h *Handler) AdminUpdateKookMember(c *gin.Context) {
	member, okMember := h.adminKookMember(c)
	if !okMember {
		return
	}
	var req kookMemberInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	parsed, err := normalizeKookMemberInput(req, false, member.MemberStatus)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	updates := map[string]interface{}{
		"username":      parsed.Username,
		"nickname":      parsed.Nickname,
		"identify_num":  parsed.IdentifyNum,
		"avatar_url":    parsed.AvatarURL,
		"is_bot":        parsed.IsBot,
		"member_status": parsed.MemberStatus,
		"joined_at":     parsed.JoinedAt,
		"exited_at":     parsed.ExitedAt,
		"remark":        parsed.Remark,
	}
	if err := h.db.Model(&model.KookMember{}).Where("id = ?", member.ID).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if err := h.db.Where("id = ?", member.ID).First(&member).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "saved", toKookMemberDTO(member))
}

func (h *Handler) AdminDeleteKookMember(c *gin.Context) {
	member, okMember := h.adminKookMember(c)
	if !okMember {
		return
	}
	if err := h.db.Delete(&member).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "delete failed")
		return
	}
	ok(c, "deleted", nil)
}

func (h *Handler) AdminSyncKookMembers(c *gin.Context) {
	token := h.kookBotToken()
	guildID := h.kookGuildID()
	if token == "" || guildID == "" {
		fail(c, http.StatusBadGateway, CodeSystemError, "kook not configured")
		return
	}

	client := resty.New()
	count := 0
	for page := 1; ; page++ {
		result, err := fetchKookMemberPage(client, token, guildID, page)
		if err != nil {
			fail(c, http.StatusBadGateway, CodeSystemError, "kook member sync failed")
			return
		}
		for _, item := range result.Data.Items {
			member := kookMemberFromAPI(guildID, item)
			if member.KookUserID == "" {
				continue
			}
			updates := kookMemberPartialUpdates(member)
			updates["member_status"] = model.KookMemberStatusJoined
			updates["exited_at"] = nil
			if member.JoinedAt != nil {
				updates["joined_at"] = member.JoinedAt
			}
			if err := upsertKookMember(h.db, member, updates); err != nil {
				fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
				return
			}
			count++
		}
		if result.Data.Meta.PageTotal <= 0 || result.Data.Meta.Page >= result.Data.Meta.PageTotal {
			break
		}
	}
	ok(c, "success", gin.H{"count": count})
}

func (h *Handler) AdminBlacklistKookMember(c *gin.Context) {
	member, okMember := h.adminKookMember(c)
	if !okMember {
		return
	}
	var req struct {
		Reason     string `json:"reason"`
		DelMsgDays int    `json:"delMsgDays"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 255 || req.DelMsgDays < 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "blacklist request invalid")
		return
	}
	if err := h.createKookBlacklist(member.GuildID, member.KookUserID, reason, req.DelMsgDays); err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, kookAdminErrorMessage("拉黑", err))
		return
	}
	now := time.Now()
	if err := h.db.Model(&model.KookMember{}).Where("id = ?", member.ID).Updates(map[string]interface{}{
		"is_blacklisted":   true,
		"blacklist_reason": nullableString(reason),
		"blacklisted_at":   now,
	}).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	_ = h.writeAdminLog("KOOK_MEMBER_BLACKLIST", "kook_member", member.ID, nullableString(reason))
	ok(c, "success", nil)
}

func (h *Handler) AdminUnblacklistKookMember(c *gin.Context) {
	member, okMember := h.adminKookMember(c)
	if !okMember {
		return
	}
	if err := h.deleteKookBlacklist(member.GuildID, member.KookUserID); err != nil {
		fail(c, http.StatusBadGateway, CodeKookOperationFailed, kookAdminErrorMessage("取消拉黑", err))
		return
	}
	if err := h.db.Model(&model.KookMember{}).Where("id = ?", member.ID).Updates(map[string]interface{}{
		"is_blacklisted":   false,
		"blacklist_reason": nil,
		"blacklisted_at":   nil,
	}).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	_ = h.writeAdminLog("KOOK_MEMBER_UNBLACKLIST", "kook_member", member.ID, nil)
	ok(c, "success", nil)
}

func (h *Handler) KookWebhook(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	encryptedLen := len(kookPayloadString(payload, "encrypt"))
	payload, err := h.decodeKookWebhookPayload(payload)
	if err != nil {
		log.Printf("kook webhook decode failed: encrypt_len=%d err=%v", encryptedLen, err)
		fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	if challenge := kookPayloadString(payload, "challenge"); challenge != "" {
		c.JSON(http.StatusOK, gin.H{"challenge": challenge})
		return
	}
	if verifyToken := h.kookVerifyToken(); verifyToken == "" || kookPayloadString(payload, "verify_token") != verifyToken {
		log.Printf("kook webhook unauthorized: encrypt_len=%d event=%q challenge=%t verify_len=%d saved_verify_len=%d", encryptedLen, kookEventType(payload), kookPayloadString(payload, "challenge") != "", len(kookPayloadString(payload, "verify_token")), len(verifyToken))
		fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	eventType := kookEventType(payload)
	member := kookMemberFromWebhook(payload)
	switch eventType {
	case "joined_guild":
		if member.GuildID == "" || member.KookUserID == "" {
			fail(c, http.StatusBadRequest, CodeParamInvalid, errKookMemberInvalid.Error())
			return
		}
		member.MemberStatus = model.KookMemberStatusJoined
		updates := kookWebhookMemberUpdates(payload, member)
		updates["member_status"] = model.KookMemberStatusJoined
		updates["exited_at"] = nil
		if member.JoinedAt != nil {
			updates["joined_at"] = member.JoinedAt
		}
		if err := upsertKookMember(h.db, member, updates); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	case "exited_guild":
		if member.GuildID == "" || member.KookUserID == "" {
			fail(c, http.StatusBadRequest, CodeParamInvalid, errKookMemberInvalid.Error())
			return
		}
		member.MemberStatus = model.KookMemberStatusExited
		updates := map[string]interface{}{"member_status": model.KookMemberStatusExited}
		if member.ExitedAt != nil {
			updates["exited_at"] = member.ExitedAt
		}
		if err := upsertKookMember(h.db, member, updates); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	case "updated_guild_member":
		if member.GuildID == "" || member.KookUserID == "" {
			fail(c, http.StatusBadRequest, CodeParamInvalid, errKookMemberInvalid.Error())
			return
		}
		member.MemberStatus = model.KookMemberStatusJoined
		if err := upsertKookMember(h.db, member, kookWebhookMemberUpdates(payload, member)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	ok(c, "success", nil)
}

func (h *Handler) decodeKookWebhookPayload(payload map[string]interface{}) (map[string]interface{}, error) {
	encrypted := kookPayloadString(payload, "encrypt")
	if encrypted == "" {
		return payload, nil
	}
	plain, err := decryptKookPayload(encrypted, h.kookEncryptKey())
	if err != nil {
		return nil, err
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(plain, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func (h *Handler) adminKookMember(c *gin.Context) (model.KookMember, bool) {
	memberID, okID := pathUint64(c, "kookMemberId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid kook member id")
		return model.KookMember{}, false
	}
	var member model.KookMember
	if err := h.db.Where("id = ?", memberID).First(&member).Error; err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "kook member not found")
			return model.KookMember{}, false
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return model.KookMember{}, false
	}
	return member, true
}

func normalizeKookMemberInput(req kookMemberInput, requireIDs bool, defaultStatus uint8) (model.KookMember, error) {
	guildID := strings.TrimSpace(req.GuildID)
	kookUserID := strings.TrimSpace(req.KookUserID)
	if requireIDs && (guildID == "" || kookUserID == "") {
		return model.KookMember{}, errKookMemberInvalid
	}
	if len([]rune(guildID)) > 64 || len([]rune(kookUserID)) > 64 {
		return model.KookMember{}, errKookMemberInvalid
	}
	status := req.MemberStatus
	if status == 0 {
		status = defaultStatus
	}
	if status != model.KookMemberStatusJoined && status != model.KookMemberStatusExited {
		return model.KookMember{}, errKookMemberInvalid
	}
	joinedAt, err := parseKookOptionalTime(req.JoinedAt)
	if err != nil {
		return model.KookMember{}, err
	}
	exitedAt, err := parseKookOptionalTime(req.ExitedAt)
	if err != nil {
		return model.KookMember{}, err
	}
	username := strings.TrimSpace(req.Username)
	nickname := strings.TrimSpace(req.Nickname)
	identifyNum := strings.TrimSpace(req.IdentifyNum)
	avatarURL := strings.TrimSpace(req.AvatarURL)
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(username)) > 64 || len([]rune(nickname)) > 64 || len([]rune(identifyNum)) > 16 || len([]rune(avatarURL)) > 255 || len([]rune(remark)) > 255 {
		return model.KookMember{}, errKookMemberInvalid
	}
	return model.KookMember{
		GuildID:      guildID,
		KookUserID:   kookUserID,
		Username:     nullableString(username),
		Nickname:     nullableString(nickname),
		IdentifyNum:  nullableString(identifyNum),
		AvatarURL:    nullableString(avatarURL),
		IsBot:        req.IsBot,
		MemberStatus: status,
		JoinedAt:     joinedAt,
		ExitedAt:     exitedAt,
		Remark:       nullableString(remark),
	}, nil
}

func parseKookMemberStatus(raw string) (uint8, error) {
	value, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		return 0, errKookMemberInvalid
	}
	status := uint8(value)
	if status != model.KookMemberStatusJoined && status != model.KookMemberStatusExited {
		return 0, errKookMemberInvalid
	}
	return status, nil
}

func parseKookOptionalTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	return parseOptionalDateTime(*value)
}

func parseKookTimeValue(value interface{}) *time.Time {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil
		}
		if parsed, err := parseOptionalDateTime(text); err == nil {
			return parsed
		}
		if number, err := strconv.ParseInt(text, 10, 64); err == nil {
			return unixKookTime(number)
		}
	case float64:
		return unixKookTime(int64(typed))
	case json.Number:
		if number, err := typed.Int64(); err == nil {
			return unixKookTime(number)
		}
	}
	return nil
}

func unixKookTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	if value > 1_000_000_000_000 {
		value /= 1000
	}
	parsed := time.Unix(value, 0)
	return &parsed
}

func toKookMemberDTO(member model.KookMember) kookMemberDTO {
	return kookMemberDTO{
		ID:              member.ID,
		GuildID:         member.GuildID,
		KookUserID:      member.KookUserID,
		Username:        stringValue(member.Username),
		Nickname:        stringValue(member.Nickname),
		IdentifyNum:     stringValue(member.IdentifyNum),
		AvatarURL:       stringValue(member.AvatarURL),
		IsBot:           member.IsBot,
		MemberStatus:    member.MemberStatus,
		JoinedAt:        kookTimeString(member.JoinedAt),
		ExitedAt:        kookTimeString(member.ExitedAt),
		IsBlacklisted:   member.IsBlacklisted,
		BlacklistReason: member.BlacklistReason,
		BlacklistedAt:   kookTimeString(member.BlacklistedAt),
		Remark:          member.Remark,
		CreatedAt:       member.GmtCreate.Format("2006-01-02 15:04:05"),
		UpdatedAt:       member.GmtModified.Format("2006-01-02 15:04:05"),
	}
}

func kookTimeString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.Format("2006-01-02 15:04:05")
	return &text
}

func fetchKookMemberPage(client *resty.Client, token, guildID string, page int) (kookMemberListResponse, error) {
	var result kookMemberListResponse
	resp, err := client.R().
		SetHeader("Authorization", "Bot "+token).
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"guild_id":  guildID,
			"page":      fmt.Sprintf("%d", page),
			"page_size": "50",
		}).
		SetResult(&result).
		Get("https://www.kookapp.cn/api/v3/guild/user-list")
	if err != nil {
		return result, err
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		return result, fmt.Errorf("kook member list failed: http=%d code=%d", resp.StatusCode(), result.Code)
	}
	return result, nil
}

func kookMemberFromAPI(guildID string, item kookMemberAPIItem) model.KookMember {
	return model.KookMember{
		GuildID:      guildID,
		KookUserID:   firstNonEmpty(item.KookUserID, item.UserID, item.ID),
		Username:     nullableString(strings.TrimSpace(item.Username)),
		Nickname:     nullableString(strings.TrimSpace(item.Nickname)),
		IdentifyNum:  nullableString(strings.TrimSpace(item.IdentifyNum)),
		AvatarURL:    nullableString(strings.TrimSpace(firstNonEmpty(item.AvatarURL, item.Avatar))),
		IsBot:        item.IsBot || item.Bot,
		MemberStatus: model.KookMemberStatusJoined,
		JoinedAt:     parseKookTimeValue(item.JoinedAt),
	}
}

func upsertKookMember(db *gorm.DB, member model.KookMember, updates map[string]interface{}) error {
	onConflict := clause.OnConflict{
		Columns: []clause.Column{{Name: "guild_id"}, {Name: "kook_user_id"}},
	}
	if len(updates) == 0 {
		onConflict.DoNothing = true
	} else {
		onConflict.DoUpdates = clause.Assignments(updates)
	}
	return db.Clauses(onConflict).Create(&member).Error
}

func kookMemberPartialUpdates(member model.KookMember) map[string]interface{} {
	updates := map[string]interface{}{"is_bot": member.IsBot}
	if member.Username != nil {
		updates["username"] = member.Username
	}
	if member.Nickname != nil {
		updates["nickname"] = member.Nickname
	}
	if member.IdentifyNum != nil {
		updates["identify_num"] = member.IdentifyNum
	}
	if member.AvatarURL != nil {
		updates["avatar_url"] = member.AvatarURL
	}
	return updates
}

func kookWebhookMemberUpdates(payload map[string]interface{}, member model.KookMember) map[string]interface{} {
	updates := kookMemberPartialUpdates(member)
	if !kookPayloadHas(payload, "is_bot", "isBot", "bot") {
		delete(updates, "is_bot")
	}
	return updates
}

func (h *Handler) createKookBlacklist(guildID, targetID, remark string, delMsgDays int) error {
	return h.postKookBlacklist("https://www.kookapp.cn/api/v3/blacklist/create", gin.H{
		"guild_id":     guildID,
		"target_id":    targetID,
		"remark":       remark,
		"del_msg_days": delMsgDays,
	})
}

func (h *Handler) deleteKookBlacklist(guildID, targetID string) error {
	return h.postKookBlacklist("https://www.kookapp.cn/api/v3/blacklist/delete", gin.H{
		"guild_id":  guildID,
		"target_id": targetID,
	})
}

func (h *Handler) postKookBlacklist(url string, body gin.H) error {
	token := h.kookBotToken()
	if token == "" {
		return fmt.Errorf("kook not configured")
	}
	var result kookAPIResponse
	resp, err := resty.New().R().
		SetHeader("Authorization", "Bot "+token).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetResult(&result).
		Post(url)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		return kookAPIError{
			HTTPStatus: resp.StatusCode(),
			Code:       result.Code,
			Message:    strings.TrimSpace(result.Message),
		}
	}
	return nil
}

func kookAdminErrorMessage(action string, err error) string {
	var apiErr kookAPIError
	if errors.As(err, &apiErr) {
		message := strings.TrimSpace(apiErr.Message)
		if strings.Contains(message, "target_id") || strings.Contains(message, "没有权限") {
			return fmt.Sprintf("KOOK %s失败：用户不存在，或机器人没有权限操作该用户。请检查机器人是否有封禁用户权限，并确认机器人角色高于目标用户。", action)
		}
		if message != "" {
			return fmt.Sprintf("KOOK %s失败：%s", action, message)
		}
	}
	if strings.Contains(err.Error(), "not configured") {
		return "KOOK 机器人未配置，无法执行操作"
	}
	return fmt.Sprintf("KOOK %s失败，请稍后再试", action)
}

func kookMemberFromWebhook(payload map[string]interface{}) model.KookMember {
	return model.KookMember{
		GuildID:     kookPayloadGuildID(payload),
		KookUserID:  kookPayloadUserID(payload),
		Username:    nullableString(kookPayloadString(payload, "username")),
		Nickname:    nullableString(kookPayloadString(payload, "nickname")),
		IdentifyNum: nullableString(kookPayloadString(payload, "identify_num", "identifyNum")),
		AvatarURL:   nullableString(kookPayloadString(payload, "avatar_url", "avatarUrl", "avatar")),
		IsBot:       kookPayloadBool(payload, "is_bot", "isBot", "bot"),
		JoinedAt:    parseKookTimeValue(kookPayloadValue(payload, "joined_at", "joinedAt")),
		ExitedAt:    parseKookTimeValue(kookPayloadValue(payload, "exited_at", "exitedAt")),
	}
}

func kookEventType(payload map[string]interface{}) string {
	for _, item := range kookPayloadMaps(payload) {
		switch value := stringFromAny(item["type"]); value {
		case "joined_guild", "exited_guild", "updated_guild_member":
			return value
		}
	}
	return kookPayloadString(payload, "type")
}

func kookPayloadGuildID(payload map[string]interface{}) string {
	if value := kookPayloadString(payload, "guild_id", "guildId"); value != "" {
		return value
	}
	switch kookEventType(payload) {
	case "joined_guild", "exited_guild", "updated_guild_member":
		return kookPayloadString(payload, "target_id", "targetId")
	default:
		return ""
	}
}

func kookPayloadUserID(payload map[string]interface{}) string {
	if value := kookPayloadString(payload, "kook_user_id", "kookUserId", "user_id", "userId"); value != "" {
		return value
	}
	for _, parent := range kookPayloadMaps(payload) {
		for _, key := range []string{"user", "member", "author"} {
			if child, ok := parent[key].(map[string]interface{}); ok {
				if value := stringFromAny(child["id"]); value != "" {
					return value
				}
			}
		}
	}
	if value := kookPayloadString(payload, "author_id", "authorId", "target_id", "targetId"); value != "" {
		return value
	}
	return ""
}

func kookPayloadString(payload map[string]interface{}, keys ...string) string {
	for _, item := range kookPayloadMaps(payload) {
		for _, key := range keys {
			if value := stringFromAny(item[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func kookPayloadBool(payload map[string]interface{}, keys ...string) bool {
	for _, item := range kookPayloadMaps(payload) {
		for _, key := range keys {
			switch value := item[key].(type) {
			case bool:
				return value
			case string:
				if value == "1" {
					return true
				}
				parsed, _ := strconv.ParseBool(value)
				return parsed
			case float64:
				return value != 0
			case json.Number:
				number, _ := value.Int64()
				return number != 0
			}
		}
	}
	return false
}

func kookPayloadValue(payload map[string]interface{}, keys ...string) interface{} {
	for _, item := range kookPayloadMaps(payload) {
		for _, key := range keys {
			if value, ok := item[key]; ok {
				return value
			}
		}
	}
	return nil
}

func kookPayloadHas(payload map[string]interface{}, keys ...string) bool {
	for _, item := range kookPayloadMaps(payload) {
		for _, key := range keys {
			if _, ok := item[key]; ok {
				return true
			}
		}
	}
	return false
}

func kookPayloadMaps(payload map[string]interface{}) []map[string]interface{} {
	maps := []map[string]interface{}{payload}
	for i := 0; i < len(maps); i++ {
		parent := maps[i]
		for _, key := range []string{"d", "extra", "body", "user", "member", "author"} {
			if child, ok := parent[key].(map[string]interface{}); ok {
				maps = append(maps, child)
			}
		}
	}
	return maps
}

func decryptKookPayload(encrypted string, encryptKey string) ([]byte, error) {
	keyText := strings.TrimSpace(encryptKey)
	if keyText == "" {
		return nil, errors.New("kook encrypt key is required")
	}
	key := make([]byte, 32)
	copy(key, []byte(keyText))

	outer, err := decodeKookBase64(encrypted)
	if err != nil {
		return nil, err
	}
	if len(outer) <= aes.BlockSize {
		return nil, errors.New("kook encrypted payload invalid")
	}
	iv := outer[:aes.BlockSize]
	cipherText, err := decodeKookBase64(string(outer[aes.BlockSize:]))
	if err != nil {
		return nil, err
	}
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("kook encrypted payload block size invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, cipherText)
	if unpadded, err := pkcs7Unpad(plain, aes.BlockSize); err == nil {
		return unpadded, nil
	}
	return bytes.TrimRight(plain, "\x00"), nil
}

func decodeKookBase64(value string) ([]byte, error) {
	text := strings.TrimSpace(value)
	var lastErr error
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(text)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func pkcs7Unpad(value []byte, blockSize int) ([]byte, error) {
	if len(value) == 0 {
		return nil, errors.New("padding invalid")
	}
	padding := int(value[len(value)-1])
	if padding == 0 || padding > blockSize || padding > len(value) {
		return nil, errors.New("padding invalid")
	}
	if !bytes.Equal(value[len(value)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
		return nil, errors.New("padding invalid")
	}
	return value[:len(value)-padding], nil
}

func stringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}
