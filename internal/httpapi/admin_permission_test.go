package httpapi

import (
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestAdminPermissionDefaults(t *testing.T) {
	if !adminHasPermission(model.AdminUser{Username: "admin"}, model.AdminPermissionKookManage) {
		t.Fatal("default admin must have all permissions")
	}
	if adminHasPermission(model.AdminUser{Username: "ops"}, model.AdminPermissionKookManage) {
		t.Fatal("normal admin without permission must not manage KOOK")
	}
	permissions := `["kook:manage"]`
	if !adminHasPermission(model.AdminUser{Username: "ops", Role: model.AdminRoleAdmin, Permissions: &permissions}, model.AdminPermissionKookManage) {
		t.Fatal("normal admin with kook permission must manage KOOK")
	}
}
