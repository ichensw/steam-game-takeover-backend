package httpapi

import (
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestTakeoverSecurityTextSkipsBlankDescription(t *testing.T) {
	got := takeoverSecurityText(parsedTakeoverInput{
		Title:       "开黑",
		Description: stringPtr("  "),
	})
	if got != "开黑" {
		t.Fatalf("takeoverSecurityText() = %q, want title only", got)
	}
}

func TestParseSecurityResultStatus(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"pass", `{"errcode":0,"result":{"suggest":"pass","label":100}}`, model.ContentAuditStatusPass},
		{"review", `{"errcode":0,"result":{"suggest":"review","label":100}}`, model.ContentAuditStatusReview},
		{"risky", `{"errcode":0,"result":{"suggest":"risky","label":100}}`, model.ContentAuditStatusRisky},
		{"wechat error", `{"errcode":40001,"errmsg":"bad token"}`, model.ContentAuditStatusError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := parseSecurityResult([]byte(tt.body), 200, false)
			if got != tt.want {
				t.Fatalf("parseSecurityResult() status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSecurityResultRequiresSuggestUnlessAllowed(t *testing.T) {
	body := []byte(`{"errcode":0,"errmsg":"ok"}`)
	if _, got := parseSecurityResult(body, 200, false); got != model.ContentAuditStatusError {
		t.Fatalf("parseSecurityResult() status = %q, want error", got)
	}
	if _, got := parseSecurityResult(body, 200, true); got != model.ContentAuditStatusPass {
		t.Fatalf("parseSecurityResult() status = %q, want pass", got)
	}
}

func TestFindSensitiveWord(t *testing.T) {
	words := []model.SensitiveWord{
		{Word: "  VX  "},
		{Word: ""},
	}
	if got := findSensitiveWord("加我vx领取", words); got != "VX" {
		t.Fatalf("findSensitiveWord() = %q, want VX", got)
	}
	if got := findSensitiveWord("正常内容", words); got != "" {
		t.Fatalf("findSensitiveWord() = %q, want empty", got)
	}
}
