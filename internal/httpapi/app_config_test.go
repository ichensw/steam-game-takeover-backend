package httpapi

import "testing"

func TestPublishTakeoverAllowed(t *testing.T) {
	tests := []struct {
		name          string
		globalEnabled bool
		steamID       string
		whitelisted   bool
		want          bool
	}{
		{"global", true, "", false, true},
		{"whitelist", false, "76561198000000000", true, true},
		{"not whitelisted", false, "76561198000000000", false, false},
		{"blank steam", false, " ", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := publishTakeoverAllowed(tt.globalEnabled, tt.steamID, tt.whitelisted)
			if got != tt.want {
				t.Fatalf("publishTakeoverAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
