package httpapi

import (
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestAdminRoles(t *testing.T) {
	if adminHasRole(model.AdminUser{Username: "admin"}, model.AdminRoleKookAdmin) {
		t.Fatal("username must not grant roles")
	}
	if adminHasRole(model.AdminUser{Username: "ops", Role: model.AdminRoleAdmin}, model.AdminRoleKookAdmin) {
		t.Fatal("normal admin must not manage KOOK")
	}
	if !adminHasRole(model.AdminUser{Username: "ops", Role: model.AdminRoleKookAdmin}, model.AdminRoleKookAdmin) {
		t.Fatal("kook admin must manage KOOK")
	}
	if !adminHasRole(model.AdminUser{Username: "ops", Role: model.AdminRoleSuperAdmin}, model.AdminRoleKookAdmin) {
		t.Fatal("super admin must manage KOOK")
	}
}
