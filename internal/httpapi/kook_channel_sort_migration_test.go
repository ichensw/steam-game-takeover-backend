package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestKookChannelSortMigrationCreatesConfigAndRunTables(t *testing.T) {
	content, err := os.ReadFile("../../migrations/039_add_kook_channel_sort.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(content))

	for _, fragment := range []string{
		"create table if not exists `ttw_kook_channel_sort_config`",
		"create table if not exists `ttw_kook_channel_sort_run`",
		"`group_ids` json not null",
		"`group_snapshot` longtext not null",
		"`plan_snapshot` longtext not null",
		"unique key `uk_execution_key` (`execution_key`)",
		"`lock_token` varchar(64)",
		"`locked_until` datetime",
		"`status` varchar(32) not null",
		"insert into `ttw_kook_channel_sort_config`",
		"(1, 0, json_array(), 'daily', 0)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
	if strings.Count(sql, "engine=innodb") != 2 {
		t.Fatalf("migration must create two InnoDB tables: %s", sql)
	}
	for _, status := range []string{"planning", "running", "succeeded", "failed", "rollback_failed"} {
		if !strings.Contains(sql, status) {
			t.Fatalf("migration must document status %q", status)
		}
	}
}
