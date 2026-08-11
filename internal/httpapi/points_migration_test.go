package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestPointsMigrationCreatesRequiredStorage(t *testing.T) {
	content, err := os.ReadFile("../../migrations/049_add_takeover_points.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(content)
	for _, fragment := range []string{
		"ADD COLUMN `points_units`",
		"ADD COLUMN `points_settled_at`",
		"CREATE TABLE IF NOT EXISTS `ttw_user_point_log`",
		"UNIQUE KEY `uk_business_key`",
		"`effective_at` datetime NOT NULL",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
