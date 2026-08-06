package httpapi

import (
	"database/sql"
	"sync"
	"time"

	"steam-game-takeover-backend/internal/config"

	"gorm.io/gorm"
)

type Handler struct {
	cfg                    config.Config
	db                     *gorm.DB
	wechatBotDB            *sql.DB
	tokenMu                sync.Mutex
	wxToken                string
	wxTokenUntil           time.Time
	kookChannelNamesMu     sync.Mutex
	kookChannelNames       map[string]string
	kookChannelNamesUntil  time.Time
	kookChannelNamesReload bool
	dashboardMu            sync.Mutex
	dashboardCache         dashboardSnapshot
	dashboardCacheUntil    time.Time
}

func NewHandler(cfg config.Config, db *gorm.DB, wechatBotDB ...*sql.DB) *Handler {
	var wxDB *sql.DB
	if len(wechatBotDB) > 0 {
		wxDB = wechatBotDB[0]
	}
	return &Handler{
		cfg:         cfg,
		db:          db,
		wechatBotDB: wxDB,
	}
}
