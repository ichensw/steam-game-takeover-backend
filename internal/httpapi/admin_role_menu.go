package httpapi

import (
	"encoding/json"
	"net/http"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
)

var allAdminMenuKeys = []string{
	"dashboard", "takeovers", "reports", "users", "admin-users",
	"kook-channels", "kook-roles", "kook-members", "kook-users",
	"feedbacks", "announcements", "settings",
}

func defaultAdminMenuKeys(role string) []string {
	switch role {
	case model.AdminRoleSuperAdmin:
		return allAdminMenuKeys
	case model.AdminRoleKookAdmin:
		return []string{"dashboard", "takeovers", "reports", "users", "kook-channels", "kook-roles", "kook-members", "kook-users", "feedbacks", "announcements", "settings"}
	default:
		return []string{"dashboard", "takeovers", "reports", "users", "feedbacks", "announcements", "settings"}
	}
}

func normalizeAdminMenuKeys(keys []string) []string {
	allowed := map[string]struct{}{}
	for _, key := range allAdminMenuKeys {
		allowed[key] = struct{}{}
	}
	seen := map[string]struct{}{}
	result := []string{}
	for _, key := range keys {
		if _, ok := allowed[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func (h *Handler) adminMenuKeys(role string) []string {
	var row model.AdminRoleMenu
	if err := h.db.Where("role = ?", role).First(&row).Error; err != nil {
		return defaultAdminMenuKeys(role)
	}
	var keys []string
	if err := json.Unmarshal([]byte(row.MenuKeys), &keys); err != nil {
		return defaultAdminMenuKeys(role)
	}
	return normalizeAdminMenuKeys(keys)
}

func (h *Handler) toAdminUserDTO(admin model.AdminUser) adminUserDTO {
	dto := toAdminUserDTO(admin)
	dto.MenuKeys = h.adminMenuKeys(dto.Role)
	return dto
}

func (h *Handler) AdminListRoleMenus(c *gin.Context) {
	ok(c, "success", gin.H{
		"allMenus": []gin.H{
			{"key": "dashboard", "label": "控制台"},
			{"key": "takeovers", "label": "接龙管理"},
			{"key": "reports", "label": "举报审核"},
			{"key": "users", "label": "用户管理"},
			{"key": "admin-users", "label": "管理员账号"},
			{"key": "kook-channels", "label": "KOOK 频道"},
			{"key": "kook-roles", "label": "KOOK 角色"},
			{"key": "kook-members", "label": "KOOK 成员"},
			{"key": "kook-users", "label": "KOOK 用户"},
			{"key": "feedbacks", "label": "反馈管理"},
			{"key": "announcements", "label": "公告管理"},
			{"key": "settings", "label": "系统设置"},
		},
		"roles": []gin.H{
			{"role": model.AdminRoleSuperAdmin, "label": "超级管理员", "menuKeys": h.adminMenuKeys(model.AdminRoleSuperAdmin)},
			{"role": model.AdminRoleKookAdmin, "label": "Kook 管理员", "menuKeys": h.adminMenuKeys(model.AdminRoleKookAdmin)},
			{"role": model.AdminRoleAdmin, "label": "普通管理员", "menuKeys": h.adminMenuKeys(model.AdminRoleAdmin)},
		},
	})
}

func (h *Handler) AdminUpdateRoleMenus(c *gin.Context) {
	var req struct {
		Roles []struct {
			Role     string   `json:"role"`
			MenuKeys []string `json:"menuKeys"`
		} `json:"roles"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	for _, item := range req.Roles {
		role := normalizeAdminRole(item.Role)
		keys := normalizeAdminMenuKeys(item.MenuKeys)
		if role == model.AdminRoleSuperAdmin && !containsString(keys, "admin-users") {
			keys = append(keys, "admin-users")
		}
		bytes, _ := json.Marshal(keys)
		if err := h.db.Exec("INSERT INTO ttw_admin_role_menu (`role`, `menu_keys`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `menu_keys` = VALUES(`menu_keys`)", role, string(bytes)).Error; err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
			return
		}
	}
	ok(c, "saved", nil)
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
