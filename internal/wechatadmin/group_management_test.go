package wechatadmin

import (
	"reflect"
	"strings"
	"testing"
)

func TestUpdateStringListTogglesAndDedupes(t *testing.T) {
	values := []string{"a@chatroom", "b@chatroom", "a@chatroom", ""}
	if got, want := updateStringList(values, "c@chatroom", true), []string{"a@chatroom", "b@chatroom", "c@chatroom"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("enable got %#v, want %#v", got, want)
	}
	if got, want := updateStringList(values, "a@chatroom", false), []string{"b@chatroom"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("disable got %#v, want %#v", got, want)
	}
}

func TestConfigStringListReadsVersionedWxbotConfig(t *testing.T) {
	raw := []byte(`{"schemaVersion":1,"config":{"ai":{"group_whitelist":["a@chatroom",""]}}}`)
	if got, want := configStringList(raw, "ai", "group_whitelist"), []string{"a@chatroom"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestManagedGroupVisibleWhereRequiresGroupName(t *testing.T) {
	if !strings.Contains(managedGroupVisibleWhere, "room_id <> ''") {
		t.Fatalf("visible group condition should reject empty room id: %s", managedGroupVisibleWhere)
	}
	if !strings.Contains(managedGroupVisibleWhere, "TRIM(COALESCE(room_name, '')) <> ''") {
		t.Fatalf("visible group condition should reject empty group names: %s", managedGroupVisibleWhere)
	}
}

func TestManagedGroupMessageStatsQueryOnlyAggregatesRequestedRooms(t *testing.T) {
	query := managedGroupMessageStatsQuery("?,?,?")
	if !strings.Contains(query, "WHERE room_id IN (?,?,?)") {
		t.Fatalf("stats query should be limited to requested rooms: %s", query)
	}
	if strings.Contains(query, "UNION") {
		t.Fatalf("stats query should not union group_info and group_messages: %s", query)
	}
}

func TestManagedGroupOrderByPrioritizesBotWhitelist(t *testing.T) {
	orderBy, args := managedGroupOrderBy(map[string]struct{}{
		"b@chatroom": {},
		"a@chatroom": {},
	})
	if !strings.HasPrefix(orderBy, "CASE WHEN room_id IN (?,?) THEN 0 ELSE 1 END") {
		t.Fatalf("order by should prioritize whitelisted rooms: %s", orderBy)
	}
	if !reflect.DeepEqual(args, []interface{}{"a@chatroom", "b@chatroom"}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestManagedGroupOrderByKeepsSimpleSortWithoutWhitelist(t *testing.T) {
	orderBy, args := managedGroupOrderBy(nil)
	if orderBy != "updated_at DESC, room_id ASC" {
		t.Fatalf("order by = %q", orderBy)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v", args)
	}
}
