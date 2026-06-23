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
