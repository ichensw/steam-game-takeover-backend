package httpapi

import "testing"

func TestIsDigits(t *testing.T) {
	if !isDigits("76561198000000000") {
		t.Fatal("isDigits() = false, want true")
	}
	for _, value := range []string{"", "abc", "123abc", "１２３"} {
		if isDigits(value) {
			t.Fatalf("isDigits(%q) = true, want false", value)
		}
	}
}
