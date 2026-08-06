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
	aiExtractProviderGPT               = "gpt"
	aiExtractProviderDoubao            = "doubao"
	doubaoAPIBaseURL                   = "https://ark.cn-beijing.volces.com/api/v3"
)

var aiExtractModelDefaults = map[string]string{
	aiExtractProviderGPT:    "gpt-5.4-mini",
	aiExtractProviderDoubao: "doubao-seed-2-0-mini-260428",
}

var aiExtractModels = map[string]map[string]struct{}{
	aiExtractProviderGPT: {
		"gpt-5.4-mini": {},
		"gpt-5.5":      {},
		"gpt-5.2":      {},
	},
	aiExtractProviderDoubao: {
		"doubao-seed-2-0-mini-260428":  {},
		"doubao-seed-2-1-turbo-260628": {},
		"doubao-seed-2-1-pro-260628":   {},
	},
}

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
	if h.aiExtractProvider() == aiExtractProviderDoubao {
		return h.aiExtractDoubaoAPIKey()
	}
	return h.aiExtractGPTAPIKey()
}

func (h *Handler) aiExtractBaseURL() string {
	if h.aiExtractProvider() == aiExtractProviderDoubao {
		return doubaoAPIBaseURL
	}
	return h.aiExtractGPTBaseURL()
}

func (h *Handler) aiExtractProvider() string {
	return inferAIExtractProvider(
		h.appConfigValue(model.AppConfigAIExtractProvider),
		h.appConfigValue(model.AppConfigAIExtractBaseURL),
	)
}

func inferAIExtractProvider(configured string, legacyBaseURL string) string {
	if strings.TrimSpace(configured) != "" {
		if provider, err := normalizeAIExtractProvider(configured); err == nil {
			return provider
		}
	}
	if isDoubaoAPIBaseURL(legacyBaseURL) {
		return aiExtractProviderDoubao
	}
	return aiExtractProviderGPT
}

func (h *Handler) aiExtractGPTBaseURL() string {
	baseURL := normalizeAIBaseURL(h.appConfigValue(model.AppConfigAIExtractGPTBaseURL))
	if baseURL != "" {
		return baseURL
	}
	legacyBaseURL := h.appConfigValue(model.AppConfigAIExtractBaseURL)
	if h.aiExtractProvider() == aiExtractProviderGPT && !isDoubaoAPIBaseURL(legacyBaseURL) {
		return normalizeAIBaseURL(legacyBaseURL)
	}
	return ""
}

func (h *Handler) aiExtractGPTAPIKey() string {
	apiKey := strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractGPTAPIKey))
	if apiKey != "" {
		return apiKey
	}
	legacyBaseURL := h.appConfigValue(model.AppConfigAIExtractBaseURL)
	if h.aiExtractProvider() == aiExtractProviderGPT && !isDoubaoAPIBaseURL(legacyBaseURL) {
		return strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractAPIKey))
	}
	return ""
}

func (h *Handler) aiExtractDoubaoAPIKey() string {
	apiKey := strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractDoubaoAPIKey))
	if apiKey != "" {
		return apiKey
	}
	if h.aiExtractProvider() == aiExtractProviderDoubao && isDoubaoAPIBaseURL(h.appConfigValue(model.AppConfigAIExtractBaseURL)) {
		return strings.TrimSpace(h.appConfigValue(model.AppConfigAIExtractAPIKey))
	}
	return ""
}

func normalizeAIExtractProvider(value string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(value))
	if provider == "" {
		return aiExtractProviderGPT, nil
	}
	switch provider {
	case aiExtractProviderGPT, "openai", "chatgpt":
		return aiExtractProviderGPT, nil
	case aiExtractProviderDoubao, "dou bao", "豆包":
		return aiExtractProviderDoubao, nil
	default:
		return "", errors.New("aiExtractProvider must be gpt or doubao")
	}
}

func normalizeAIBaseURL(value string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(value), "/")
	return strings.TrimSuffix(baseURL, "/chat/completions")
}

func isDoubaoAPIBaseURL(value string) bool {
	return strings.EqualFold(normalizeAIBaseURL(value), doubaoAPIBaseURL)
}

func normalizeAIExtractModel(provider string, value string) (string, error) {
	normalizedProvider, err := normalizeAIExtractProvider(provider)
	if err != nil {
		return "", err
	}
	modelName := strings.TrimSpace(value)
	if modelName == "" {
		return aiExtractModelDefaults[normalizedProvider], nil
	}
	if _, ok := aiExtractModels[normalizedProvider][modelName]; !ok {
		return "", errors.New("aiExtractModel is not available for the selected provider")
	}
	return modelName, nil
}

func (h *Handler) aiExtractModel() string {
	modelName, err := normalizeAIExtractModel(h.aiExtractProvider(), h.appConfigValue(model.AppConfigAIExtractModel))
	if err != nil {
		return aiExtractModelDefaults[h.aiExtractProvider()]
	}
	return modelName
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

func parseConfigBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func validateDailyTakeoverExpirationDays(days int) error {
	if days < minDailyTakeoverExpirationDays || days > maxDailyTakeoverExpirationDays {
		return errors.New("dailyTakeoverExpirationDays must be between 1 and 365")
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
		"aiExtractProvider":           h.aiExtractProvider(),
		"aiExtractGptBaseUrl":         h.aiExtractGPTBaseURL(),
		"aiExtractGptApiKey":          h.aiExtractGPTAPIKey(),
		"aiExtractDoubaoApiKey":       h.aiExtractDoubaoAPIKey(),
		"aiExtractApiKey":             h.aiExtractAPIKey(),
		"aiExtractBaseUrl":            h.aiExtractBaseURL(),
		"aiExtractModel":              h.aiExtractModel(),
		"dailyTakeoverExpirationDays": h.dailyTakeoverExpirationDays(),
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
		AIExtractProvider           *string `json:"aiExtractProvider"`
		AIExtractGPTBaseURL         *string `json:"aiExtractGptBaseUrl"`
		AIExtractGPTAPIKey          *string `json:"aiExtractGptApiKey"`
		AIExtractDoubaoAPIKey       *string `json:"aiExtractDoubaoApiKey"`
		AIExtractAPIKey             *string `json:"aiExtractApiKey"`
		AIExtractBaseURL            *string `json:"aiExtractBaseUrl"`
		AIExtractModel              *string `json:"aiExtractModel"`
		DailyTakeoverExpirationDays *int    `json:"dailyTakeoverExpirationDays"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	if req.PublishTakeoverEnabled == nil && req.UAPIKey == nil && req.SteamWebAPIKey == nil && req.KookBotToken == nil && req.KookGuildID == nil && req.KookVerifyToken == nil && req.KookEncryptKey == nil && req.AIExtractEnabled == nil && req.AIExtractProvider == nil && req.AIExtractGPTBaseURL == nil && req.AIExtractGPTAPIKey == nil && req.AIExtractDoubaoAPIKey == nil && req.AIExtractAPIKey == nil && req.AIExtractBaseURL == nil && req.AIExtractModel == nil && req.DailyTakeoverExpirationDays == nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "settings is required")
		return
	}
	if req.DailyTakeoverExpirationDays != nil {
		if err := validateDailyTakeoverExpirationDays(*req.DailyTakeoverExpirationDays); err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
			return
		}
	}
	nextAIExtractProvider := h.aiExtractProvider()
	if req.AIExtractProvider != nil {
		provider, err := normalizeAIExtractProvider(*req.AIExtractProvider)
		if err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
			return
		}
		nextAIExtractProvider = provider
	}
	var normalizedAIExtractModel string
	if req.AIExtractModel != nil {
		modelName, err := normalizeAIExtractModel(nextAIExtractProvider, *req.AIExtractModel)
		if err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
			return
		}
		normalizedAIExtractModel = modelName
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
	if req.AIExtractProvider != nil {
		provider, _ := normalizeAIExtractProvider(*req.AIExtractProvider)
		if err := h.saveAppConfig(model.AppConfigAIExtractProvider, provider); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.AIExtractGPTBaseURL != nil {
		if err := h.saveAppConfig(model.AppConfigAIExtractGPTBaseURL, normalizeAIBaseURL(*req.AIExtractGPTBaseURL)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.AIExtractGPTAPIKey != nil {
		if err := h.saveAppConfig(model.AppConfigAIExtractGPTAPIKey, strings.TrimSpace(*req.AIExtractGPTAPIKey)); err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	if req.AIExtractDoubaoAPIKey != nil {
		if err := h.saveAppConfig(model.AppConfigAIExtractDoubaoAPIKey, strings.TrimSpace(*req.AIExtractDoubaoAPIKey)); err != nil {
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
		if err := h.saveAppConfig(model.AppConfigAIExtractModel, normalizedAIExtractModel); err != nil {
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
	ok(c, "saved", gin.H{
		"publishTakeoverEnabled":      h.publishTakeoverEnabled(),
		"uapiKey":                     h.uapiKey(),
		"steamWebApiKey":              h.steamWebAPIKey(),
		"kookBotToken":                h.kookBotToken(),
		"kookGuildId":                 h.kookGuildID(),
		"kookVerifyToken":             h.kookVerifyToken(),
		"kookEncryptKey":              h.kookEncryptKey(),
		"aiExtractEnabled":            h.aiExtractEnabled(),
		"aiExtractProvider":           h.aiExtractProvider(),
		"aiExtractGptBaseUrl":         h.aiExtractGPTBaseURL(),
		"aiExtractGptApiKey":          h.aiExtractGPTAPIKey(),
		"aiExtractDoubaoApiKey":       h.aiExtractDoubaoAPIKey(),
		"aiExtractApiKey":             h.aiExtractAPIKey(),
		"aiExtractBaseUrl":            h.aiExtractBaseURL(),
		"aiExtractModel":              h.aiExtractModel(),
		"dailyTakeoverExpirationDays": h.dailyTakeoverExpirationDays(),
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
