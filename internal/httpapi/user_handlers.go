package httpapi

import (
	"crypto/sha1"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
		"publishTakeoverEnabled": h.publishTakeoverEnabled(),
	})
}

func (h *Handler) WebLogin(c *gin.Context) {
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
	if steamID == "" || len([]rune(steamID)) > 64 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "steamId is required and must be at most 64 characters")
		return
	}
	now := time.Now()
	if nickname == "" && req.Gender == 0 && avatarURL == "" {
		user, err := h.findUserBySteamID(steamID)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
			return
		}
		if user.ID == 0 {
			user = model.User{
				OpenID:        webOpenIDForSteamID(steamID),
				SteamID:       stringPtr(steamID),
				LastLoginTime: &now,
			}
			if err := h.db.Create(&user).Error; err != nil {
				fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
				return
			}
		} else if err := h.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
			"last_login_time": now,
		}).Error; err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
			return
		}
		if err := h.db.Where("id = ? AND is_deleted = ?", user.ID, false).First(&user).Error; err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
			return
		}
		h.respondWebLogin(c, user)
		return
	}

	if nickname == "" || len([]rune(nickname)) > 32 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "nickname is required and must be at most 32 characters")
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

	user, err := h.findUserBySteamID(steamID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}
	if user.ID == 0 {
		user = model.User{OpenID: webOpenIDForSteamID(steamID)}
	}
	user.Nickname = stringPtr(nickname)
	user.SteamID = stringPtr(steamID)
	user.Gender = &req.Gender
	user.AvatarURL = stringPtr(avatarURL)
	user.IsProfileCompleted = true
	user.LastLoginTime = &now

	if user.ID == 0 {
		if err := h.db.Create(&user).Error; err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
			return
		}
	} else if err := h.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"nickname":             nickname,
		"steam_id":             steamID,
		"gender":               req.Gender,
		"avatar_url":           stringPtr(avatarURL),
		"is_profile_completed": true,
		"last_login_time":      now,
	}).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}
	if err := h.db.Where("id = ? AND is_deleted = ?", user.ID, false).First(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}
	h.respondWebLogin(c, user)
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
	if nickname == "" || len([]rune(nickname)) > 32 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "nickname is required and must be at most 32 characters")
		return
	}
	if steamID == "" || len([]rune(steamID)) > 64 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "steamId is required and must be at most 64 characters")
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
	if user.SteamID != nil && *user.SteamID != "" && *user.SteamID != steamID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "steamId cannot be changed")
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		var linked model.User
		err := tx.Where("steam_id = ? AND id <> ? AND openid LIKE ? AND is_deleted = ?", steamID, user.ID, "web_%", false).Order("id ASC").First(&linked).Error
		if err != nil && !isNotFound(err) {
			return err
		}
		if linked.ID != 0 {
			if err := h.mergeUsersBySteamID(tx, linked, user); err != nil {
				return err
			}
			if linked.IsBlocked {
				user.IsBlocked = true
			}
			if linked.IsAdmin {
				user.IsAdmin = true
			}
		}
		return tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
			"nickname":             nickname,
			"steam_id":             steamID,
			"gender":               req.Gender,
			"avatar_url":           stringPtr(avatarURL),
			"is_profile_completed": true,
			"is_blocked":           user.IsBlocked,
			"is_admin":             user.IsAdmin,
		}).Error
	}); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if err := h.db.Where("id = ? AND is_deleted = ?", user.ID, false).First(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "saved", toUserDTO(user))
}

func (h *Handler) respondWebLogin(c *gin.Context, user model.User) {
	token, err := h.signUserToken(user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "token sign failed")
		return
	}
	ok(c, "success", gin.H{
		"token":                  token,
		"user":                   toUserDTO(user),
		"publishTakeoverEnabled": h.publishTakeoverEnabled(),
	})
}

func (h *Handler) findUserBySteamID(steamID string) (model.User, error) {
	var users []model.User
	if err := h.db.Where("steam_id = ? AND is_deleted = ?", steamID, false).Order("id ASC").Find(&users).Error; err != nil {
		return model.User{}, err
	}
	if len(users) == 0 {
		return model.User{}, nil
	}
	for _, user := range users {
		if !strings.HasPrefix(user.OpenID, "web_") {
			return user, nil
		}
	}
	return users[0], nil
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

func webOpenIDForSteamID(steamID string) string {
	return fmt.Sprintf("web_%x", sha1.Sum([]byte(steamID)))
}

func (h *Handler) mergeUsersBySteamID(tx *gorm.DB, from model.User, into model.User) error {
	var memberships []model.TakeoverMember
	if err := tx.Where("user_id = ?", from.ID).Find(&memberships).Error; err != nil {
		return err
	}
	for _, membership := range memberships {
		var existing model.TakeoverMember
		err := tx.Where("takeover_id = ? AND user_id = ?", membership.TakeoverID, into.ID).First(&existing).Error
		if err != nil && !isNotFound(err) {
			return err
		}
		if existing.ID != 0 {
			if existing.MemberState != model.MemberStateJoined && membership.MemberState == model.MemberStateJoined {
				if err := tx.Model(&model.TakeoverMember{}).Where("id = ?", existing.ID).Update("member_state", model.MemberStateJoined).Error; err != nil {
					return err
				}
			}
			if err := tx.Delete(&model.TakeoverMember{}, membership.ID).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Model(&model.TakeoverMember{}).Where("id = ?", membership.ID).Update("user_id", into.ID).Error; err != nil {
			return err
		}
	}

	if err := tx.Model(&model.Takeover{}).Where("creator_user_id = ?", from.ID).Update("creator_user_id", into.ID).Error; err != nil {
		return err
	}

	var block model.BlockUser
	err := tx.Where("user_id = ? AND is_deleted = 0", from.ID).First(&block).Error
	if err != nil && !isNotFound(err) {
		return err
	}
	if block.ID != 0 {
		var existingBlock model.BlockUser
		err := tx.Where("user_id = ? AND is_deleted = 0", into.ID).First(&existingBlock).Error
		if err != nil && !isNotFound(err) {
			return err
		}
		if existingBlock.ID != 0 {
			if err := tx.Model(&model.BlockUser{}).Where("id = ?", block.ID).Update("is_deleted", true).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&model.BlockUser{}).Where("id = ?", block.ID).Updates(map[string]interface{}{
			"user_id": into.ID,
			"openid":  into.OpenID,
		}).Error; err != nil {
			return err
		}
	}

	return tx.Delete(&model.User{}, from.ID).Error
}
