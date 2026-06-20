package httpapi

import (
	"steam-game-takeover-backend/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config, db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	h := NewHandler(cfg, db)

	api := r.Group("/api")
	api.GET("/health", h.Health)
	api.POST("/auth/wx-login", h.WXLogin)

	api.GET("/takeovers", h.UserAuth(), h.ListTakeovers)
	api.GET("/takeovers/:takeoverId", h.UserAuth(), h.GetTakeover)
	api.POST("/takeovers", h.UserAuth(), h.CreateTakeover)
	api.POST("/takeovers/:takeoverId/join", h.UserAuth(), h.JoinTakeover)

	api.GET("/me/profile", h.UserAuth(), h.GetProfile)
	api.PUT("/me/profile", h.UserAuth(), h.SaveProfile)

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
