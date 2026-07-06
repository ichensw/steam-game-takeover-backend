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
	api.GET("/app-config", h.GetAppConfig)
	api.POST("/auth/wx-login", h.WXLogin)
	api.POST("/auth/bot-login", h.BotLogin)

	api.GET("/takeovers", h.UserAuth(), h.ListTakeovers)
	api.GET("/takeovers/:takeoverId", h.UserAuth(), h.GetTakeover)
	api.POST("/takeovers", h.UserAuth(), h.CreateTakeover)
	api.PUT("/takeovers/:takeoverId", h.UserAuth(), h.UpdateTakeover)
	api.DELETE("/takeovers/:takeoverId", h.UserAuth(), h.DeleteTakeover)
	api.POST("/takeovers/:takeoverId/join", h.UserAuth(), h.JoinTakeover)
	api.POST("/takeovers/:takeoverId/leave", h.UserAuth(), h.LeaveTakeover)
	api.PUT("/takeovers/:takeoverId/member-remark", h.UserAuth(), h.UpdateMemberRemark)
	api.POST("/takeovers/:takeoverId/reports", h.UserAuth(), h.ReportTakeoverMember)

	api.GET("/me/profile", h.UserAuth(), h.GetProfile)
	api.GET("/me/summary", h.UserAuth(), h.GetMeSummary)
	api.GET("/me/takeovers", h.UserAuth(), h.ListMyTakeovers)
	api.PUT("/me/profile", h.UserAuth(), h.SaveProfile)
	api.POST("/uploads/image", h.UserAuth(), h.UploadImage)
	api.POST("/user-feedback", h.UserAuth(), h.SubmitUserFeedback)
	api.POST("/user-feedback/images", h.UserAuth(), h.UploadUserFeedbackImage)
	api.GET("/kook/channels", h.UserAuth(), h.ListKookChannels)
	api.GET("/kook/channels/all", h.UserAuth(), h.ListAllKookChannels)
	api.GET("/kook/channel-tree", h.UserAuth(), h.ListKookChannelTree)

	admin := api.Group("/admin")
	admin.POST("/auth/login", h.AdminLogin)
	admin.POST("/auth/logout", h.AdminLogout)

	adminAuthed := admin.Group("", h.AdminAuth())
	adminAuthed.POST("/admin-users", h.AdminCreateAdminUser)
	adminAuthed.GET("/admin-users", h.AdminListAdminUsers)
	adminAuthed.GET("/dashboard/summary", h.AdminDashboardSummary)
	adminAuthed.GET("/settings", h.AdminGetSettings)
	adminAuthed.PUT("/settings", h.AdminUpdateSettings)
	adminAuthed.GET("/takeovers", h.AdminListTakeovers)
	adminAuthed.GET("/takeovers/:takeoverId", h.AdminGetTakeover)
	adminAuthed.PUT("/takeovers/:takeoverId", h.AdminUpdateTakeover)
	adminAuthed.DELETE("/takeovers/:takeoverId", h.AdminDeleteTakeover)
	adminAuthed.GET("/users", h.AdminListUsers)
	adminAuthed.GET("/users/summary", h.AdminUserSummary)
	adminAuthed.POST("/users/admin/batch", h.AdminBatchSetUserAdmin)
	adminAuthed.GET("/users/:userId", h.AdminGetUser)
	adminAuthed.POST("/users/:userId/ban", h.AdminBanUser)
	adminAuthed.POST("/users/:userId/unban", h.AdminUnbanUser)
	adminAuthed.POST("/users/:userId/credit", h.AdminRestoreUserCredit)
	adminAuthed.GET("/reports", h.AdminListReports)
	adminAuthed.GET("/reports/:reportId", h.AdminGetReport)
	adminAuthed.POST("/reports/:reportId/approve", h.AdminApproveReport)
	adminAuthed.POST("/reports/:reportId/reject", h.AdminRejectReport)
	adminAuthed.POST("/reports/:reportId/handle", h.AdminHandleReport)
	adminAuthed.GET("/user-feedbacks", h.AdminListUserFeedbacks)
	adminAuthed.GET("/user-feedbacks/:feedbackId", h.AdminGetUserFeedback)
	adminAuthed.PUT("/user-feedbacks/:feedbackId/status", h.AdminUpdateUserFeedbackStatus)
	adminAuthed.POST("/publish-whitelist/batch", h.AdminBatchPublishWhitelist)

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
