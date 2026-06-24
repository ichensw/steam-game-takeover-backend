package httpapi

import (
	"strings"

	"steam-game-takeover-backend/internal/model"
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
