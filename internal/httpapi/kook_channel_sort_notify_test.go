package httpapi

import (
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestKookChannelSortAdminUserIDs(t *testing.T) {
	roles := "[\"role-admin\"]"
	ids := kookChannelSortAdminUserIDs(map[string]interface{}{
		"permission_users": []interface{}{
			map[string]interface{}{
				"user":  map[string]interface{}{"id": "direct-admin"},
				"allow": float64(32),
				"deny":  float64(0),
			},
			map[string]interface{}{
				"user":  map[string]interface{}{"id": "blocked-admin"},
				"allow": float64(32),
				"deny":  float64(32),
			},
		},
		"permission_overwrites": []interface{}{
			map[string]interface{}{"role_id": "role-admin", "allow": float64(32), "deny": float64(0)},
		},
	}, []model.KookMember{
		{KookUserID: "role-admin-user", RoleIDs: &roles},
	})

	if len(ids) != 2 || ids[0] != "direct-admin" || ids[1] != "role-admin-user" {
		t.Fatalf("recipient IDs = %#v, want direct and role admin", ids)
	}
}
