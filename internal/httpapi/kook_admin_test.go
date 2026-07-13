package httpapi

import "testing"

func TestKookRoleUserNameHelpers(t *testing.T) {
	data := map[string]interface{}{
		"permission_users": []interface{}{
			map[string]interface{}{
				"user": map[string]interface{}{
					"id":       "user-1",
					"nickname": "nick",
				},
			},
		},
	}

	rows := kookRoleRows(data)
	if len(rows) != 1 {
		t.Fatalf("kookRoleRows() length = %d, want 1", len(rows))
	}
	if got := kookRoleUserID(rows[0]); got != "user-1" {
		t.Fatalf("kookRoleUserID() = %q, want user-1", got)
	}
	if got := kookRoleInlineUserName(rows[0]); got != "nick" {
		t.Fatalf("kookRoleInlineUserName() = %q, want nick", got)
	}
}
