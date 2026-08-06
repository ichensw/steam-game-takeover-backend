package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"steam-game-takeover-backend/internal/config"
	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
)

func TestWechatBotProxyPolicy(t *testing.T) {
	tests := []struct {
		method  string
		path    string
		menus   []string
		allowed bool
	}{
		{http.MethodGet, "/groups", []string{"wechat-messages"}, true},
		{http.MethodGet, "/groups/manage", []string{"wechat-groups", "wechat-wxbot-control"}, true},
		{http.MethodGet, "/groups/manage/123@chatroom/members", []string{"wechat-groups", "wechat-wxbot-control"}, true},
		{http.MethodGet, "/groups/manage/123@chatroom/events", []string{"wechat-groups", "wechat-wxbot-control"}, true},
		{http.MethodPut, "/groups/manage/123@chatroom/whitelist", []string{"wechat-groups", "wechat-wxbot-control"}, true},
		{http.MethodDelete, "/groups/manage/123@chatroom/whitelist", nil, false},
		{http.MethodGet, "/messages", []string{"wechat-messages"}, true},
		{http.MethodGet, "/ai/status", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/jobs", []string{"wechat-ai-memory"}, true},
		{http.MethodGet, "/ai/jobs/12", []string{"wechat-ai-memory"}, true},
		{http.MethodGet, "/ai/history-learning", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/history-learning", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/history-learning/12/pause", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/history-learning/12/resume", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/history-learning/12/cancel", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/history-learning/12/retry", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/errors/12/retry", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/memory/facts", nil, false},
		{http.MethodGet, "/ai/role-card", []string{"wechat-ai-memory"}, true},
		{http.MethodPut, "/ai/role-card", []string{"wechat-ai-memory"}, true},
		{http.MethodGet, "/ai/prompt-instructions", []string{"wechat-ai-memory"}, true},
		{http.MethodPut, "/ai/prompt-instructions", []string{"wechat-ai-memory"}, true},
		{http.MethodGet, "/ai/reply-samples", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/reply-samples", []string{"wechat-ai-memory"}, true},
		{http.MethodDelete, "/ai/reply-samples/12", []string{"wechat-ai-memory"}, true},
		{http.MethodGet, "/ai/reply-conversation-samples", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/reply-conversation-samples", []string{"wechat-ai-memory"}, true},
		{http.MethodDelete, "/ai/reply-conversation-samples/12", []string{"wechat-ai-memory"}, true},
		{http.MethodGet, "/ai/reply-logs", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/reply-logs/12/feedback", []string{"wechat-ai-memory"}, true},
		{http.MethodPost, "/ai/history-learning/12/delete", nil, false},
		{http.MethodDelete, "/ai/jobs/12", nil, false},
		{http.MethodGet, "/stats/daily", []string{"wechat-stats"}, true},
		{http.MethodPost, "/stats/daily", nil, false},
		{http.MethodGet, "/tables", []string{"wechat-database"}, true},
		{http.MethodGet, "/tables/group_messages", []string{"wechat-database"}, true},
		{http.MethodGet, "/tables/group_messages/rows", []string{"wechat-database"}, true},
		{http.MethodDelete, "/tables/group_messages", nil, false},
		{http.MethodGet, "/tables/group-messages", nil, false},
		{http.MethodGet, "/wxbots", []string{"wechat-wxbot-control"}, true},
		{http.MethodGet, "/wxbots/wxbot-01/config", []string{"wechat-wxbot-control"}, true},
		{http.MethodPut, "/wxbots/wxbot-01/config", []string{"wechat-wxbot-control"}, true},
		{http.MethodPost, "/wxbots/wxbot-01/config", nil, false},
		{http.MethodGet, "/wxbot/config", nil, false},
		{http.MethodGet, "/auth/me", nil, false},
	}

	for _, tt := range tests {
		menus, allowed := requiredWechatBotMenus(tt.method, tt.path)
		if allowed != tt.allowed || !sameStrings(menus, tt.menus) {
			t.Fatalf("%s %s: menus = %#v, allowed = %v", tt.method, tt.path, menus, allowed)
		}
	}
}

func TestWechatBotProxyRequiresWechatDB(t *testing.T) {
	h := NewHandler(config.Config{WechatBotSharedSecret: "shared-secret"}, nil)
	rec := proxyRequest(h, http.MethodGet, "/messages", "page=2&keyword=steam", model.AdminUser{ID: 42, Username: "ops", Role: model.AdminRoleSuperAdmin})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured db status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestWechatBotProxyRejectsMissingAdminAndUnknownPath(t *testing.T) {
	h := NewHandler(config.Config{WechatBotSharedSecret: "secret"}, nil)
	if rec := proxyRequest(h, http.MethodGet, "/messages", "", model.AdminUser{}); rec.Code != http.StatusForbidden {
		t.Fatalf("missing admin status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec := proxyRequest(h, http.MethodGet, "/auth/me", "", model.AdminUser{ID: 1, Role: model.AdminRoleSuperAdmin}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestWxbotControlProxyRejectsUnknownPath(t *testing.T) {
	h := NewHandler(config.Config{WechatBotSharedSecret: "token"}, nil)
	if rec := wxbotRequest(h, http.MethodGet, "/unknown", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestWxbotControlProxyRequiresWechatDB(t *testing.T) {
	h := NewHandler(config.Config{WechatBotSharedSecret: "expected"}, nil)
	if rec := wxbotRequest(h, http.MethodGet, "/config", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured db status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestWechatBotAdminConfigUsesWxbotAPIToken(t *testing.T) {
	h := NewHandler(config.Config{WechatBotSharedSecret: "gateway", WxbotAPIToken: "wxbot"}, nil)
	if got := h.wechatBotAdminConfig().WxbotAPIToken; got != "wxbot" {
		t.Fatalf("wxbot api token = %q, want wxbot", got)
	}

	h = NewHandler(config.Config{WechatBotSharedSecret: "gateway"}, nil)
	if got := h.wechatBotAdminConfig().WxbotAPIToken; got != "gateway" {
		t.Fatalf("fallback wxbot api token = %q, want gateway", got)
	}
}

func proxyRequest(h *Handler, method, path, query string, admin model.AdminUser) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, "/api/admin/wechat-bot"+path+"?"+query, nil)
	c.Params = gin.Params{{Key: "path", Value: path}}
	if admin.ID != 0 {
		c.Set(contextAdminKey, admin)
	}
	h.AdminProxyWechatBot(c)
	return rec
}

func wxbotRequest(h *Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, "/api/wxbot"+path, nil)
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Request.Header.Set(wxbotTokenHeader, "token")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "path", Value: path}}
	h.ProxyWechatBotControl(c)
	return rec
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
