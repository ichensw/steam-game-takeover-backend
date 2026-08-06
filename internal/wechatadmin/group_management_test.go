package wechatadmin

import (
	"database/sql"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestManagedGroupOrderByPrioritizesCombinedWhitelists(t *testing.T) {
	orderBy, args := managedGroupOrderBy(
		map[string]struct{}{
			"b@chatroom": {},
			"a@chatroom": {},
		},
		map[string]struct{}{
			"c@chatroom": {},
			"a@chatroom": {},
		},
	)
	if !strings.Contains(orderBy, "WHEN room_id IN (?,?) AND room_id IN (?,?) THEN 0") {
		t.Fatalf("order by should prioritize groups in both whitelists: %s", orderBy)
	}
	if !strings.Contains(orderBy, "WHEN room_id IN (?,?) THEN 1") {
		t.Fatalf("order by should keep bot-only groups before ai-only groups: %s", orderBy)
	}
	if !strings.Contains(orderBy, "WHEN room_id IN (?,?) THEN 2") {
		t.Fatalf("order by should include ai-only groups before unlisted groups: %s", orderBy)
	}
	if !reflect.DeepEqual(args, []interface{}{
		"a@chatroom", "b@chatroom",
		"a@chatroom", "c@chatroom",
		"a@chatroom", "b@chatroom",
		"a@chatroom", "c@chatroom",
	}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestManagedGroupOrderByKeepsSimpleSortWithoutWhitelist(t *testing.T) {
	orderBy, args := managedGroupOrderBy(nil, nil)
	if orderBy != "updated_at DESC, room_id ASC" {
		t.Fatalf("order by = %q", orderBy)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v", args)
	}
}

func TestGroupMemberItemsKeepsResponseFields(t *testing.T) {
	items := groupMemberItems([]groupMemberRow{{
		MemberWxid:     "wxid_a",
		DisplayName:    "阿白",
		MessageCount:   7,
		FirstMessageAt: 1700000000,
		LastMessageAt:  1700003600,
	}}, time.UTC)
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
	if items[0]["memberWxid"] != "wxid_a" || items[0]["displayName"] != "阿白" || items[0]["messageCount"] != 7 {
		t.Fatalf("item fields = %#v", items[0])
	}
	if _, ok := items[0]["firstMessageAt"].(map[string]interface{}); !ok {
		t.Fatalf("firstMessageAt should use unixJSON: %#v", items[0]["firstMessageAt"])
	}
}

func TestGroupMemberSearchKeywordAcceptsKeywordSearchAndQ(t *testing.T) {
	req := httptest.NewRequest("GET", "/?search=%20%E6%A9%99%E5%AD%90%20", nil)
	if got := groupMemberSearchKeyword(req); got != "橙子" {
		t.Fatalf("search keyword = %q", got)
	}
	req = httptest.NewRequest("GET", "/?q=wxid_a", nil)
	if got := groupMemberSearchKeyword(req); got != "wxid_a" {
		t.Fatalf("q keyword = %q", got)
	}
	req = httptest.NewRequest("GET", "/?keyword=display", nil)
	if got := groupMemberSearchKeyword(req); got != "display" {
		t.Fatalf("keyword = %q", got)
	}
}

func TestGroupMemberCountQueryAddsSearch(t *testing.T) {
	query, args := groupMemberCountQuery("room@chatroom", "橙_子")
	if !strings.Contains(query, "COUNT(DISTINCT sender_wxid)") {
		t.Fatalf("count query should count distinct members: %s", query)
	}
	if !strings.Contains(query, "sender_wxid LIKE ? ESCAPE '\\\\'") || !strings.Contains(query, "sender_name LIKE ? ESCAPE '\\\\'") {
		t.Fatalf("count query should search wxid and names: %s", query)
	}
	if !reflect.DeepEqual(args, []interface{}{"room@chatroom", "%橙\\_子%", "%橙\\_子%"}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestGroupMemberRowsQuerySearchesMatchedMembersAndKeepsFullStats(t *testing.T) {
	query, args := groupMemberRowsQuery("room@chatroom", "橙子", 20, 40)
	if !strings.Contains(query, "JOIN (") || !strings.Contains(query, "SELECT DISTINCT sender_wxid") {
		t.Fatalf("search rows query should first match member ids: %s", query)
	}
	if !strings.Contains(query, "COUNT(*) AS message_count") || !strings.Contains(query, "MAX(gm.created_at) AS last_message_at") {
		t.Fatalf("search rows query should aggregate all member messages after matching: %s", query)
	}
	if !reflect.DeepEqual(args, []interface{}{"room@chatroom", "%橙子%", "%橙子%", "room@chatroom", 20, 40}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestGroupMemberEventQueriesSearchWxidEventNameAndMessageNames(t *testing.T) {
	countQuery, countArgs := groupMemberEventCountQuery("room@chatroom", "橙子")
	if !strings.Contains(countQuery, "COUNT(*)") || !strings.Contains(countQuery, "e.member_wxid LIKE ? ESCAPE '\\\\'") || !strings.Contains(countQuery, "e.member_name LIKE ? ESCAPE '\\\\'") {
		t.Fatalf("event count query should search wxid and event member name: %s", countQuery)
	}
	if !strings.Contains(countQuery, "FROM group_messages") || !strings.Contains(countQuery, "sender_name LIKE ? ESCAPE '\\\\'") {
		t.Fatalf("event count query should include known message names: %s", countQuery)
	}
	if !reflect.DeepEqual(countArgs, []interface{}{"room@chatroom", "%橙子%", "%橙子%", "room@chatroom", "%橙子%"}) {
		t.Fatalf("count args = %#v", countArgs)
	}

	listQuery, listArgs := groupMemberEventRowsQuery("room@chatroom", "橙子", 20, 0)
	if !strings.Contains(listQuery, "ORDER BY e.created_at DESC") {
		t.Fatalf("event list query should keep created_at sort: %s", listQuery)
	}
	if !reflect.DeepEqual(listArgs, []interface{}{"room@chatroom", "%橙子%", "%橙子%", "room@chatroom", "%橙子%", 20, 0}) {
		t.Fatalf("list args = %#v", listArgs)
	}
}

func TestGroupMemberEventItemsKeepsResponseFields(t *testing.T) {
	items := groupMemberEventItems([]groupMemberEventRow{{
		ID:          9,
		RoomID:      "room@chatroom",
		RoomName:    "测试群",
		Action:      "join",
		MemberWxid:  "wxid_b",
		MemberName:  "阿蓝",
		MemberCount: sqlNullInt64(23),
		RawPayload:  sqlNullString(`{"ok":true}`),
		CreatedAt:   1700000000,
	}}, time.UTC)
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
	if items[0]["id"] != int64(9) || items[0]["memberCount"] != int64(23) || items[0]["rawPayload"] != `{"ok":true}` {
		t.Fatalf("item fields = %#v", items[0])
	}
}

func sqlNullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: true}
}

func sqlNullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}
