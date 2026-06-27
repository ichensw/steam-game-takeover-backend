package httpapi

import (
	"net/http"
	"strings"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func (h *Handler) publishTakeoverEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(h.appConfigValue(model.AppConfigPublishTakeoverEnabled))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (h *Handler) uapiKey() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigUAPIKey))
}

func (h *Handler) steamWebAPIKey() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigSteamWebAPIKey))
}

func (h *Handler) kookBotToken() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigKookBotToken))
}

func (h *Handler) kookGuildID() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigKookGuildID))
}

func (h *Handler) apiBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(h.appConfigValue(model.AppConfigAPIBaseURL)), "/")
}

func (h *Handler) GetAppConfig(c *gin.Context) {
	ok(c, "success", gin.H{
		"apiBaseUrl": h.apiBaseURL(),
	})
}

func (h *Handler) appConfigValue(key string) string {
	var config model.AppConfig
	if err := h.db.Where("config_key = ?", key).First(&config).Error; err != nil {
		return ""
	}
	return config.ConfigValue
}

func (h *Handler) canPublishTakeover(user model.User) bool {
	globalEnabled := h.publishTakeoverEnabled()
	if globalEnabled {
		return true
	}
	steamID := strings.TrimSpace(stringValue(user.SteamID))
	var count int64
	if err := h.db.Model(&model.PublishTakeoverWhitelist{}).
		Where("enabled = ? AND (openid = ? OR (steam_id <> '' AND steam_id = ?))", true, user.OpenID, steamID).
		Count(&count).Error; err != nil {
		return false
	}
	return publishTakeoverAllowed(globalEnabled, count > 0)
}

func publishTakeoverAllowed(globalEnabled bool, whitelisted bool) bool {
	return globalEnabled || whitelisted
}

func (h *Handler) AdminGetSettings(c *gin.Context) {
	ok(c, "success", gin.H{
		"publishTakeoverEnabled": h.publishTakeoverEnabled(),
		"uapiKey":                h.uapiKey(),
		"steamWebApiKey":         h.steamWebAPIKey(),
		"kookBotToken":           h.kookBotToken(),
		"kookGuildId":            h.kookGuildID(),
	})
}

func (h *Handler) AdminUpdateSettings(c *gin.Context) {
	var req struct {
		PublishTakeoverEnabled *bool   `json:"publishTakeoverEnabled"`
		UAPIKey                *string `json:"uapiKey"`
		SteamWebAPIKey         *string `json:"steamWebApiKey"`
		KookBotToken           *string `json:"kookBotToken"`
		KookGuildID            *string `json:"kookGuildId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	if req.PublishTakeoverEnabled == nil && req.UAPIKey == nil && req.SteamWebAPIKey == nil && req.KookBotToken == nil && req.KookGuildID == nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "settings is required")
		return
	}
	if req.PublishTakeoverEnabled != nil {
		if err := h.saveAppConfig(model.AppConfigPublishTakeoverEnabled, boolString(*req.PublishTakeoverEnabled)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.UAPIKey != nil {
		if err := h.saveAppConfig(model.AppConfigUAPIKey, strings.TrimSpace(*req.UAPIKey)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.SteamWebAPIKey != nil {
		if err := h.saveAppConfig(model.AppConfigSteamWebAPIKey, strings.TrimSpace(*req.SteamWebAPIKey)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.KookBotToken != nil {
		if err := h.saveAppConfig(model.AppConfigKookBotToken, strings.TrimSpace(*req.KookBotToken)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.KookGuildID != nil {
		if err := h.saveAppConfig(model.AppConfigKookGuildID, strings.TrimSpace(*req.KookGuildID)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	ok(c, "saved", gin.H{
		"publishTakeoverEnabled": h.publishTakeoverEnabled(),
		"uapiKey":                h.uapiKey(),
		"steamWebApiKey":         h.steamWebAPIKey(),
		"kookBotToken":           h.kookBotToken(),
		"kookGuildId":            h.kookGuildID(),
	})
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
