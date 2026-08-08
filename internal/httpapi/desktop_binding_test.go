package httpapi

import (
	"testing"
	"time"

	"steam-game-takeover-backend/internal/config"
)

func TestDesktopBindingSecretHashIsStable(t *testing.T) {
	secret := "desktop-binding-secret"
	if got, want := desktopBindingSecretHash(secret), "5a5a4fddb177f3aebc26b151b6b6d9b8320f05658c80469521eb8a5172e592f3"; got != want {
		t.Fatalf("hash = %q, want %q", got, want)
	}
	if desktopBindingSecretHash(secret) == desktopBindingSecretHash(secret+"x") {
		t.Fatal("different secrets must have different hashes")
	}
}

func TestDesktopTokenClaims(t *testing.T) {
	h := &Handler{cfg: config.Config{JWTSecret: "test-secret"}}
	expiresAt := time.Now().Add(time.Hour)
	token, err := h.signDesktopToken(12, 34, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := parseToken(token, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	if claims.TokenType != tokenTypeDesktop || claims.UserID != 12 || claims.DeviceID != 34 {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestKookChannelJumpURL(t *testing.T) {
	got := kookChannelJumpURL("guild 1", "channel/2")
	want := "https://www.kookapp.cn/direct/channel?c=channel%2F2&g=guild+1"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
	if got := kookChannelJumpURL("", "channel"); got != "" {
		t.Fatalf("url without guild = %q", got)
	}
}
