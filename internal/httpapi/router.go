package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"steam-game-takeover-backend/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config, db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(corsMiddleware())

	h := NewHandler(cfg, db)

	api := r.Group("/api")
	api.GET("/health", h.Health)
	api.POST("/auth/wx-login", h.WXLogin)
	api.POST("/auth/web-login", h.WebLogin)
	api.POST("/auth/bot-login", h.BotLogin)

	api.GET("/takeovers", h.UserAuth(), h.ListTakeovers)
	api.GET("/takeovers/:takeoverId", h.UserAuth(), h.GetTakeover)
	api.POST("/takeovers", h.UserAuth(), h.CreateTakeover)
	api.POST("/takeovers/:takeoverId/join", h.UserAuth(), h.JoinTakeover)
	api.POST("/takeovers/:takeoverId/leave", h.UserAuth(), h.LeaveTakeover)

	api.GET("/me/profile", h.UserAuth(), h.GetProfile)
	api.PUT("/me/profile", h.UserAuth(), h.SaveProfile)
	api.POST("/uploads/image", h.UserAuth(), h.UploadImage)

	api.POST("/admin/login", h.AdminLogin)
	admin := api.Group("/admin", h.AdminAuth())
	admin.GET("/takeovers/:takeoverId", h.AdminGetTakeover)
	admin.PUT("/takeovers/:takeoverId", h.AdminUpdateTakeover)
	admin.DELETE("/takeovers/:takeoverId", h.AdminDeleteTakeover)
	admin.POST("/users/:userId/block", h.AdminBlockUser)
	admin.POST("/users/:userId/unblock", h.AdminUnblockUser)
	admin.GET("/blocked-users", h.AdminBlockedUsers)

	return r
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := map[string]struct{}{
		"http://127.0.0.1:5177":           {},
		"http://localhost:5177":           {},
		"https://tabbits-nest.vercel.app": {},
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowedOrigins[origin]; ok || isAllowedLocalDevOrigin(origin) || strings.HasSuffix(origin, ".vercel.app") {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func isAllowedLocalDevOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	if parsed.Scheme != "http" {
		return false
	}

	host := parsed.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}
