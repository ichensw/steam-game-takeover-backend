package httpapi

import "testing"

func TestToKookChannelList(t *testing.T) {
	var result kookChannelListResponse
	result.Data.Items = append(result.Data.Items, struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Topic    string `json:"topic"`
		ParentID string `json:"parent_id"`
		Level    int    `json:"level"`
	}{
		ID:       "1895580130534522",
		Name:     "新人指导处",
		Topic:    "欢迎新人",
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
