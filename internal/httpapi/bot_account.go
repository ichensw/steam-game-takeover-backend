package httpapi

import (
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/config"
	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const botQueryOpenID = "bot_query_default"

func EnsureBotQueryUser(db *gorm.DB, cfg config.Config) (model.User, error) {
	if !cfg.BotQueryEnabled {
		return model.User{}, nil
	}
	return ensureBotQueryUser(db, cfg)
}

func (h *Handler) BotLogin(c *gin.Context) {
	if !h.cfg.BotQueryEnabled {
		fail(c, http.StatusForbidden, CodeUnauthorized, "bot query account disabled")
		return
	}

	user, err := ensureBotQueryUser(h.db, h.cfg)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "bot login failed")
		return
	}

	token, err := h.signUserToken(user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "token sign failed")
		return
	}
	ok(c, "success", gin.H{"token": token, "user": toUserDTO(user)})
}

func ensureBotQueryUser(db *gorm.DB, cfg config.Config) (model.User, error) {
	nickname := strings.TrimSpace(cfg.BotQueryNickname)
	if nickname == "" {
		nickname = "WeChat Bot"
	}
	steamID := strings.TrimSpace(cfg.BotQuerySteamID)
	if steamID == "" {
		steamID = "wechat-bot-query"
	}
	gender := cfg.BotQueryGender
	if gender != model.GenderMale && gender != model.GenderFemale {
		gender = model.GenderMale
	}
	avatarURL := normalizeAvatarURLForGender(strings.TrimSpace(cfg.BotQueryAvatarURL), gender)
	now := time.Now()

	var user model.User
	err := db.Where("openid = ? AND is_deleted = ?", botQueryOpenID, false).First(&user).Error
	if err != nil && !isNotFound(err) {
		return model.User{}, err
	}
	if user.ID == 0 {
		user = model.User{
			OpenID:             botQueryOpenID,
			Nickname:           stringPtr(nickname),
			SteamID:            stringPtr(steamID),
			Gender:             &gender,
			AvatarURL:          stringPtr(avatarURL),
			IsProfileCompleted: false,
			IsBlocked:          false,
			IsDeleted:          false,
			LastLoginTime:      &now,
		}
		if err := db.Create(&user).Error; err != nil {
			return model.User{}, err
		}
		return user, nil
	}
	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"nickname":             stringPtr(nickname),
		"steam_id":             stringPtr(steamID),
		"gender":               gender,
		"avatar_url":           stringPtr(avatarURL),
		"is_profile_completed": false,
		"is_blocked":           false,
		"is_deleted":           false,
		"last_login_time":      now,
		"gmt_modified":         now,
	}).Error; err != nil {
		return model.User{}, err
	}
	if err := db.Where("id = ? AND is_deleted = ?", user.ID, false).First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}
