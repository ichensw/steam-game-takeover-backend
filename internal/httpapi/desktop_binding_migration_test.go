package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestDesktopBindingMigrationCreatesRequiredTables(t *testing.T) {
	content, err := os.ReadFile("../../migrations/048_add_desktop_device_binding.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"create table if not exists `ttw_desktop_binding`",
		"`claim_secret_hash` varchar(64) not null",
		"`expires_at` datetime not null",
		"create table if not exists `ttw_desktop_device`",
		"`revoked_at` datetime default null",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration is missing %q", fragment)
		}
	}
}
