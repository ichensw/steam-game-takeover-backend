package httpapi

import (
	"sync"
	"time"

	"steam-game-takeover-backend/internal/config"

	"gorm.io/gorm"
)

type Handler struct {
	cfg          config.Config
	db           *gorm.DB
	tokenMu      sync.Mutex
	wxToken      string
	wxTokenUntil time.Time
}

func NewHandler(cfg config.Config, db *gorm.DB) *Handler {
	return &Handler{cfg: cfg, db: db}
}
