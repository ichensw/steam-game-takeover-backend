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
			"reply_enabled": false,
			"api_base_url": "",
			"api_key": "",
			"reply_model": "",
			"reply_context_messages": 0,
			"reply_input_token_budget": 0,
			"worker_queue_size": 0,
			"reply_timeout_seconds": 0
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
	if got := cfg["ai"]["group_whitelist"].([]interface{})[0]; got != "47759534463@chatroom" {
		t.Fatalf("group_whitelist = %#v, want configured room", got)
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
	if _, ok := cfg["summary_reminder"]["jobs"]; !ok {
		t.Fatal("summary_reminder.jobs default missing")
	}
}

func TestNormalizeWxbotConfigAcceptsVectorSettings(t *testing.T) {
	normalized, err := normalizeWxbotConfig(json.RawMessage(`{
		"ai": {
			"vector_enabled": true,
			"vector_qdrant_url": "https://qdrant.example.com/",
			"vector_embedding_base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1/",
			"vector_embedding_model": "qwen3.7-text-embedding",
			"vector_sync_interval_seconds": 60,
			"vector_sync_batch_size": 32,
			"vector_search_limit": 8,
			"vector_min_score": 0.4
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
	if ai["vector_qdrant_url"] != "https://qdrant.example.com/" || ai["vector_min_score"] != float64(0.4) {
		t.Fatalf("vector config = %#v", ai)
	}
}

func TestNormalizeWxbotConfigAcceptsCurrentRobotAIFields(t *testing.T) {
	normalized, err := normalizeWxbotConfig(json.RawMessage(`{
		"ai": {
			"enabled": true,
			"api_base_url": "https://hairfree.work/v1",
			"reply_input_token_budget": 6000,
			"vector_enabled": true,
			"vector_qdrant_url": "https://qdrant.rabbits.ink",
			"vector_embedding_base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1",
			"vector_embedding_model": "qwen3.7-text-embedding",
			"vector_sync_interval_seconds": 60,
			"vector_sync_batch_size": 32,
			"vector_search_limit": 8,
			"vector_min_score": 0.4
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
	if got := envelope.Config["ai"]["reply_input_token_budget"]; got != float64(6000) {
		t.Fatalf("reply_input_token_budget = %#v", got)
	}
}

func TestNormalizeWxbotCurrentConfigFallsBackToEmptyOnInvalidPayload(t *testing.T) {
	currentConfig, hasCurrentConfig, lastConfigError := normalizeWxbotCurrentConfig(
		json.RawMessage(`{"ai":{"unknown_future_field":true}}`),
		"",
	)

	if hasCurrentConfig {
		t.Fatal("invalid current config should not be marked as usable")
	}
	if len(currentConfig) == 0 || !json.Valid(currentConfig) {
		t.Fatalf("fallback current config must be valid JSON: %q", string(currentConfig))
	}
	if lastConfigError == "" {
		t.Fatal("invalid current config should keep a diagnostic error")
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
			"reply_model": "gpt-5.5"
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

func TestVersionedWxbotConfigDropsRemovedLegacyFields(t *testing.T) {
	normalized := versionedWxbotConfigRaw([]byte(`{
		"ai": {
			"enabled": true,
			"api_base_url": "https://example.com/v1",
			"reply_model": "gpt-5.4-mini",
			"summary_model": "gpt-5.4-mini",
			"merge_model": "gpt-5.4-mini",
			"proactive_enabled": true
		}
	}`))
	var envelope struct {
		Config map[string]map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(normalized, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"summary_model", "merge_model", "proactive_enabled"} {
		if _, ok := envelope.Config["ai"][field]; ok {
			t.Fatalf("legacy ai field %q should be removed", field)
		}
	}
	if got := envelope.Config["ai"]["enabled"]; got != true {
		t.Fatalf("enabled = %#v, want true", got)
	}
}

func TestNormalizeWxbotAIStatusRequiresJSONObject(t *testing.T) {
	if got := string(normalizeWxbotAIStatus(json.RawMessage(`[]`))); got != `{}` {
		t.Fatalf("non-object status = %s, want {}", got)
	}
	if got := string(normalizeWxbotAIStatus(json.RawMessage(`{"vector":{"configured":true}}`))); got != `{"vector":{"configured":true}}` {
		t.Fatalf("valid status = %s", got)
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
