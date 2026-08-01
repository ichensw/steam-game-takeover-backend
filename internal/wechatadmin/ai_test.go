package wechatadmin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func TestEvidenceMessageIDsCollectsNestedUniqueIDs(t *testing.T) {
	runs := []map[string]interface{}{
		{"resultJson": map[string]interface{}{
			"member_deltas": []interface{}{
				map[string]interface{}{"evidence_msg_ids": []interface{}{"m1", "m2"}},
				map[string]interface{}{"evidenceMsgIds": []interface{}{"m2", "m3"}},
			},
		}},
	}
	got := evidenceMessageIDs(runs)
	want := []string{"m1", "m2", "m3"}
	if len(got) != len(want) {
		t.Fatalf("ids = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %#v, want %#v", got, want)
		}
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

func TestGroupBrainRoutesRequireAdminAuth(t *testing.T) {
	handler := NewServer(Config{GatewaySharedSecret: "test-secret"}, nil)
	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/ai/memory/facts"},
		{http.MethodGet, "/api/ai/memory/relationships"},
		{http.MethodGet, "/api/ai/memory/events"},
		{http.MethodGet, "/api/ai/interventions"},
		{http.MethodGet, "/api/ai/memory/feedbacks"},
		{http.MethodGet, "/api/ai/config"},
		{http.MethodPut, "/api/ai/config"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s = %d, want %d", route.method, route.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestProactiveConfigPayloadUsesDefaultsForInvalidValues(t *testing.T) {
	got := proactiveConfigPayload(map[string]interface{}{
		"proactive_enabled":                   true,
		"proactive_observer_interval_seconds": float64(1),
		"proactive_settle_seconds":            float64(30),
		"proactive_timeout_seconds":           float64(2),
	})
	if got["proactiveEnabled"] != true || got["proactiveObserverIntervalSeconds"] != 60 || got["proactiveSettleSeconds"] != 30 || got["proactiveTimeoutSeconds"] != 45 {
		t.Fatalf("proactive config = %#v", got)
	}
}
