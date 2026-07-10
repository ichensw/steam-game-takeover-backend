package httpapi

import (
	"net/http"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) AdminRequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, ok := currentAdmin(c)
		if !ok || !adminHasRole(admin, roles...) {
			fail(c, http.StatusForbidden, CodeAdminUnauthorized, "permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) AdminRequireSuperAdmin() gin.HandlerFunc {
	return h.AdminRequireRole(model.AdminRoleSuperAdmin)
}

func adminRole(admin model.AdminUser) string {
	if admin.Role != "" {
		return admin.Role
	}
	return model.AdminRoleAdmin
}

func isSuperAdmin(admin model.AdminUser) bool {
	return adminRole(admin) == model.AdminRoleSuperAdmin
}

func adminHasRole(admin model.AdminUser, roles ...string) bool {
	if isSuperAdmin(admin) {
		return true
	}
	role := adminRole(admin)
	for _, allowed := range roles {
		if role == allowed {
			return true
		}
	}
	return false
}

func normalizeAdminRole(role string) string {
	switch role {
	case model.AdminRoleSuperAdmin:
		return model.AdminRoleSuperAdmin
	case model.AdminRoleKookAdmin:
		return model.AdminRoleKookAdmin
	default:
		return model.AdminRoleAdmin
	}
}
