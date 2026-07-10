package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestLeaveActivityBackfillMigration(t *testing.T) {
	content, err := os.ReadFile("../../migrations/038_backfill_takeover_member_leave_activity.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	if !strings.Contains(sql, "`action`, `remark`") || !strings.Contains(sql, " 2,") {
		t.Fatal("migration must backfill leave activities with action=2")
	}
	if !strings.Contains(sql, "not exists") || !strings.Contains(sql, "a.`action` = 2") {
		t.Fatal("migration must avoid duplicating existing leave activities")
	}
}
