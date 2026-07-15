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
