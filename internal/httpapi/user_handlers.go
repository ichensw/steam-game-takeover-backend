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
	"gorm.io/gorm/clause"
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
	user := model.User{
		OpenID:        session.OpenID,
		UnionID:       unionID,
		LastLoginTime: &now,
	}
	if err := h.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "openid"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"unionid":         unionID,
			"last_login_time": now,
			"gmt_modified":    now,
		}),
	}).Create(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}
	if err := h.db.Where("openid = ?", session.OpenID).First(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}

	token, err := h.signUserToken(user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "token sign failed")
		return
	}
	ok(c, "success", gin.H{"token": token, "user": toUserDTO(user)})
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
		if err := h.db.First(&user, user.ID).Error; err != nil {
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
	if err := h.db.First(&user, user.ID).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}
	h.respondWebLogin(c, user)
}

func (h *Handler) GetProfile(c *gin.Context) {
	user, _ := currentUser(c)
	ok(c, "success", toUserDTO(user))
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
	if len([]rune(avatarURL)) > 255 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "avatarUrl must be at most 255 characters")
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		var linked model.User
		err := tx.Where("steam_id = ? AND id <> ? AND openid LIKE ?", steamID, user.ID, "web_%").Order("id ASC").First(&linked).Error
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
		}
		return tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
			"nickname":             nickname,
			"steam_id":             steamID,
			"gender":               req.Gender,
			"avatar_url":           stringPtr(avatarURL),
			"is_profile_completed": true,
			"is_blocked":           user.IsBlocked,
		}).Error
	}); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if err := h.db.First(&user, user.ID).Error; err != nil {
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
	ok(c, "success", gin.H{"token": token, "user": toUserDTO(user)})
}

func (h *Handler) findUserBySteamID(steamID string) (model.User, error) {
	var users []model.User
	if err := h.db.Where("steam_id = ?", steamID).Order("id ASC").Find(&users).Error; err != nil {
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
