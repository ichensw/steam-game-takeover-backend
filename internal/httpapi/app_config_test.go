package httpapi

import "testing"

func TestPublishTakeoverAllowed(t *testing.T) {
	tests := []struct {
		name          string
		globalEnabled bool
		whitelisted   bool
		want          bool
	}{
		{"global", true, false, true},
		{"whitelist", false, true, true},
		{"not whitelisted", false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := publishTakeoverAllowed(tt.globalEnabled, tt.whitelisted)
			if got != tt.want {
				t.Fatalf("publishTakeoverAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBoolString(t *testing.T) {
	if boolString(true) != "true" || boolString(false) != "false" {
		t.Fatal("boolString() returned unexpected value")
	}
}

func TestParseDailyTakeoverExpirationDays(t *testing.T) {
	tests := []struct {
		raw  string
		want int
	}{
		{raw: "", want: 10},
		{raw: "abc", want: 10},
		{raw: "0", want: 10},
		{raw: "366", want: 10},
		{raw: "1", want: 1},
		{raw: "365", want: 365},
	}
	for _, tt := range tests {
		if got := parseDailyTakeoverExpirationDays(tt.raw); got != tt.want {
			t.Fatalf("parseDailyTakeoverExpirationDays(%q) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}

func TestParseWechatSummaryMaxMessages(t *testing.T) {
	tests := []struct {
		raw  string
		want int
	}{
		{raw: "", want: 1000},
		{raw: "abc", want: 1000},
		{raw: "0", want: 1000},
		{raw: "10001", want: 1000},
		{raw: "1", want: 1},
		{raw: "10000", want: 10000},
	}
	for _, tt := range tests {
		if got := parseWechatSummaryMaxMessages(tt.raw); got != tt.want {
			t.Fatalf("parseWechatSummaryMaxMessages(%q) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}

func TestValidateDailyTakeoverExpirationDays(t *testing.T) {
	for _, days := range []int{1, 10, 365} {
		if err := validateDailyTakeoverExpirationDays(days); err != nil {
			t.Fatalf("validateDailyTakeoverExpirationDays(%d) returned %v", days, err)
		}
	}
	for _, days := range []int{0, 366} {
		if err := validateDailyTakeoverExpirationDays(days); err == nil {
			t.Fatalf("validateDailyTakeoverExpirationDays(%d) accepted invalid value", days)
		}
	}
}

func TestValidateWechatSummaryMaxMessages(t *testing.T) {
	for _, messages := range []int{1, 1000, 10000} {
		if err := validateWechatSummaryMaxMessages(messages); err != nil {
			t.Fatalf("validateWechatSummaryMaxMessages(%d) returned %v", messages, err)
		}
	}
	for _, messages := range []int{0, 10001} {
		if err := validateWechatSummaryMaxMessages(messages); err == nil {
			t.Fatalf("validateWechatSummaryMaxMessages(%d) accepted invalid value", messages)
		}
	}
}

func TestNormalizeAIExtractProvider(t *testing.T) {
	for input, want := range map[string]string{
		"":       aiExtractProviderGPT,
		"GPT":    aiExtractProviderGPT,
		"openai": aiExtractProviderGPT,
		"豆包":     aiExtractProviderDoubao,
		"doubao": aiExtractProviderDoubao,
	} {
		got, err := normalizeAIExtractProvider(input)
		if err != nil || got != want {
			t.Fatalf("normalizeAIExtractProvider(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := normalizeAIExtractProvider("other"); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

func TestIsDoubaoAPIBaseURL(t *testing.T) {
	if !isDoubaoAPIBaseURL(doubaoAPIBaseURL + "/chat/completions") {
		t.Fatal("expected Ark chat completion URL to resolve as Doubao")
	}
	if isDoubaoAPIBaseURL("https://gpt.example.com/v1") {
		t.Fatal("unexpected GPT endpoint classified as Doubao")
	}
}

func TestInferAIExtractProviderMigratesLegacyArkConfig(t *testing.T) {
	if got := inferAIExtractProvider("", doubaoAPIBaseURL); got != aiExtractProviderDoubao {
		t.Fatalf("legacy Ark config provider = %q, want %q", got, aiExtractProviderDoubao)
	}
	if got := inferAIExtractProvider("gpt", doubaoAPIBaseURL); got != aiExtractProviderGPT {
		t.Fatalf("explicit provider = %q, want %q", got, aiExtractProviderGPT)
	}
}

func TestNormalizeAIExtractModel(t *testing.T) {
	for _, tt := range []struct {
		provider string
		model    string
		want     string
	}{
		{aiExtractProviderGPT, "gpt-5.5", "gpt-5.5"},
		{aiExtractProviderGPT, "", "gpt-5.4-mini"},
		{aiExtractProviderDoubao, "doubao-seed-2-1-turbo-260628", "doubao-seed-2-1-turbo-260628"},
		{aiExtractProviderDoubao, "", "doubao-seed-2-0-mini-260428"},
	} {
		got, err := normalizeAIExtractModel(tt.provider, tt.model)
		if err != nil || got != tt.want {
			t.Fatalf("normalizeAIExtractModel(%q, %q) = %q, %v; want %q", tt.provider, tt.model, got, err, tt.want)
		}
	}
	if _, err := normalizeAIExtractModel(aiExtractProviderDoubao, "gpt-5.5"); err == nil {
		t.Fatal("expected provider/model mismatch to fail")
	}
}
