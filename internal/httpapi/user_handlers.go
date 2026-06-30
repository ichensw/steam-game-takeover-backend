package httpapi

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errNicknameTaken = errors.New("nickname already taken")
	errSteamIDTaken  = errors.New("steamId already taken")
)

func (h *Handler) Health(c *gin.Context) {
	ok(c, "ok", gin.H{"status": "ok"})
}

func (h *Handler) WXLogin(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "code is required")
		return
	}

	session, err := h.codeToSession(req.Code)
	if err != nil {
		log.Printf("wx-login codeToSession failed: %v", err)
		fail(c, http.StatusBadGateway, CodeSystemError, "wechat login failed")
		return
	}

	now := time.Now()
	unionID := stringPtr(session.UnionID)
	user, err := h.upsertActiveWXUser(session.OpenID, unionID, now)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}

	token, err := h.signUserToken(user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "token sign failed")
		return
	}
	ok(c, "success", gin.H{
		"token":                  token,
		"user":                   toUserDTO(user),
		"publishTakeoverEnabled": h.canPublishTakeover(user),
	})
}

func (h *Handler) GetProfile(c *gin.Context) {
	user, _ := currentUser(c)
	ok(c, "success", toUserDTO(user))
}

func (h *Handler) GetMeSummary(c *gin.Context) {
	user, _ := currentUser(c)

	var createdCount int64
	if err := h.db.Model(&model.Takeover{}).
		Where("creator_user_id = ? AND is_deleted = ?", user.ID, false).
		Count(&createdCount).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var joinedCount int64
	if err := h.db.Table("ttw_takeover_member AS m").
		Joins("JOIN ttw_takeover AS t ON t.id = m.takeover_id").
		Where("m.user_id = ? AND m.member_state = ? AND t.is_deleted = ? AND t.creator_user_id <> ?", user.ID, model.MemberStateJoined, false, user.ID).
		Count(&joinedCount).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var takeovers []model.Takeover
	if err := h.db.Table("ttw_takeover AS t").
		Select("DISTINCT t.*").
		Joins("LEFT JOIN ttw_takeover_member AS m ON m.takeover_id = t.id AND m.user_id = ? AND m.member_state = ?", user.ID, model.MemberStateJoined).
		Where("t.is_deleted = ? AND (t.creator_user_id = ? OR m.user_id IS NOT NULL)", false, user.ID).
		Order("t.gmt_create DESC").
		Limit(1).
		Scan(&takeovers).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	recent := make([]takeoverDTO, 0, len(takeovers))
	for _, takeover := range takeovers {
		joined, hasJoined, err := h.takeoverStats(takeover.ID, user.ID)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto := toTakeoverDTOWithCreator(h.db, takeover, joined, hasJoined)
		dto.IsCreator = isTakeoverCreator(user, takeover)
		members, err := h.takeoverMembers(takeover.ID, false, 5)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto.PreviewMembers = members
		recent = append(recent, dto)
	}

	ok(c, "success", gin.H{
		"user":         toUserDTO(user),
		"createdCount": createdCount,
		"joinedCount":  joinedCount,
		"recent":       recent,
	})
}

func (h *Handler) ListMyTakeovers(c *gin.Context) {
	user, _ := currentUser(c)
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 10)
	if pageSize > 50 {
		pageSize = 50
	}

	query := h.db.Table("ttw_takeover AS t").
		Where("t.is_deleted = ? AND (t.creator_user_id = ? OR EXISTS (SELECT 1 FROM ttw_takeover_member m WHERE m.takeover_id = t.id AND m.user_id = ? AND m.member_state = ?))", false, user.ID, user.ID, model.MemberStateJoined)
	query = applyMyTakeoverKeywordFilter(query, c.Query("keyword"))

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var takeovers []model.Takeover
	if err := query.Select("t.*").
		Order("t.gmt_create DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&takeovers).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]takeoverDTO, 0, len(takeovers))
	for _, takeover := range takeovers {
		joined, hasJoined, err := h.takeoverStats(takeover.ID, user.ID)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto := toTakeoverDTOWithCreator(h.db, takeover, joined, hasJoined)
		dto.IsCreator = isTakeoverCreator(user, takeover)
		members, err := h.takeoverMembers(takeover.ID, false, 5)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto.PreviewMembers = members
		list = append(list, dto)
	}

	ok(c, "success", gin.H{
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
		"list":     list,
	})
}

func applyMyTakeoverKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return query
	}
	like := "%" + keyword + "%"
	return query.Where(
		"t.title LIKE ? OR t.description LIKE ? OR EXISTS (SELECT 1 FROM ttw_user cu WHERE cu.id = t.creator_user_id AND cu.is_deleted = ? AND cu.nickname LIKE ?) OR EXISTS (SELECT 1 FROM ttw_takeover_member km JOIN ttw_user ku ON ku.id = km.user_id WHERE km.takeover_id = t.id AND km.member_state = ? AND ku.is_deleted = ? AND ku.nickname LIKE ?)",
		like,
		like,
		false,
		like,
		model.MemberStateJoined,
		false,
		like,
	)
}

func (h *Handler) SaveProfile(c *gin.Context) {
	user, _ := currentUser(c)
	var req struct {
		Nickname  string `json:"nickname"`
		SteamID   string `json:"steamId"`
		Gender    uint8  `json:"gender"`
		AvatarURL string `json:"avatarUrl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}

	nickname := strings.TrimSpace(req.Nickname)
	steamID := strings.TrimSpace(req.SteamID)
	avatarURL := strings.TrimSpace(req.AvatarURL)
	if nicknameLen := len([]rune(nickname)); nicknameLen < 2 || nicknameLen > 12 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "nickname must be between 2 and 12 characters")
		return
	}
	if len([]rune(steamID)) > 64 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "steamId must be at most 64 characters")
		return
	}
	if steamID != "" && !isDigits(steamID) {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "steamId must contain digits only")
		return
	}
	if req.Gender != model.GenderMale && req.Gender != model.GenderFemale {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "gender must be 1 or 2")
		return
	}
	avatarURL = normalizeAvatarURLForGender(avatarURL, req.Gender)
	if len([]rune(avatarURL)) > 255 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "avatarUrl must be at most 255 characters")
		return
	}
	currentSteamID := normalizeSteamID64ToFriendCode(strings.TrimSpace(stringValue(user.SteamID)))
	if steamID != "" && currentSteamID != steamID {
		normalizedSteamID, err := h.validateSteamFriendCode(steamID)
		if err != nil {
			if errors.Is(err, errSteamFriendCodeInvalid) {
				fail(c, http.StatusBadRequest, CodeParamInvalid, "steam friend code invalid")
				return
			}
			fail(c, http.StatusBadGateway, CodeSystemError, "steam friend code check failed")
			return
		}
		steamID = normalizedSteamID
	}
	if err := h.checkTextSecurity(contentSecurityTarget{
		User:        user,
		ContentType: "profile",
		TargetID:    user.ID,
		Scene:       contentSceneProfile,
	}, nickname); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "content security reject")
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		taken, err := h.isNicknameTaken(tx, nickname, user.ID)
		if err != nil {
			return err
		}
		if taken {
			return errNicknameTaken
		}

		if steamID != "" {
			taken, err := h.isSteamIDTaken(tx, steamID, user.ID)
			if err != nil {
				return err
			}
			if taken {
				return errSteamIDTaken
			}
		}

		return tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
			"nickname":             nickname,
			"steam_id":             optionalStringPtr(steamID),
			"gender":               req.Gender,
			"avatar_url":           stringPtr(avatarURL),
			"is_profile_completed": true,
			"is_admin":             user.IsAdmin,
		}).Error
	}); err != nil {
		if errors.Is(err, errNicknameTaken) {
			fail(c, http.StatusConflict, CodeParamInvalid, errNicknameTaken.Error())
			return
		}
		if errors.Is(err, errSteamIDTaken) {
			fail(c, http.StatusConflict, CodeSteamIDTaken, errSteamIDTaken.Error())
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if err := h.db.Where("id = ? AND is_deleted = ?", user.ID, false).First(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "saved", toUserDTO(user))
}

func (h *Handler) isNicknameTaken(tx *gorm.DB, nickname string, currentUserID uint64) (bool, error) {
	var count int64
	err := tx.Model(&model.User{}).
		Where("nickname = ? AND id <> ? AND is_deleted = ?", nickname, currentUserID, false).
		Count(&count).Error
	return count > 0, err
}

func (h *Handler) isSteamIDTaken(tx *gorm.DB, steamID string, currentUserID uint64) (bool, error) {
	var count int64
	err := tx.Model(&model.User{}).
		Where("steam_id = ? AND id <> ? AND is_deleted = ?", steamID, currentUserID, false).
		Count(&count).Error
	return count > 0, err
}

func isDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

func (h *Handler) upsertActiveWXUser(openID string, unionID *string, now time.Time) (model.User, error) {
	var user model.User
	err := h.db.Where("openid = ? AND is_deleted = ?", openID, false).First(&user).Error
	if err != nil && !isNotFound(err) {
		return model.User{}, err
	}
	if user.ID == 0 {
		user = model.User{
			OpenID:        openID,
			UnionID:       unionID,
			LastLoginTime: &now,
		}
		return user, h.db.Create(&user).Error
	}
	err = h.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"unionid":         unionID,
		"last_login_time": now,
	}).Error
	if err != nil {
		return model.User{}, err
	}
	return user, h.db.Where("id = ? AND is_deleted = ?", user.ID, false).First(&user).Error
}
