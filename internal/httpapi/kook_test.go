package httpapi

import "testing"

func TestToKookChannelList(t *testing.T) {
	var result kookChannelListResponse
	result.Data.Items = append(result.Data.Items, struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Topic    string `json:"topic"`
		Type     int    `json:"type"`
		ParentID string `json:"parent_id"`
		Level    int    `json:"level"`
	}{
		ID:       "1895580130534522",
		Name:     "新人指导处",
		Topic:    "欢迎新人",
		Type:     1,
		ParentID: "7838521948344271",
		Level:    2,
	})
	result.Data.Meta.Page = 1
	result.Data.Meta.PageTotal = 3
	result.Data.Meta.PageSize = 50
	result.Data.Meta.Total = 120

	list, meta := toKookChannelList(result)
	if len(list) != 1 || list[0].ParentID != "7838521948344271" {
		t.Fatalf("unexpected channel list: %#v", list)
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

func TestToKookChannelTree(t *testing.T) {
	channels := []kookChannelDTO{
		{ID: "1", Name: "游戏区", Type: 0},
		{ID: "2", Name: "空分组", Type: 0},
		{ID: "3", Name: "文字频道", Type: 1, ParentID: "1"},
		{ID: "4", Name: "聊天娱乐厅", Type: 2, ParentID: "1"},
		{ID: "5", Name: "未分组语音", Type: 2, ParentID: "missing"},
	}

	tree := toKookChannelTree(channels)
	if len(tree) != 2 {
		t.Fatalf("len(tree) = %d, want 2", len(tree))
	}
	if tree[0].ID != "5" {
		t.Fatalf("ungrouped channel should stay top-level: %#v", tree[0])
	}
	if tree[1].ID != "1" || len(tree[1].Children) != 1 || tree[1].Children[0].ID != "4" {
		t.Fatalf("unexpected children: %#v", tree[0].Children)
	}
}
