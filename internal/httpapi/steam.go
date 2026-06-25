package httpapi

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/go-resty/resty/v2"
)

var errSteamFriendCodeInvalid = errors.New("steam friend code invalid")

type steamSummaryResponse struct {
	SteamID3 string `json:"steamid3"`
}

func (h *Handler) validateSteamFriendCode(steamID string) (string, error) {
	var result steamSummaryResponse
	req := resty.New().R().SetQueryParam("steamid", steamID).SetResult(&result)
	if key := h.uapiKey(); key != "" {
		req.SetAuthToken(key)
	}
	resp, err := req.Get("https://uapis.cn/api/v1/game/steam/summary")
	if err != nil {
		return "", err
	}
	switch resp.StatusCode() {
	case 200:
		return friendCodeFromSteamID3(result.SteamID3, steamID), nil
	case 404:
		return "", errSteamFriendCodeInvalid
	default:
		return "", fmt.Errorf("steam summary http status %d", resp.StatusCode())
	}
}

func friendCodeFromSteamID3(steamID3, fallback string) string {
	steamID3 = strings.TrimSpace(steamID3)
	if strings.HasPrefix(steamID3, "[U:1:") && strings.HasSuffix(steamID3, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(steamID3, "[U:1:"), "]")
	}
	return normalizeSteamID64ToFriendCode(fallback)
}

func normalizeSteamID64ToFriendCode(steamID string) string {
	if len(steamID) < 17 {
		return steamID
	}
	value, ok := new(big.Int).SetString(steamID, 10)
	if !ok {
		return steamID
	}
	base := big.NewInt(76561197960265728)
	if value.Cmp(base) <= 0 {
		return steamID
	}
	return value.Sub(value, base).String()
}
