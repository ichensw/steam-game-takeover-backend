package httpapi

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestToKookChannelList(t *testing.T) {
	var result kookChannelListResponse
	result.Data.Items = append(result.Data.Items, struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Type     int    `json:"type"`
		ParentID string `json:"parent_id"`
		Level    int    `json:"level"`
	}{
		ID:       "1895580130534522",
		Name:     "新人指导处",
		Type:     1,
		ParentID: "7838521948344271",
		Level:    100,
	})
	result.Data.Meta.Page = 1
	result.Data.Meta.PageTotal = 3
	result.Data.Meta.PageSize = 50
	result.Data.Meta.Total = 120

	list, meta := toKookChannelList(result)
	if len(list) != 1 || list[0].ParentID != "7838521948344271" {
		t.Fatalf("unexpected channel list: %#v", list)
	}
	if list[0].Level != 1 || list[0].KookSort != 100 {
		t.Fatalf("unexpected level fields: %#v", list[0])
	}
	if meta["pageTotal"] != 3 || meta["total"] != 120 {
		t.Fatalf("unexpected meta: %#v", meta)
	}
}

func TestFilterKookVoiceChannels(t *testing.T) {
	channels := []kookChannelDTO{
		{ID: "1", Type: 0},
		{ID: "2", Type: 1},
		{ID: "3", Type: 2},
	}
	list := filterKookVoiceChannels(channels)
	if len(list) != 2 || list[0].ID != "1" || list[1].ID != "3" {
		t.Fatalf("unexpected filtered list: %#v", list)
	}
}

func TestCloneKookChannels(t *testing.T) {
	channels := []kookChannelDTO{{ID: "1"}}
	cloned := cloneKookChannels(channels)
	cloned[0].ID = "2"
	if channels[0].ID != "1" {
		t.Fatalf("clone changed source: %#v", channels)
	}
}

func TestCloneGinH(t *testing.T) {
	meta := gin.H{"total": 1}
	cloned := cloneGinH(meta)
	cloned["total"] = 2
	if meta["total"] != 1 {
		t.Fatalf("clone changed source: %#v", meta)
	}
}

func TestToKookChannelTree(t *testing.T) {
	channels := []kookChannelDTO{
		{ID: "1", Name: "游戏区", Type: 0, KookSort: 10},
		{ID: "2", Name: "空分组", Type: 0},
		{ID: "3", Name: "文字频道", Type: 1, ParentID: "1"},
		{ID: "4", Name: "聊天娱乐厅", Type: 2, ParentID: "1", KookSort: 20},
		{ID: "5", Name: "未分组语音", Type: 2, ParentID: "missing", KookSort: 30},
		{ID: "6", Name: "子分组", Type: 0, ParentID: "1", KookSort: 40},
		{ID: "7", Name: "三层语音", Type: 2, ParentID: "6", KookSort: 50},
	}

	tree := toKookChannelTree(channels)
	if len(tree) != 2 {
		t.Fatalf("len(tree) = %d, want 2", len(tree))
	}
	if tree[0].ID != "1" || tree[0].Level != 1 || tree[0].KookSort != 10 {
		t.Fatalf("unexpected group: %#v", tree[0])
	}
	if len(tree[0].Children) != 2 || tree[0].Children[0].ID != "4" || tree[0].Children[0].Level != 2 {
		t.Fatalf("unexpected children: %#v", tree[0].Children)
	}
	if tree[0].Children[1].ID != "6" || tree[0].Children[1].Children[0].Level != 3 {
		t.Fatalf("unexpected nested children: %#v", tree[0].Children[1])
	}
	if tree[1].ID != "5" || tree[1].Level != 1 || tree[1].KookSort != 30 {
		t.Fatalf("ungrouped channel should stay top-level: %#v", tree[1])
	}
}
