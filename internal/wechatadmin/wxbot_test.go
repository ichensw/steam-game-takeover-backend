package wechatadmin

import (
	"encoding/json"
	"os"
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
			"takeover_recruitment_enabled": true,
			"proactive_enabled": true,
			"proactive_observer_interval_seconds": 60,
			"proactive_settle_seconds": 90,
			"proactive_timeout_seconds": 45,
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
			"summary_input_token_budget": 0,
			"reply_input_token_budget": 0,
			"memory_input_token_budget": 0,
			"proactive_input_token_budget": 0,
			"proactive_max_jobs_per_scan": 0,
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
	var envelope struct {
		SchemaVersion int                               `json:"schemaVersion"`
		Config        map[string]map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(normalized, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != wxbotConfigSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", envelope.SchemaVersion, wxbotConfigSchemaVersion)
	}
	cfg := envelope.Config
	if got := cfg["ai"]["reply_model"]; got != "gpt-5.4-mini" {
		t.Fatalf("reply_model = %#v, want default", got)
	}
	if got := cfg["ai"]["summary_model"]; got != "gpt-5.4-mini" {
		t.Fatalf("summary_model = %#v, want normalized id", got)
	}
	if got := cfg["ai"]["merge_model"]; got != "gpt-5.5" {
		t.Fatalf("merge_model = %#v, want normalized id", got)
	}
	if got := cfg["ai"]["manual_deep_model"]; got != "gpt-5.2" {
		t.Fatalf("manual_deep_model = %#v, want normalized id", got)
	}
	if got := cfg["ai"]["group_whitelist"].([]interface{})[0]; got != "47759534463@chatroom" {
		t.Fatalf("group_whitelist = %#v, want configured room", got)
	}
	if got := cfg["ai"]["takeover_recruitment_enabled"]; got != true {
		t.Fatalf("takeover_recruitment_enabled = %#v, want true", got)
	}
	if got := cfg["ai"]["proactive_enabled"]; got != true {
		t.Fatalf("proactive_enabled = %#v, want true", got)
	}
	if got := cfg["ai"]["proactive_settle_seconds"]; got != float64(90) {
		t.Fatalf("proactive_settle_seconds = %#v, want 90", got)
	}
	if _, ok := cfg["ai"]["mention_aliases"]; !ok {
		t.Fatal("ai.mention_aliases default missing")
	}
	if got := cfg["ai"]["provider"]; got != aiProviderGPT {
		t.Fatalf("provider = %#v, want %q", got, aiProviderGPT)
	}
	if _, ok := cfg["ai"]["profile_depth"]; ok {
		t.Fatal("profile_depth should not be in normalized config")
	}
	if got := cfg["ai"]["scan_interval_seconds"]; got != float64(300) {
		t.Fatalf("scan_interval_seconds = %#v, want default", got)
	}
	if got := cfg["ai"]["summary_input_token_budget"]; got != float64(12000) {
		t.Fatalf("summary_input_token_budget = %#v, want default", got)
	}
	if _, ok := cfg["summary_reminder"]["jobs"]; !ok {
		t.Fatal("summary_reminder.jobs default missing")
	}
}

func TestNormalizeWxbotConfigConfiguresDoubaoProvider(t *testing.T) {
	normalized, err := normalizeWxbotConfig(json.RawMessage(`{
		"ai": {
			"enabled": true,
			"provider": "doubao",
			"doubao_api_key": "ark-key"
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Config map[string]map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(normalized, &envelope); err != nil {
		t.Fatal(err)
	}
	ai := envelope.Config["ai"]
	if got := ai["provider"]; got != aiProviderDoubao {
		t.Fatalf("provider = %#v, want %q", got, aiProviderDoubao)
	}
	if got := ai["api_base_url"]; got != doubaoAPIBaseURL {
		t.Fatalf("api_base_url = %#v, want %q", got, doubaoAPIBaseURL)
	}
	if got := ai["api_key"]; got != "ark-key" {
		t.Fatalf("api_key = %#v, want active doubao key", got)
	}
	if got := ai["reply_model"]; got != aiProviderModelDefaults[aiProviderDoubao]["reply_model"] {
		t.Fatalf("reply_model = %#v, want doubao default", got)
	}
}

func TestNormalizeWxbotConfigReplacesModelsOutsideSelectedProvider(t *testing.T) {
	normalized, err := normalizeWxbotConfig(json.RawMessage(`{
		"ai": {
			"provider": "doubao",
			"reply_model": "gpt-5.5",
			"summary_model": "doubao-seed-2-1-turbo-260628"
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Config map[string]map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(normalized, &envelope); err != nil {
		t.Fatal(err)
	}
	ai := envelope.Config["ai"]
	if got := ai["reply_model"]; got != aiProviderModelDefaults[aiProviderDoubao]["reply_model"] {
		t.Fatalf("reply_model = %#v, want %q", got, aiProviderModelDefaults[aiProviderDoubao]["reply_model"])
	}
	if got := ai["summary_model"]; got != "doubao-seed-2-1-turbo-260628" {
		t.Fatalf("summary_model = %#v, want selected doubao model", got)
	}
}

func TestNormalizeWxbotConfigAcceptsVersionedPayload(t *testing.T) {
	normalized, err := normalizeWxbotConfig(json.RawMessage(`{"schemaVersion":1,"config":{"bot":{"name":"Bot","group_whitelist":["a@chatroom"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parseBotGroupWhitelist(normalized); !ok {
		t.Fatal("versioned config should still be readable by whitelist parser")
	}
}

func TestNormalizeWxbotConfigRejectsFutureVersion(t *testing.T) {
	_, err := normalizeWxbotConfig(json.RawMessage(`{"schemaVersion":2,"config":{}}`))
	if err == nil {
		t.Fatal("expected future schema version to be rejected")
	}
}

func TestNormalizeWxbotConfigDropsLegacyProfileDepth(t *testing.T) {
	normalized, err := normalizeWxbotConfig(json.RawMessage(`{"ai":{"enabled":false,"profile_depth":"chaos"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Config map[string]map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(normalized, &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope.Config["ai"]["profile_depth"]; ok {
		t.Fatal("legacy profile_depth should be dropped")
	}
}

func TestWxbotConfigSchemaFileMatchesNormalizer(t *testing.T) {
	raw, err := os.ReadFile("../../docs/contracts/wxbot-config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		SchemaVersion int                          `json:"schemaVersion"`
		Sections      map[string]map[string]string `json:"sections"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != wxbotConfigSchemaVersion {
		t.Fatalf("schema file version = %d, want %d", doc.SchemaVersion, wxbotConfigSchemaVersion)
	}
	schema := wxbotConfigSchema()
	if len(doc.Sections) != len(schema) {
		t.Fatalf("schema sections = %d, want %d", len(doc.Sections), len(schema))
	}
	for section, fields := range schema {
		docFields, ok := doc.Sections[section]
		if !ok {
			t.Fatalf("section %s missing from schema file", section)
		}
		if len(docFields) != len(fields) {
			t.Fatalf("section %s fields = %d, want %d", section, len(docFields), len(fields))
		}
		for field, spec := range fields {
			if got := docFields[field]; got != spec.kind {
				t.Fatalf("%s.%s kind = %q, want %q", section, field, got, spec.kind)
			}
		}
	}
}

func TestParseBotGroupWhitelist(t *testing.T) {
	values, ok := parseBotGroupWhitelist([]byte(`{"schemaVersion":1,"config":{"bot":{"group_whitelist":["a@chatroom","b@chatroom"]}}}`))
	if !ok || len(values) != 2 || values[0] != "a@chatroom" || values[1] != "b@chatroom" {
		t.Fatalf("whitelist = %#v, ok=%v", values, ok)
	}
	values, ok = parseBotGroupWhitelist([]byte(`{"bot":{"group_whitelist":[]}}`))
	if !ok || len(values) != 0 {
		t.Fatalf("empty whitelist = %#v, ok=%v", values, ok)
	}
	if _, ok := parseBotGroupWhitelist([]byte(`{"ai":{"group_whitelist":["ai@chatroom"]}}`)); ok {
		t.Fatal("ai whitelist should not be used as bot group whitelist")
	}
}

func TestBotGroupAllowedUsesWhitelistOnly(t *testing.T) {
	allowed := botGroupWhitelistSet([]string{" a@chatroom ", "", "b@chatroom"})
	if !botGroupAllowed("a@chatroom", allowed) || !botGroupAllowed(" b@chatroom ", allowed) {
		t.Fatalf("expected whitelisted rooms to be allowed: %#v", allowed)
	}
	if botGroupAllowed("c@chatroom", allowed) || botGroupAllowed("", allowed) {
		t.Fatalf("non-whitelisted room should not be allowed: %#v", allowed)
	}
}
