package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
		{http.MethodGet, "/groups", []string{"wechat-messages", "wechat-summary"}, true},
		{http.MethodGet, "/messages", []string{"wechat-messages"}, true},
		{http.MethodPost, "/messages/summary", []string{"wechat-summary"}, true},
		{http.MethodGet, "/messages/summary/history", []string{"wechat-summary"}, true},
		{http.MethodGet, "/messages/summary/12", []string{"wechat-summary"}, true},
		{http.MethodGet, "/messages/summary/12/messages", []string{"wechat-summary"}, true},
		{http.MethodGet, "/stats/daily", []string{"wechat-stats"}, true},
		{http.MethodPost, "/stats/daily", nil, false},
		{http.MethodGet, "/tables", []string{"wechat-database"}, true},
		{http.MethodGet, "/tables/group_messages", []string{"wechat-database"}, true},
		{http.MethodGet, "/tables/group_messages/rows", []string{"wechat-database"}, true},
		{http.MethodDelete, "/tables/group_messages", nil, false},
		{http.MethodGet, "/tables/group-messages", nil, false},
		{http.MethodGet, "/auth/me", nil, false},
	}

	for _, tt := range tests {
		menus, allowed := requiredWechatBotMenus(tt.method, tt.path)
		if allowed != tt.allowed || !sameStrings(menus, tt.menus) {
			t.Fatalf("%s %s: menus = %#v, allowed = %v", tt.method, tt.path, menus, allowed)
		}
	}
}

func TestWechatBotProxyForwardsVerifiedIdentityAndQuery(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/messages" || r.URL.RawQuery != "page=2&keyword=steam" {
			t.Fatalf("unexpected upstream URL: %s", r.URL.String())
		}
		if r.Header.Get(wechatBotSecretHeader) != "shared-secret" || r.Header.Get(wechatBotAdminIDHeader) != "42" || r.Header.Get(wechatBotAdminUsernameHeader) != "ops" || r.Header.Get(wechatBotSummaryMaxHeader) != "1000" {
			t.Fatalf("unexpected trusted headers: %#v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"success":true,"data":{"ok":true}}`))
	}))
	defer upstream.Close()

	h := NewHandler(config.Config{
		WechatBotAdminURL:       upstream.URL + "/api",
		WechatBotSharedSecret:   "shared-secret",
		WechatBotProxyTimeout:   time.Second,
		WechatBotSummaryTimeout: time.Second,
	}, nil)
	rec := proxyRequest(h, http.MethodGet, "/messages", "page=2&keyword=steam", model.AdminUser{ID: 42, Username: "ops", Role: model.AdminRoleSuperAdmin})
	if rec.Code != http.StatusAccepted || rec.Header().Get("Content-Type") != "application/json" || rec.Body.String() != `{"success":true,"data":{"ok":true}}` {
		t.Fatalf("unexpected response: status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
}

func TestWechatBotProxyRejectsMissingAdminAndUnknownPath(t *testing.T) {
	h := NewHandler(config.Config{WechatBotAdminURL: "http://127.0.0.1:1/api", WechatBotSharedSecret: "secret", WechatBotProxyTimeout: time.Second}, nil)
	if rec := proxyRequest(h, http.MethodGet, "/messages", "", model.AdminUser{}); rec.Code != http.StatusForbidden {
		t.Fatalf("missing admin status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec := proxyRequest(h, http.MethodGet, "/auth/me", "", model.AdminUser{ID: 1, Role: model.AdminRoleSuperAdmin}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestWechatBotProxyMapsTransportFailureAndTimeout(t *testing.T) {
	failing := NewHandler(config.Config{WechatBotAdminURL: "http://127.0.0.1:1/api", WechatBotSharedSecret: "secret", WechatBotProxyTimeout: 100 * time.Millisecond}, nil)
	if rec := proxyRequest(failing, http.MethodGet, "/messages", "", model.AdminUser{ID: 1, Role: model.AdminRoleSuperAdmin}); rec.Code != http.StatusBadGateway {
		t.Fatalf("transport status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer upstream.Close()
	timed := NewHandler(config.Config{WechatBotAdminURL: upstream.URL + "/api", WechatBotSharedSecret: "secret", WechatBotProxyTimeout: 10 * time.Millisecond}, nil)
	if rec := proxyRequest(timed, http.MethodGet, "/messages", "", model.AdminUser{ID: 1, Role: model.AdminRoleSuperAdmin}); rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("timeout status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
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
