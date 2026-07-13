package httpapi

import (
	"net/http"
	"sync"
	"time"

	"steam-game-takeover-backend/internal/config"

	"gorm.io/gorm"
)

type Handler struct {
	cfg                    config.Config
	db                     *gorm.DB
	tokenMu                sync.Mutex
	wxToken                string
	wxTokenUntil           time.Time
	wechatBotClient        *http.Client
	wechatBotSummaryClient *http.Client
}

func NewHandler(cfg config.Config, db *gorm.DB) *Handler {
	proxyTimeout := cfg.WechatBotProxyTimeout
	if proxyTimeout <= 0 {
		proxyTimeout = 20 * time.Second
	}
	summaryTimeout := cfg.WechatBotSummaryTimeout
	if summaryTimeout <= 0 {
		summaryTimeout = 75 * time.Second
	}
	return &Handler{
		cfg:                    cfg,
		db:                     db,
		wechatBotClient:        &http.Client{Timeout: proxyTimeout},
		wechatBotSummaryClient: &http.Client{Timeout: summaryTimeout},
	}
}
