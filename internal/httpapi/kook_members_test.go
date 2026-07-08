package httpapi

import (
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestParseKookTimeValue(t *testing.T) {
	got := parseKookTimeValue("2026-07-08 10:00:00")
	if got == nil || got.Format("2006-01-02 15:04:05") != "2026-07-08 10:00:00" {
		t.Fatalf("parseKookTimeValue(datetime) = %v", got)
	}

	got = parseKookTimeValue(float64(1783485600000))
	if got == nil || got.Unix() != 1783485600 {
		t.Fatalf("parseKookTimeValue(ms) unix = %v, want 1783485600", got)
	}
}

func TestNormalizeKookMemberInput(t *testing.T) {
	member, err := normalizeKookMemberInput(kookMemberInput{
		GuildID:    " guild-1 ",
		KookUserID: " user-1 ",
		Username:   " abc ",
	}, true, model.KookMemberStatusJoined)
	if err != nil {
		t.Fatalf("normalizeKookMemberInput() error = %v", err)
	}
	if member.GuildID != "guild-1" || member.KookUserID != "user-1" || member.MemberStatus != model.KookMemberStatusJoined {
		t.Fatalf("normalizeKookMemberInput() = %+v", member)
	}
}

func TestKookWebhookMemberUpdatesKeepsBotWhenAbsent(t *testing.T) {
	member := kookMemberFromWebhook(map[string]interface{}{
		"extra": map[string]interface{}{
			"body": map[string]interface{}{
				"guild_id": "guild-1",
				"user_id":  "user-1",
				"nickname": "abc",
			},
		},
	})
	updates := kookWebhookMemberUpdates(map[string]interface{}{}, member)
	if _, ok := updates["is_bot"]; ok {
		t.Fatal("is_bot update exists when webhook payload did not include bot field")
	}
}

func TestKookSystemEventUsesExtraTypeAndBodyUser(t *testing.T) {
	payload := map[string]interface{}{
		"d": map[string]interface{}{
			"type":      float64(255),
			"target_id": "guild-1",
			"extra": map[string]interface{}{
				"type": "joined_guild",
				"body": map[string]interface{}{
					"user_id":   "user-1",
					"joined_at": float64(1783485600000),
				},
			},
		},
	}
	if got := kookEventType(payload); got != "joined_guild" {
		t.Fatalf("kookEventType() = %q, want joined_guild", got)
	}
	member := kookMemberFromWebhook(payload)
	if member.GuildID != "guild-1" || member.KookUserID != "user-1" {
		t.Fatalf("kookMemberFromWebhook() = %+v", member)
	}
}

func TestKookAdminErrorMessageShowsPermissionHint(t *testing.T) {
	got := kookAdminErrorMessage("拉黑", kookAPIError{
		HTTPStatus: 200,
		Code:       40000,
		Message:    "target_id不存在或者你没有权限操作",
	})
	want := "KOOK 拉黑失败：用户不存在，或机器人没有权限操作该用户。请检查机器人是否有封禁用户权限，并确认机器人角色高于目标用户。"
	if got != want {
		t.Fatalf("kookAdminErrorMessage() = %q, want %q", got, want)
	}
}
