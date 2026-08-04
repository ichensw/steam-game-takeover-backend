package wechatadmin

import "testing"

func TestAIDedupeKeyMatchesWxbotRepository(t *testing.T) {
	start, end := 1.5, 2.5
	got := aiDedupeKey(aiJobInsert{
		RoomID:      "r@chatroom",
		JobType:     "vector_backfill",
		WindowStart: &start,
		WindowEnd:   &end,
	})
	if got == "" || got != aiDedupeKey(aiJobInsert{RoomID: "r@chatroom", JobType: "vector_backfill", WindowStart: &start, WindowEnd: &end}) {
		t.Fatalf("dedupe key must be stable: %s", got)
	}
}

func TestVectorBackfillRequiresTimeWindow(t *testing.T) {
	if !manualAIJobTypes["vector_backfill"] {
		t.Fatal("vector_backfill must be a manual AI job")
	}
	if !aiJobRequiresWindow("vector_backfill") {
		t.Fatal("vector_backfill must require a time window")
	}
}

func TestNormalizeAIResolvedValueIsBoolean(t *testing.T) {
	for _, test := range []struct {
		value interface{}
		want  bool
	}{
		{[]byte("0"), false},
		{[]byte("1"), true},
		{int64(0), false},
		{int64(1), true},
	} {
		got, ok := normalizeAIValue("resolved", test.value).(bool)
		if !ok || got != test.want {
			t.Fatalf("resolved %#v = %#v, want boolean %#v", test.value, got, test.want)
		}
	}
}

func TestValidAIReplyFeedback(t *testing.T) {
	for _, value := range []string{"human", "too_ai", "too_much"} {
		if !validAIReplyFeedback(value) {
			t.Fatalf("feedback %q should be allowed", value)
		}
	}
	if validAIReplyFeedback("ignore") {
		t.Fatal("unexpected feedback should be rejected")
	}
}
