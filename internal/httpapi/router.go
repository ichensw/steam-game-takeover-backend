package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
)

func NewRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api")
	api.GET("/health", h.Health)
	api.GET("/app-config", h.GetAppConfig)
	api.POST("/auth/wx-login", h.WXLogin)
	api.POST("/auth/bot-login", h.BotLogin)

	api.GET("/takeovers", h.UserAuth(), h.ListTakeovers)
	api.GET("/takeovers/summary", h.UserAuth(), h.ListTakeoverSummaries)
	api.GET("/takeovers/:takeoverId", h.OptionalUserAuth(), h.GetTakeover)
	api.GET("/takeovers/:takeoverId/member-activities", h.OptionalUserAuth(), h.ListTakeoverMemberActivities)
	api.POST("/takeovers", h.UserAuth(), h.CreateTakeover)
	api.PUT("/takeovers/:takeoverId", h.UserAuth(), h.UpdateTakeover)
	api.DELETE("/takeovers/:takeoverId", h.UserAuth(), h.DeleteTakeover)
	api.POST("/takeovers/:takeoverId/join", h.UserAuth(), h.JoinTakeover)
	api.POST("/takeovers/:takeoverId/leave", h.UserAuth(), h.LeaveTakeover)
	api.PUT("/takeovers/:takeoverId/member-remark", h.UserAuth(), h.UpdateMemberRemark)
	api.POST("/takeovers/:takeoverId/members/:userId/kick", h.UserAuth(), h.KickTakeoverMember)
	api.POST("/takeovers/:takeoverId/members/:userId/block", h.UserAuth(), h.BlockTakeoverMember)
	api.POST("/takeovers/:takeoverId/reminder-subscription", h.UserAuth(), h.SubscribeTakeoverReminder)
	api.POST("/takeovers/:takeoverId/reports", h.UserAuth(), h.ReportTakeoverMember)

	api.GET("/me/profile", h.UserAuth(), h.GetProfile)
	api.GET("/me/summary", h.UserAuth(), h.GetMeSummary)
	api.GET("/me/takeovers", h.UserAuth(), h.ListMyTakeovers)
	api.GET("/me/credit-logs", h.UserAuth(), h.ListMyCreditLogs)
	api.GET("/me/blocked-users", h.UserAuth(), h.ListBlockedUsers)
	api.PUT("/me/profile", h.UserAuth(), h.SaveProfile)
	api.POST("/users/:userId/block", h.UserAuth(), h.BlockUser)
	api.POST("/users/:userId/unblock", h.UserAuth(), h.UnblockUser)
	api.POST("/uploads/image", h.UserAuth(), h.UploadImage)
	api.POST("/user-feedback", h.UserAuth(), h.SubmitUserFeedback)
	api.GET("/user-feedbacks", h.UserAuth(), h.ListMyUserFeedbacks)
	api.POST("/user-feedback/images", h.UserAuth(), h.UploadUserFeedbackImage)
	api.GET("/announcements/current", h.UserAuth(), h.GetCurrentAnnouncement)
	api.POST("/announcements/:announcementId/read", h.UserAuth(), h.MarkAnnouncementRead)
	api.GET("/kook/channels", h.UserAuth(), h.ListKookChannels)
	api.GET("/kook/channels/all", h.UserAuth(), h.ListAllKookChannels)
	api.GET("/kook/channel-tree", h.UserAuth(), h.ListKookChannelTree)
	api.POST("/kook/webhook", h.KookWebhook)
	api.Any("/wxbot/*path", h.ProxyWechatBotControl)

	admin := api.Group("/admin")
	admin.POST("/auth/login", h.AdminLogin)
	admin.POST("/auth/logout", h.AdminLogout)

	adminAuthed := admin.Group("", h.AdminAuth())
	adminAuthed.GET("/me", h.AdminGetMe)
	adminAuthed.PUT("/me", h.AdminUpdateMe)
	adminAuthed.PUT("/me/password", h.AdminUpdateMePassword)
	adminAuthed.POST("/admin-users", h.AdminRequireSuperAdmin(), h.AdminCreateAdminUser)
	adminAuthed.GET("/admin-users", h.AdminRequireSuperAdmin(), h.AdminListAdminUsers)
	adminAuthed.PUT("/admin-users/:adminUserId", h.AdminRequireSuperAdmin(), h.AdminUpdateAdminUser)
	adminAuthed.GET("/role-menus", h.AdminRequireSuperAdmin(), h.AdminListRoleMenus)
	adminAuthed.PUT("/role-menus", h.AdminRequireSuperAdmin(), h.AdminUpdateRoleMenus)
	adminAuthed.Any("/wechat-bot/*path", h.AdminProxyWechatBot)
	adminAuthed.GET("/dashboard/summary", h.AdminDashboardSummary)
	adminAuthed.GET("/settings", h.AdminGetSettings)
	adminAuthed.PUT("/settings", h.AdminUpdateSettings)
	adminAuthed.POST("/uploads/image", h.AdminUploadImage)
	adminAuthed.GET("/takeovers", h.AdminListTakeovers)
	adminAuthed.POST("/takeovers", h.AdminCreateTakeover)
	adminAuthed.POST("/takeovers/summary/refresh", h.AdminRefreshTakeoverSummaries)
	adminAuthed.GET("/takeovers/:takeoverId/member-activities", h.AdminListTakeoverMemberActivities)
	adminAuthed.GET("/takeovers/:takeoverId", h.AdminGetTakeover)
	adminAuthed.PUT("/takeovers/:takeoverId", h.AdminUpdateTakeover)
	adminAuthed.DELETE("/takeovers/:takeoverId", h.AdminDeleteTakeover)
	adminAuthed.GET("/users", h.AdminListUsers)
	adminAuthed.GET("/users/summary", h.AdminUserSummary)
	adminAuthed.POST("/users/admin/batch", h.AdminBatchSetUserAdmin)
	adminAuthed.GET("/user-blocks", h.AdminListUserBlocks)
	adminAuthed.POST("/user-blocks", h.AdminCreateUserBlock)
	adminAuthed.PUT("/user-blocks/:blockId", h.AdminUpdateUserBlock)
	adminAuthed.DELETE("/user-blocks/:blockId", h.AdminDeleteUserBlock)
	adminAuthed.GET("/users/:userId", h.AdminGetUser)
	adminAuthed.POST("/users/:userId/ban", h.AdminBanUser)
	adminAuthed.POST("/users/:userId/unban", h.AdminUnbanUser)
	adminAuthed.POST("/users/:userId/credit", h.AdminRestoreUserCredit)
	adminAuthed.POST("/users/:userId/credit/penalty", h.AdminPenalizeUserCredit)
	adminAuthed.POST("/takeover-view/batch", h.AdminBatchSetTakeoverView)
	adminAuthed.GET("/reports", h.AdminListReports)
	adminAuthed.POST("/reports", h.AdminCreateReport)
	adminAuthed.GET("/reports/:reportId", h.AdminGetReport)
	adminAuthed.POST("/reports/:reportId/approve", h.AdminApproveReport)
	adminAuthed.POST("/reports/:reportId/reject", h.AdminRejectReport)
	adminAuthed.POST("/reports/:reportId/handle", h.AdminHandleReport)
	adminAuthed.GET("/user-feedbacks", h.AdminListUserFeedbacks)
	adminAuthed.GET("/user-feedbacks/:feedbackId", h.AdminGetUserFeedback)
	adminAuthed.PUT("/user-feedbacks/:feedbackId/status", h.AdminUpdateUserFeedbackStatus)
	adminAuthed.GET("/announcements", h.AdminListAnnouncements)
	adminAuthed.POST("/announcements", h.AdminCreateAnnouncement)
	adminAuthed.GET("/announcements/:announcementId", h.AdminGetAnnouncement)
	adminAuthed.PUT("/announcements/:announcementId", h.AdminUpdateAnnouncement)
	adminAuthed.POST("/announcements/:announcementId/enable", h.AdminSetAnnouncementEnabled)
	adminAuthed.POST("/announcements/:announcementId/disable", h.AdminSetAnnouncementDisabled)
	adminAuthed.DELETE("/announcements/:announcementId", h.AdminDeleteAnnouncement)
	kookAdmin := adminAuthed.Group("", h.AdminRequireRole(model.AdminRoleKookAdmin))
	kookAdmin.GET("/kook-members", h.AdminListKookMembers)
	kookAdmin.POST("/kook-members", h.AdminCreateKookMember)
	kookAdmin.POST("/kook-members/sync", h.AdminSyncKookMembers)
	kookAdmin.GET("/kook-members/:kookMemberId", h.AdminGetKookMember)
	kookAdmin.PUT("/kook-members/:kookMemberId", h.AdminUpdateKookMember)
	kookAdmin.DELETE("/kook-members/:kookMemberId", h.AdminDeleteKookMember)
	kookAdmin.POST("/kook-members/:kookMemberId/blacklist", h.AdminBlacklistKookMember)
	kookAdmin.POST("/kook-members/:kookMemberId/unblacklist", h.AdminUnblacklistKookMember)
	kookAdmin.GET("/kook-channels", h.AdminListKookChannels)
	kookAdmin.GET("/kook-channels/usage-summary", h.AdminKookVoiceChannelUsageSummary)
	kookAdmin.GET("/kook-channel-sort/config", h.AdminGetKookChannelSortConfig)
	kookAdmin.PUT("/kook-channel-sort/config", h.AdminUpdateKookChannelSortConfig)
	kookAdmin.GET("/kook-channel-sort/runs", h.AdminListKookChannelSortRuns)
	kookAdmin.POST("/kook-channel-sort/preview", h.AdminPreviewKookChannelSort)
	kookAdmin.POST("/kook-channel-sort/run", h.AdminRunKookChannelSort)
	kookAdmin.POST("/kook-channels", h.AdminCreateKookChannel)
	kookAdmin.GET("/kook-channels/:channelId", h.AdminGetKookChannel)
	kookAdmin.POST("/kook-channels/:channelId/move", h.AdminMoveKookChannel)
	kookAdmin.PUT("/kook-channels/:channelId", h.AdminUpdateKookChannel)
	kookAdmin.DELETE("/kook-channels/:channelId", h.AdminDeleteKookChannel)
	kookAdmin.GET("/kook-channels/:channelId/users", h.AdminListKookChannelUsers)
	kookAdmin.POST("/kook-channels/:channelId/move-user", h.AdminMoveKookChannelUsers)
	kookAdmin.POST("/kook-channels/:channelId/kickout", h.AdminKickoutKookChannelUser)
	kookAdmin.GET("/kook-channels/:channelId/roles", h.AdminGetKookChannelRoles)
	kookAdmin.POST("/kook-channels/:channelId/roles", h.AdminCreateKookChannelRole)
	kookAdmin.PUT("/kook-channels/:channelId/roles", h.AdminUpdateKookChannelRole)
	kookAdmin.DELETE("/kook-channels/:channelId/roles", h.AdminDeleteKookChannelRole)
	kookAdmin.POST("/kook-channels/:channelId/roles/sync", h.AdminSyncKookChannelRoles)
	kookAdmin.GET("/kook-roles", h.AdminListKookGuildRoles)
	kookAdmin.POST("/kook-roles", h.AdminCreateKookGuildRole)
	kookAdmin.PUT("/kook-roles/:roleId", h.AdminUpdateKookGuildRole)
	kookAdmin.DELETE("/kook-roles/:roleId", h.AdminDeleteKookGuildRole)
	kookAdmin.POST("/kook-roles/:roleId/grant", h.AdminGrantKookGuildRole)
	kookAdmin.POST("/kook-roles/:roleId/revoke", h.AdminRevokeKookGuildRole)
	kookAdmin.GET("/kook-users/me", h.AdminGetKookUserMe)
	kookAdmin.GET("/kook-users/view/:userId", h.AdminGetKookUser)
	kookAdmin.POST("/kook-users/bot/offline", h.AdminOfflineKookBot)
	kookAdmin.POST("/kook-users/bot/online", h.AdminOnlineKookBot)
	kookAdmin.GET("/kook-users/bot/online-status", h.AdminGetKookBotOnlineStatus)
	kookAdmin.GET("/kook-voice/stats", h.AdminKookVoiceStats)
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
