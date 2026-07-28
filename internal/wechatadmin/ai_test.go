package wechatadmin

import "testing"

func TestAIDedupeKeyMatchesWxbotRepository(t *testing.T) {
	start, end := 1.5, 2.5
	got := aiDedupeKey(aiJobInsert{
		RoomID:      "r@chatroom",
		JobType:     "segment_summary",
		WindowStart: &start,
		WindowEnd:   &end,
	})
	const want = "7b1866bf0dcac027bfe3c01c0c1327a89a94db85ac92d26924f282dc160049dc"
	if got != want {
		t.Fatalf("dedupe key = %s, want %s", got, want)
	}
}
