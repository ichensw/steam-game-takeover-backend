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
