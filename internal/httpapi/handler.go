package httpapi

import (
	"steam-game-takeover-backend/internal/config"

	"gorm.io/gorm"
)

type Handler struct {
	cfg config.Config
	db  *gorm.DB
}

func NewHandler(cfg config.Config, db *gorm.DB) *Handler {
	return &Handler{cfg: cfg, db: db}
}
