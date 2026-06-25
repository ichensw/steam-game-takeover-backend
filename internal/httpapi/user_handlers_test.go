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

func TestFriendCodeFromSteamID3(t *testing.T) {
	if got := friendCodeFromSteamID3("[U:1:1738029940]", "76561199698295668"); got != "1738029940" {
		t.Fatalf("friendCodeFromSteamID3() = %q, want 1738029940", got)
	}
}

func TestNormalizeSteamID64ToFriendCode(t *testing.T) {
	if got := normalizeSteamID64ToFriendCode("76561199698295668"); got != "1738029940" {
		t.Fatalf("normalizeSteamID64ToFriendCode() = %q, want 1738029940", got)
	}
	if got := normalizeSteamID64ToFriendCode("1738029940"); got != "1738029940" {
		t.Fatalf("normalizeSteamID64ToFriendCode(friend code) = %q, want 1738029940", got)
	}
}
