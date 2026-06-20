package httpapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

type wxSession struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (h *Handler) codeToSession(code string) (wxSession, error) {
	if h.cfg.WXLoginMock {
		openID := strings.TrimSpace(code)
		if openID == "" {
			return wxSession{}, errors.New("empty mock openid")
		}
		return wxSession{OpenID: "mock_" + openID}, nil
	}
	if h.cfg.WXAppID == "" || h.cfg.WXAppSecret == "" {
		return wxSession{}, errors.New("wechat app config missing")
	}

	var session wxSession
	resp, err := resty.New().R().
		SetQueryParams(map[string]string{
			"appid":      h.cfg.WXAppID,
			"secret":     h.cfg.WXAppSecret,
			"js_code":    code,
			"grant_type": "authorization_code",
		}).
		SetResult(&session).
		Get("https://api.weixin.qq.com/sns/jscode2session")
	if err != nil {
		return wxSession{}, err
	}
	if resp.IsError() {
		return wxSession{}, fmt.Errorf("wechat http status %d", resp.StatusCode())
	}
	if session.ErrCode != 0 {
		return wxSession{}, fmt.Errorf("wechat error %d: %s", session.ErrCode, session.ErrMsg)
	}
	if session.OpenID == "" {
		return wxSession{}, errors.New("wechat openid empty")
	}
	return session, nil
}
