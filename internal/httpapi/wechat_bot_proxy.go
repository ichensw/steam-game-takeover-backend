package httpapi

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
)

const (
	wechatBotSecretHeader        = "X-Wechat-Bot-Admin-Secret"
	wechatBotAdminIDHeader       = "X-Wechat-Bot-Admin-ID"
	wechatBotAdminUsernameHeader = "X-Wechat-Bot-Admin-Username"
)

var tablePathPattern = regexp.MustCompile(`^/tables/[A-Za-z0-9_]+(?:/rows)?$`)

func requiredWechatBotMenus(method, path string) ([]string, bool) {
	switch {
	case method == http.MethodGet && path == "/groups":
		return []string{"wechat-messages", "wechat-summary"}, true
	case method == http.MethodGet && path == "/messages":
		return []string{"wechat-messages"}, true
	case method == http.MethodPost && path == "/messages/summary":
		return []string{"wechat-summary"}, true
	case method == http.MethodGet && path == "/stats/daily":
		return []string{"wechat-stats"}, true
	case method == http.MethodGet && (path == "/tables" || tablePathPattern.MatchString(path)):
		return []string{"wechat-database"}, true
	default:
		return nil, false
	}
}

func hasAnyMenu(menuKeys, required []string) bool {
	for _, wanted := range required {
		if containsString(menuKeys, wanted) {
			return true
		}
	}
	return false
}

func (h *Handler) wechatBotAdminAllowed(admin model.AdminUser, required []string) bool {
	if admin.Role == model.AdminRoleSuperAdmin {
		return true
	}
	return h.db != nil && hasAnyMenu(h.adminMenuKeys(admin.Role), required)
}

func (h *Handler) AdminProxyWechatBot(c *gin.Context) {
	path := c.Param("path")
	required, allowed := requiredWechatBotMenus(c.Request.Method, path)
	if !allowed {
		fail(c, http.StatusNotFound, CodeParamInvalid, "wechat bot endpoint not found")
		return
	}
	admin, authenticated := currentAdmin(c)
	if !authenticated || !h.wechatBotAdminAllowed(admin, required) {
		fail(c, http.StatusForbidden, CodeAdminUnauthorized, "permission denied")
		return
	}
	if h.cfg.WechatBotAdminURL == "" || h.cfg.WechatBotSharedSecret == "" {
		fail(c, http.StatusServiceUnavailable, CodeSystemError, "wechat bot gateway is not configured")
		return
	}

	target, err := url.JoinPath(h.cfg.WechatBotAdminURL, path)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "wechat bot gateway URL is invalid")
		return
	}
	if c.Request.URL.RawQuery != "" {
		target += "?" + c.Request.URL.RawQuery
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target, c.Request.Body)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "create wechat bot request failed")
		return
	}
	for _, header := range []string{"Accept", "Content-Type"} {
		if value := c.GetHeader(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	request.Header.Set(wechatBotSecretHeader, h.cfg.WechatBotSharedSecret)
	request.Header.Set(wechatBotAdminIDHeader, strconv.FormatUint(admin.ID, 10))
	request.Header.Set(wechatBotAdminUsernameHeader, admin.Username)

	client := h.wechatBotClient
	if c.Request.Method == http.MethodPost && path == "/messages/summary" {
		client = h.wechatBotSummaryClient
	}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			fail(c, http.StatusGatewayTimeout, CodeSystemError, "wechat bot request timed out")
			return
		}
		fail(c, http.StatusBadGateway, CodeSystemError, "wechat bot service unavailable")
		return
	}
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Status(response.StatusCode)
	_, _ = io.Copy(c.Writer, response.Body)
}

func isTimeoutError(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}
