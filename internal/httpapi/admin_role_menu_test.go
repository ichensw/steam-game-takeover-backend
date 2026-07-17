package httpapi

import (
	"reflect"
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestDefaultAdminMenuKeysIncludeWechatForSuperAdminOnly(t *testing.T) {
	for _, key := range []string{"wechat-messages", "wechat-summary", "wechat-stats", "wechat-database", "wechat-wxbot-control"} {
		if !containsString(defaultAdminMenuKeys(model.AdminRoleSuperAdmin), key) {
			t.Fatalf("super admin missing %s", key)
		}
		if containsString(defaultAdminMenuKeys(model.AdminRoleAdmin), key) {
			t.Fatalf("normal admin unexpectedly has %s", key)
		}
	}
}

func TestNormalizeAdminMenuKeysAcceptsWechatKeys(t *testing.T) {
	got := normalizeAdminMenuKeys([]string{"users", "user-blocks", "invalid", "wechat-summary", "user-blocks"})
	want := []string{"users", "user-blocks", "wechat-summary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestEnsureRoleMenuKeysBackfillsStoredSuperAdminMenus(t *testing.T) {
	got := ensureRoleMenuKeys(model.AdminRoleSuperAdmin, []string{"dashboard"})
	for _, key := range []string{"user-blocks", "admin-users", "wechat-messages", "wechat-summary", "wechat-stats", "wechat-database", "wechat-wxbot-control"} {
		if !containsString(got, key) {
			t.Fatalf("stored super admin menus missing %s", key)
		}
	}

	normal := ensureRoleMenuKeys(model.AdminRoleAdmin, []string{"dashboard"})
	if containsString(normal, "wechat-messages") {
		t.Fatal("normal admin received forced WeChat permissions")
	}
}
