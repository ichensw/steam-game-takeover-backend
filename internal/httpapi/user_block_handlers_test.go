package httpapi

import (
	"strings"
	"testing"

	"steam-game-takeover-backend/internal/model"

	"gorm.io/gorm"
)

func TestCanBlockTakeoverMemberOnlyAllowsCreator(t *testing.T) {
	takeover := model.Takeover{CreatorUserID: 10}

	if !canBlockTakeoverMember(model.User{ID: 10}, takeover) {
		t.Fatal("creator should be allowed to block takeover member")
	}
	if canBlockTakeoverMember(model.User{ID: 20, IsAdmin: true}, takeover) {
		t.Fatal("admin should not block on behalf of takeover creator")
	}
}

func TestUserBlockRoutesAreRegistered(t *testing.T) {
	routes := NewRouter(&Handler{}).Routes()
	want := map[string]string{
		"GET /api/me/blocked-users":                             "",
		"POST /api/users/:userId/unblock":                       "",
		"POST /api/takeovers/:takeoverId/members/:userId/block": "",
		"GET /api/admin/user-blocks":                            "",
		"POST /api/admin/user-blocks":                           "",
		"PUT /api/admin/user-blocks/:blockId":                   "",
		"DELETE /api/admin/user-blocks/:blockId":                "",
	}
	for _, route := range routes {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing routes: %#v", want)
	}
}

func TestJoinRequestRoutesAreNotRegistered(t *testing.T) {
	for _, route := range NewRouter(&Handler{}).Routes() {
		if route.Path == "/api/takeovers/:takeoverId/join-requests" ||
			route.Path == "/api/takeovers/:takeoverId/join-requests/:requestId/approve" ||
			route.Path == "/api/takeovers/:takeoverId/join-requests/:requestId/reject" {
			t.Fatalf("join request route should not be registered: %s %s", route.Method, route.Path)
		}
	}
}

func TestActiveTakeoverMemberBlockQueryOnlyMatchesJoinedMembers(t *testing.T) {
	db := newTakeoverExpirationDryRunDB(t)
	sqlText := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return activeTakeoverMemberBlockQuery(tx, 12, 34).Limit(1).Count(new(int64))
	})
	for _, clause := range []string{
		"FROM ttw_takeover_member AS m",
		"JOIN ttw_user_block AS b ON b.owner_user_id = m.user_id",
		"m.takeover_id = 12",
		"m.member_state = 1",
		"b.blocked_user_id = 34",
	} {
		if !strings.Contains(sqlText, clause) {
			t.Fatalf("member block SQL missing %q: %s", clause, sqlText)
		}
	}
}

func TestTakeoverJoinUnavailableMessageIsGeneric(t *testing.T) {
	if got := friendlyMessage(CodeTakeoverJoinUnavailable, ""); got != "当前接龙暂时无法加入，请稍后重试。" {
		t.Fatalf("friendlyMessage() = %q", got)
	}
}

func TestValidateUserBlockPairRejectsSelf(t *testing.T) {
	if err := validateUserBlockPair(12, 12); err == nil {
		t.Fatal("same owner and blocked user should be rejected")
	}
}
