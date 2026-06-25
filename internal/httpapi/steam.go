package httpapi

import (
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
)

var errSteamFriendCodeInvalid = errors.New("steam friend code invalid")

func (h *Handler) validateSteamFriendCode(steamID string) error {
	req := resty.New().R().SetQueryParam("steamid", steamID)
	if key := h.uapiKey(); key != "" {
		req.SetAuthToken(key)
	}
	resp, err := req.Get("https://uapis.cn/api/v1/game/steam/summary")
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		return nil
	case 404:
		return errSteamFriendCodeInvalid
	default:
		return fmt.Errorf("steam summary http status %d", resp.StatusCode())
	}
}
