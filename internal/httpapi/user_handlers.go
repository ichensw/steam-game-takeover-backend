package httpapi

import (
	"log"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
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

	if err := h.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"nickname":             nickname,
		"steam_id":             steamID,
		"gender":               req.Gender,
		"avatar_url":           stringPtr(avatarURL),
		"is_profile_completed": true,
	}).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "saved", gin.H{"profileCompleted": true})
}
