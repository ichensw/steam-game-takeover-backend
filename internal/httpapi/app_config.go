package httpapi

import (
	"net/http"
	"strings"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func (h *Handler) publishTakeoverEnabled() bool {
	var config model.AppConfig
	if err := h.db.Where("config_key = ?", model.AppConfigPublishTakeoverEnabled).First(&config).Error; err != nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(config.ConfigValue)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (h *Handler) canPublishTakeover(user model.User) bool {
	globalEnabled := h.publishTakeoverEnabled()
	if globalEnabled {
		return true
	}
	if user.SteamID == nil || strings.TrimSpace(*user.SteamID) == "" {
		return false
	}
	steamID := strings.TrimSpace(*user.SteamID)
	var count int64
	if err := h.db.Model(&model.PublishTakeoverWhitelist{}).
		Where("steam_id = ? AND enabled = ?", steamID, true).
		Count(&count).Error; err != nil {
		return false
	}
	return publishTakeoverAllowed(globalEnabled, steamID, count > 0)
}

func publishTakeoverAllowed(globalEnabled bool, steamID string, whitelisted bool) bool {
	return globalEnabled || (strings.TrimSpace(steamID) != "" && whitelisted)
}

func (h *Handler) AdminGetSettings(c *gin.Context) {
	ok(c, "success", gin.H{"publishTakeoverEnabled": h.publishTakeoverEnabled()})
}

func (h *Handler) AdminUpdateSettings(c *gin.Context) {
	var req struct {
		PublishTakeoverEnabled *bool `json:"publishTakeoverEnabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	if req.PublishTakeoverEnabled == nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "publishTakeoverEnabled is required")
		return
	}
	if err := h.saveAppConfig(model.AppConfigPublishTakeoverEnabled, boolString(*req.PublishTakeoverEnabled)); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "saved", gin.H{"publishTakeoverEnabled": *req.PublishTakeoverEnabled})
}

func (h *Handler) saveAppConfig(key string, value string) error {
	config := model.AppConfig{ConfigKey: key, ConfigValue: value}
	return h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "config_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"config_value"}),
	}).Create(&config).Error
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
