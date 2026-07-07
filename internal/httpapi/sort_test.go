package httpapi

import (
	"strings"
	"testing"
	"time"

	"steam-game-takeover-backend/internal/model"
)

func TestSortClause(t *testing.T) {
	allowed := map[string]string{"createdAt": "gmt_create", "nickname": "nickname"}
	if got := sortClause("nickname", "asc", allowed, "gmt_create"); got != "nickname ASC" {
		t.Fatalf("sortClause() = %q, want nickname ASC", got)
	}
	if got := sortClause("bad", "desc", allowed, "gmt_create"); got != "gmt_create DESC" {
		t.Fatalf("sortClause() = %q, want fallback DESC", got)
	}
}

func TestWXUserPublishWhitelistSortClause(t *testing.T) {
	got := wxUserPublishWhitelistSortClause("asc")
	if !strings.Contains(got, "ttw_publish_takeover_whitelist") {
		t.Fatalf("sort clause missing whitelist table: %q", got)
	}
	if !strings.HasSuffix(got, "ASC") {
		t.Fatalf("sort clause = %q, want ASC direction", got)
	}
}

func TestTakeoverSoonAcrossMidnight(t *testing.T) {
	now := time.Date(2026, 7, 7, 23, 30, 0, 0, time.Local)
	takeover := model.Takeover{ScheduleType: model.ScheduleDaily, PlayTime: "01:00:00"}
	if !isTakeoverSoon(takeover, now) {
		t.Fatal("daily 01:00 should be soon at 23:30")
	}
}
