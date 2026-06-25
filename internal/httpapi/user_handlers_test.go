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

func TestFriendCodeToSteamID64(t *testing.T) {
	if got := friendCodeToSteamID64("1738029940"); got != "76561199698295668" {
		t.Fatalf("friendCodeToSteamID64() = %q, want 76561199698295668", got)
	}
	if got := friendCodeToSteamID64("76561199698295668"); got != "76561199698295668" {
		t.Fatalf("friendCodeToSteamID64(steamID64) = %q, want 76561199698295668", got)
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
