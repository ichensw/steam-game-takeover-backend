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
	Response struct {
		Players []struct {
			SteamID string `json:"steamid"`
		} `json:"players"`
	} `json:"response"`
}

func (h *Handler) validateSteamFriendCode(steamID string) (string, error) {
	var result steamSummaryResponse
	steamID64 := friendCodeToSteamID64(steamID)
	req := resty.New().R().SetQueryParam("steamids", steamID64).SetResult(&result)
	if key := h.steamWebAPIKey(); key != "" {
		req.SetQueryParam("key", key)
	}
	resp, err := req.Get("https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/")
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("steam summary http status %d", resp.StatusCode())
	}
	if len(result.Response.Players) == 0 {
		return "", errSteamFriendCodeInvalid
	}
	return normalizeSteamID64ToFriendCode(result.Response.Players[0].SteamID), nil
}

func friendCodeToSteamID64(steamID string) string {
	steamID = strings.TrimSpace(steamID)
	if len(steamID) >= 17 {
		return steamID
	}
	value, ok := new(big.Int).SetString(steamID, 10)
	if !ok {
		return steamID
	}
	return value.Add(value, big.NewInt(76561197960265728)).String()
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
