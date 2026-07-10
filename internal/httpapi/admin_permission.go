package httpapi

import (
	"encoding/json"
	"net/http"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) AdminRequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, ok := currentAdmin(c)
		if !ok || !adminHasPermission(admin, permission) {
			fail(c, http.StatusForbidden, CodeAdminUnauthorized, "permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) AdminRequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, ok := currentAdmin(c)
		if !ok || !isSuperAdmin(admin) {
			fail(c, http.StatusForbidden, CodeAdminUnauthorized, "permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func adminRole(admin model.AdminUser) string {
	if admin.Role != "" {
		return admin.Role
	}
	if admin.Username == "admin" {
		return model.AdminRoleSuperAdmin
	}
	return model.AdminRoleAdmin
}

func isSuperAdmin(admin model.AdminUser) bool {
	return admin.Username == "admin" || adminRole(admin) == model.AdminRoleSuperAdmin
}

func adminHasPermission(admin model.AdminUser, permission string) bool {
	if isSuperAdmin(admin) {
		return true
	}
	for _, item := range adminPermissions(admin) {
		if item == permission {
			return true
		}
	}
	return false
}

func adminPermissions(admin model.AdminUser) []string {
	if admin.Permissions == nil || *admin.Permissions == "" {
		return []string{}
	}
	var permissions []string
	if err := json.Unmarshal([]byte(*admin.Permissions), &permissions); err != nil {
		return []string{}
	}
	return permissions
}

func normalizeAdminRole(role string) string {
	if role == model.AdminRoleSuperAdmin {
		return model.AdminRoleSuperAdmin
	}
	return model.AdminRoleAdmin
}

func normalizeAdminPermissions(permissions []string) []string {
	allowed := map[string]struct{}{
		model.AdminPermissionKookManage: {},
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if _, ok := allowed[permission]; !ok {
			continue
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		result = append(result, permission)
	}
	return result
}

func adminPermissionsJSON(permissions []string) (*string, error) {
	normalized := normalizeAdminPermissions(permissions)
	if len(normalized) == 0 {
		empty := "[]"
		return &empty, nil
	}
	bytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	value := string(bytes)
	return &value, nil
}
