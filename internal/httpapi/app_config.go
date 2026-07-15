package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

const (
	defaultDailyTakeoverExpirationDays = 10
	minDailyTakeoverExpirationDays     = 1
	maxDailyTakeoverExpirationDays     = 365
	defaultWechatSummaryMaxMessages    = 1000
	minWechatSummaryMaxMessages        = 1
	maxWechatSummaryMaxMessages        = 10000
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

func (h *Handler) kookVerifyToken() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigKookVerifyToken))
}

func (h *Handler) kookEncryptKey() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigKookEncryptKey))
}

func (h *Handler) apiBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(h.appConfigValue(model.AppConfigAPIBaseURL)), "/")
}

func (h *Handler) aiExtractEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractEnabled))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (h *Handler) aiExtractAPIKey() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractAPIKey))
}

func (h *Handler) aiExtractBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractBaseURL)), "/")
}

func (h *Handler) aiExtractModel() string {
	return strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractModel))
}

func parseDailyTakeoverExpirationDays(raw string) int {
	days, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || days < minDailyTakeoverExpirationDays || days > maxDailyTakeoverExpirationDays {
		return defaultDailyTakeoverExpirationDays
	}
	return days
}

func (h *Handler) dailyTakeoverExpirationDays() int {
	return parseDailyTakeoverExpirationDays(h.appConfigValue(model.AppConfigDailyTakeoverExpirationDays))
}

func parseWechatSummaryMaxMessages(raw string) int {
	messages, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || messages < minWechatSummaryMaxMessages || messages > maxWechatSummaryMaxMessages {
		return defaultWechatSummaryMaxMessages
	}
	return messages
}

func (h *Handler) wechatSummaryMaxMessages() int {
	return parseWechatSummaryMaxMessages(h.appConfigValue(model.AppConfigWechatSummaryMaxMessages))
}

func validateDailyTakeoverExpirationDays(days int) error {
	if days < minDailyTakeoverExpirationDays || days > maxDailyTakeoverExpirationDays {
		return errors.New("dailyTakeoverExpirationDays must be between 1 and 365")
	}
	return nil
}

func validateWechatSummaryMaxMessages(messages int) error {
	if messages < minWechatSummaryMaxMessages || messages > maxWechatSummaryMaxMessages {
		return errors.New("wechatSummaryMaxMessages must be between 1 and 10000")
	}
	return nil
}

func (h *Handler) GetAppConfig(c *gin.Context) {
	ok(c, "success", gin.H{
		"apiBaseUrl": h.apiBaseURL(),
	})
}

func (h *Handler) appConfigValue(key string) string {
	if h.db == nil {
		return ""
	}
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
		"publishTakeoverEnabled":      h.publishTakeoverEnabled(),
		"uapiKey":                     h.uapiKey(),
		"steamWebApiKey":              h.steamWebAPIKey(),
		"kookBotToken":                h.kookBotToken(),
		"kookGuildId":                 h.kookGuildID(),
		"kookVerifyToken":             h.kookVerifyToken(),
		"kookEncryptKey":              h.kookEncryptKey(),
		"aiExtractEnabled":            h.aiExtractEnabled(),
		"aiExtractApiKey":             h.aiExtractAPIKey(),
		"aiExtractBaseUrl":            h.aiExtractBaseURL(),
		"aiExtractModel":              h.aiExtractModel(),
		"dailyTakeoverExpirationDays": h.dailyTakeoverExpirationDays(),
		"wechatSummaryMaxMessages":    h.wechatSummaryMaxMessages(),
	})
}

func (h *Handler) AdminUpdateSettings(c *gin.Context) {
	var req struct {
		PublishTakeoverEnabled      *bool   `json:"publishTakeoverEnabled"`
		UAPIKey                     *string `json:"uapiKey"`
		SteamWebAPIKey              *string `json:"steamWebApiKey"`
		KookBotToken                *string `json:"kookBotToken"`
		KookGuildID                 *string `json:"kookGuildId"`
		KookVerifyToken             *string `json:"kookVerifyToken"`
		KookEncryptKey              *string `json:"kookEncryptKey"`
		AIExtractEnabled            *bool   `json:"aiExtractEnabled"`
		AIExtractAPIKey             *string `json:"aiExtractApiKey"`
		AIExtractBaseURL            *string `json:"aiExtractBaseUrl"`
		AIExtractModel              *string `json:"aiExtractModel"`
		DailyTakeoverExpirationDays *int    `json:"dailyTakeoverExpirationDays"`
		WechatSummaryMaxMessages    *int    `json:"wechatSummaryMaxMessages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	if req.PublishTakeoverEnabled == nil && req.UAPIKey == nil && req.SteamWebAPIKey == nil && req.KookBotToken == nil && req.KookGuildID == nil && req.KookVerifyToken == nil && req.KookEncryptKey == nil && req.AIExtractEnabled == nil && req.AIExtractAPIKey == nil && req.AIExtractBaseURL == nil && req.AIExtractModel == nil && req.DailyTakeoverExpirationDays == nil && req.WechatSummaryMaxMessages == nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "settings is required")
		return
	}
	if req.DailyTakeoverExpirationDays != nil {
		if err := validateDailyTakeoverExpirationDays(*req.DailyTakeoverExpirationDays); err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
			return
		}
	}
	if req.WechatSummaryMaxMessages != nil {
		if err := validateWechatSummaryMaxMessages(*req.WechatSummaryMaxMessages); err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
			return
		}
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
	if req.KookVerifyToken != nil {
		if err := h.saveAppConfig(model.AppConfigKookVerifyToken, strings.TrimSpace(*req.KookVerifyToken)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.KookEncryptKey != nil {
		if err := h.saveAppConfig(model.AppConfigKookEncryptKey, strings.TrimSpace(*req.KookEncryptKey)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.AIExtractEnabled != nil {
		if err := h.saveAppConfig(model.AppConfigAIExtractEnabled, boolString(*req.AIExtractEnabled)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.AIExtractAPIKey != nil {
		if err := h.saveAppConfig(model.AppConfigAIExtractAPIKey, strings.TrimSpace(*req.AIExtractAPIKey)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.AIExtractBaseURL != nil {
		if err := h.saveAppConfig(model.AppConfigAIExtractBaseURL, strings.TrimRight(strings.TrimSpace(*req.AIExtractBaseURL), "/")); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.AIExtractModel != nil {
		if err := h.saveAppConfig(model.AppConfigAIExtractModel, strings.TrimSpace(*req.AIExtractModel)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.DailyTakeoverExpirationDays != nil {
		if err := h.saveAppConfig(model.AppConfigDailyTakeoverExpirationDays, strconv.Itoa(*req.DailyTakeoverExpirationDays)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.WechatSummaryMaxMessages != nil {
		if err := h.saveAppConfig(model.AppConfigWechatSummaryMaxMessages, strconv.Itoa(*req.WechatSummaryMaxMessages)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	ok(c, "saved", gin.H{
		"publishTakeoverEnabled":      h.publishTakeoverEnabled(),
		"uapiKey":                     h.uapiKey(),
		"steamWebApiKey":              h.steamWebAPIKey(),
		"kookBotToken":                h.kookBotToken(),
		"kookGuildId":                 h.kookGuildID(),
		"kookVerifyToken":             h.kookVerifyToken(),
		"kookEncryptKey":              h.kookEncryptKey(),
		"aiExtractEnabled":            h.aiExtractEnabled(),
		"aiExtractApiKey":             h.aiExtractAPIKey(),
		"aiExtractBaseUrl":            h.aiExtractBaseURL(),
		"aiExtractModel":              h.aiExtractModel(),
		"dailyTakeoverExpirationDays": h.dailyTakeoverExpirationDays(),
		"wechatSummaryMaxMessages":    h.wechatSummaryMaxMessages(),
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
