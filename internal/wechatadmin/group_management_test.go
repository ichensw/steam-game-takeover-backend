package wechatadmin

import (
	"reflect"
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
