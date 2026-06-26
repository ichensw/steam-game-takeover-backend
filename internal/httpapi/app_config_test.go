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
