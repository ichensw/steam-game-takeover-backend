package wechatadmin

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseStatsRangeDefaultsToToday(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	before := time.Now().In(loc).Format(statsDateLayout)
	start, end, err := parseStatsRange("", "", loc)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().In(loc).Format(statsDateLayout)
	if got := start.In(loc).Format(statsDateLayout); got != before && got != after {
		t.Fatalf("default start = %s, want %s", got, after)
	}
	if end.Sub(start) != 24*time.Hour {
		t.Fatalf("default range = %s, want 24h", end.Sub(start))
	}
}

func TestNormalizeWxbotConfigAcceptsAISection(t *testing.T) {
	raw := json.RawMessage(`{
		"summary_reminder": {"enabled": true},
		"ai": {
			"enabled": false,
			"group_whitelist": ["47759534463@chatroom"],
			"auto_memory_enabled": false,
			"reply_enabled": false,
			"api_base_url": "",
			"api_key": "",
			"reply_model": "",
			"summary_model": "5.4 Mini",
			"merge_model": "5.5",
			"manual_deep_model": "5.6 Luna",
			"scan_interval_seconds": 0,
			"segment_min_messages": 0,
			"segment_quiet_seconds": 0,
			"segment_stale_seconds": 0,
			"profile_min_segments": 0,
			"max_segment_messages": 0,
			"reply_context_messages": 0,
			"worker_queue_size": 0,
			"reply_timeout_seconds": 0,
			"summary_timeout_seconds": 0,
			"merge_timeout_seconds": 0
		}
	}`)
	normalized, err := normalizeWxbotConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]map[string]interface{}
	if err := json.Unmarshal(normalized, &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg["ai"]["reply_model"]; got != "gpt-5.4-mini" {
		t.Fatalf("reply_model = %#v, want default", got)
	}
	if got := cfg["ai"]["summary_model"]; got != "gpt-5.4-mini" {
		t.Fatalf("summary_model = %#v, want normalized id", got)
	}
	if got := cfg["ai"]["merge_model"]; got != "gpt-5.5" {
		t.Fatalf("merge_model = %#v, want normalized id", got)
	}
	if got := cfg["ai"]["manual_deep_model"]; got != "gpt-5.6-luna" {
		t.Fatalf("manual_deep_model = %#v, want normalized id", got)
	}
	if got := cfg["ai"]["group_whitelist"].([]interface{})[0]; got != "47759534463@chatroom" {
		t.Fatalf("group_whitelist = %#v, want configured room", got)
	}
	if got := cfg["ai"]["scan_interval_seconds"]; got != float64(300) {
		t.Fatalf("scan_interval_seconds = %#v, want default", got)
	}
	if _, ok := cfg["summary_reminder"]["jobs"]; !ok {
		t.Fatal("summary_reminder.jobs default missing")
	}
}
