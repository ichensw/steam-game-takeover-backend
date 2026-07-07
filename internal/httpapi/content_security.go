package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/go-resty/resty/v2"
)

const (
	contentSceneProfile = 1
	contentScenePost    = 3
)

var errContentSecurityReject = errors.New("content security reject")

type contentSecurityTarget struct {
	User        model.User
	ContentType string
	TargetID    uint64
	Scene       uint8
}

type wxAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type wxSecurityResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  struct {
		Suggest string `json:"suggest"`
		Label   int    `json:"label"`
	} `json:"result"`
}

func (h *Handler) checkTextSecurity(target contentSecurityTarget, content string) error {
	content = strings.TrimSpace(content)
	if content == "" || !h.cfg.ContentSecurityEnabled {
		return nil
	}
	if matched, err := h.matchSensitiveWord(content); err != nil {
		h.writeContentAudit(target, model.ContentAuditStatusError, map[string]interface{}{"error": err.Error()})
		return errContentSecurityReject
	} else if matched != "" {
		h.writeContentAudit(target, model.ContentAuditStatusRisky, map[string]interface{}{"source": "local", "word": matched})
		return errContentSecurityReject
	}
	token, err := h.wechatAccessToken()
	if err != nil {
		h.writeContentAudit(target, model.ContentAuditStatusError, map[string]interface{}{"error": err.Error()})
		return errContentSecurityReject
	}

	req := map[string]interface{}{
		"openid":  target.User.OpenID,
		"scene":   target.Scene,
		"version": 2,
		"content": content,
	}
	resp, err := resty.New().R().
		SetQueryParam("access_token", token).
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post("https://api.weixin.qq.com/wxa/msg_sec_check")
	if err != nil {
		h.writeContentAudit(target, model.ContentAuditStatusError, map[string]interface{}{"error": err.Error()})
		return errContentSecurityReject
	}

	result, status := parseSecurityResult(resp.Body(), resp.StatusCode(), false)
	if isWechatTokenInvalid(result) {
		h.clearWechatAccessToken()
		token, err = h.wechatAccessToken()
		if err != nil {
			h.writeContentAudit(target, model.ContentAuditStatusError, map[string]interface{}{"error": err.Error()})
			return errContentSecurityReject
		}
		resp, err = resty.New().R().
			SetQueryParam("access_token", token).
			SetHeader("Content-Type", "application/json").
			SetBody(req).
			Post("https://api.weixin.qq.com/wxa/msg_sec_check")
		if err != nil {
			h.writeContentAudit(target, model.ContentAuditStatusError, map[string]interface{}{"error": err.Error()})
			return errContentSecurityReject
		}
		result, status = parseSecurityResult(resp.Body(), resp.StatusCode(), false)
	}
	h.writeContentAudit(target, status, result)
	if status != model.ContentAuditStatusPass {
		return errContentSecurityReject
	}
	return nil
}

func (h *Handler) clearWechatAccessToken() {
	h.tokenMu.Lock()
	defer h.tokenMu.Unlock()
	h.wxToken = ""
	h.wxTokenUntil = time.Time{}
}

func isWechatTokenInvalid(result map[string]interface{}) bool {
	errCode, _ := result["errcode"].(float64)
	return int(errCode) == 40001 || int(errCode) == 42001
}

func (h *Handler) checkImageSecurity(target contentSecurityTarget, filename string, data []byte) error {
	if len(data) == 0 || !h.cfg.ContentSecurityEnabled {
		return nil
	}
	token, err := h.wechatAccessToken()
	if err != nil {
		h.writeContentAudit(target, model.ContentAuditStatusError, map[string]interface{}{"error": err.Error()})
		return errContentSecurityReject
	}

	resp, err := resty.New().R().
		SetQueryParam("access_token", token).
		SetFileReader("media", filename, bytes.NewReader(data)).
		Post("https://api.weixin.qq.com/wxa/img_sec_check")
	if err != nil {
		h.writeContentAudit(target, model.ContentAuditStatusError, map[string]interface{}{"error": err.Error()})
		return errContentSecurityReject
	}

	result, status := parseSecurityResult(resp.Body(), resp.StatusCode(), true)
	h.writeContentAudit(target, status, result)
	if status != model.ContentAuditStatusPass {
		return errContentSecurityReject
	}
	return nil
}

func (h *Handler) matchSensitiveWord(content string) (string, error) {
	var words []model.SensitiveWord
	if err := h.db.Where("enabled = ?", true).Find(&words).Error; err != nil {
		return "", err
	}
	return findSensitiveWord(content, words), nil
}

func findSensitiveWord(content string, words []model.SensitiveWord) string {
	content = strings.ToLower(content)
	for _, row := range words {
		word := strings.TrimSpace(row.Word)
		if word != "" && strings.Contains(content, strings.ToLower(word)) {
			return word
		}
	}
	return ""
}

func (h *Handler) wechatAccessToken() (string, error) {
	h.tokenMu.Lock()
	defer h.tokenMu.Unlock()

	if h.wxToken != "" && time.Now().Before(h.wxTokenUntil) {
		return h.wxToken, nil
	}
	if h.cfg.WXAppID == "" || h.cfg.WXAppSecret == "" {
		return "", errors.New("wechat app config missing")
	}

	resp, err := resty.New().R().
		SetQueryParams(map[string]string{
			"grant_type": "client_credential",
			"appid":      h.cfg.WXAppID,
			"secret":     h.cfg.WXAppSecret,
		}).
		Get("https://api.weixin.qq.com/cgi-bin/token")
	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("wechat token http status %d", resp.StatusCode())
	}

	var tokenResp wxAccessTokenResponse
	if err := json.Unmarshal(resp.Body(), &tokenResp); err != nil {
		return "", fmt.Errorf("decode wechat token: %w", err)
	}
	if tokenResp.ErrCode != 0 {
		return "", fmt.Errorf("wechat token error %d: %s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}
	if tokenResp.AccessToken == "" {
		return "", errors.New("wechat access token empty")
	}

	ttl := tokenResp.ExpiresIn - 300
	if ttl <= 0 {
		ttl = 300
	}
	h.wxToken = tokenResp.AccessToken
	h.wxTokenUntil = time.Now().Add(time.Duration(ttl) * time.Second)
	return h.wxToken, nil
}

func parseSecurityResult(body []byte, statusCode int, allowEmptySuggest bool) (map[string]interface{}, string) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{"httpStatus": statusCode, "body": truncateString(string(body), 500)}, model.ContentAuditStatusError
	}
	if statusCode < 200 || statusCode >= 300 {
		result["httpStatus"] = statusCode
		return result, model.ContentAuditStatusError
	}

	errCode, _ := result["errcode"].(float64)
	if errCode != 0 {
		return result, model.ContentAuditStatusError
	}

	nested, _ := result["result"].(map[string]interface{})
	suggest, _ := nested["suggest"].(string)
	switch suggest {
	case model.ContentAuditStatusPass:
		return result, model.ContentAuditStatusPass
	case model.ContentAuditStatusReview:
		return result, model.ContentAuditStatusReview
	case model.ContentAuditStatusRisky:
		return result, model.ContentAuditStatusRisky
	default:
		if allowEmptySuggest && suggest == "" {
			return result, model.ContentAuditStatusPass
		}
		return result, model.ContentAuditStatusError
	}
}

func (h *Handler) writeContentAudit(target contentSecurityTarget, status string, wxResult interface{}) {
	raw, err := json.Marshal(wxResult)
	if err != nil {
		raw = []byte(`{"error":"marshal wx result failed"}`)
	}
	value := string(raw)
	audit := model.ContentAudit{
		UserID:      target.User.ID,
		OpenID:      target.User.OpenID,
		ContentType: target.ContentType,
		TargetID:    target.TargetID,
		Scene:       target.Scene,
		Status:      status,
		WXResult:    &value,
	}
	if err := h.db.Create(&audit).Error; err != nil {
		log.Printf("write content audit failed: %v", err)
	}
}

func takeoverSecurityText(parsed parsedTakeoverInput) string {
	if parsed.Description == nil || strings.TrimSpace(*parsed.Description) == "" {
		return parsed.Title
	}
	return parsed.Title + "\n" + strings.TrimSpace(*parsed.Description)
}
