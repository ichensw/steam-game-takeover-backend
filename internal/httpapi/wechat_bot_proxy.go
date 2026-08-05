package httpapi

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"steam-game-takeover-backend/internal/model"
	"steam-game-takeover-backend/internal/wechatadmin"

	"github.com/gin-gonic/gin"
)

const (
	wechatBotSecretHeader        = "X-Wechat-Bot-Admin-Secret"
	wechatBotAdminIDHeader       = "X-Wechat-Bot-Admin-ID"
	wechatBotAdminUsernameHeader = "X-Wechat-Bot-Admin-Username"
	wxbotTokenHeader             = "X-Wxbot-Token"
	wechatBotSummaryMaxHeader    = "X-Wechat-Bot-Summary-Max-Messages"
	wechatBotSummaryPromptHeader = "X-Wechat-Bot-Summary-Prompt"
	wechatBotSummaryStyleHeader  = "X-Wechat-Bot-Summary-Style"
	wechatBotSummaryModelHeader  = "X-Wechat-Bot-Summary-Model"
	wechatBotSummaryModelsHeader = "X-Wechat-Bot-Summary-Compare-Models"
	wechatBotSummarySendHeader   = "X-Wechat-Bot-Summary-Auto-Send"
)

var (
	tablePathPattern                     = regexp.MustCompile(`^/tables/[A-Za-z0-9_]+(?:/rows)?$`)
	summaryPathPattern                   = regexp.MustCompile(`^/messages/summary/[0-9]+(?:/messages)?$`)
	summaryJobPathPattern                = regexp.MustCompile(`^/messages/summary-jobs(?:/[0-9]+)?$`)
	groupManagePathPattern               = regexp.MustCompile(`^/groups/manage(?:/[^/]+/(?:members|events|whitelist))?$`)
	wxbotPathPattern                     = regexp.MustCompile(`^/wxbots(?:/[A-Za-z0-9_-]+/config)?$`)
	aiJobPathPattern                     = regexp.MustCompile(`^/ai/jobs/[0-9]+$`)
	aiHistoryActionPattern               = regexp.MustCompile(`^/ai/history-learning/[0-9]+/(pause|resume|cancel|retry)$`)
	aiErrorActionPattern                 = regexp.MustCompile(`^/ai/errors/[0-9]+/(retry|resolve)$`)
	aiReplySamplePathPattern             = regexp.MustCompile(`^/ai/reply-samples/[0-9]+$`)
	aiReplyConversationSamplePathPattern = regexp.MustCompile(`^/ai/reply-conversation-samples/[0-9]+$`)
	aiReplyLogFeedbackPattern            = regexp.MustCompile(`^/ai/reply-logs/[0-9]+/feedback$`)
)

func wxbotControlAllowed(method, path string) bool {
	switch path {
	case "/heartbeat":
		return method == http.MethodPost
	case "/config":
		return method == http.MethodGet
	case "/config/applied":
		return method == http.MethodPost
	case "/ai/history-learning/next":
		return method == http.MethodGet
	default:
		return method == http.MethodPost &&
			strings.HasPrefix(path, "/ai/history-learning/") &&
			strings.HasSuffix(path, "/progress")
	}
}

func requiredWechatBotMenus(method, path string) ([]string, bool) {
	switch {
	case aiWechatBotPathAllowed(method, path):
		return []string{"wechat-ai-memory"}, true
	case method == http.MethodGet && path == "/groups":
		return []string{"wechat-messages", "wechat-summary"}, true
	case (method == http.MethodGet || method == http.MethodPut) && groupManagePathPattern.MatchString(path):
		return []string{"wechat-groups", "wechat-wxbot-control"}, true
	case method == http.MethodGet && path == "/messages":
		return []string{"wechat-messages"}, true
	case method == http.MethodPost && (path == "/messages/summary" || path == "/messages/summary-jobs"):
		return []string{"wechat-summary"}, true
	case method == http.MethodGet && summaryJobPathPattern.MatchString(path):
		return []string{"wechat-summary"}, true
	case method == http.MethodGet && (path == "/messages/summary/history" || summaryPathPattern.MatchString(path)):
		return []string{"wechat-summary"}, true
	case method == http.MethodGet && path == "/stats/daily":
		return []string{"wechat-stats"}, true
	case method == http.MethodGet && (path == "/tables" || tablePathPattern.MatchString(path)):
		return []string{"wechat-database"}, true
	case (method == http.MethodGet || method == http.MethodPut) && wxbotPathPattern.MatchString(path):
		return []string{"wechat-wxbot-control"}, true
	default:
		return nil, false
	}
}

func aiWechatBotPathAllowed(method, path string) bool {
	if method == http.MethodGet {
		switch path {
		case "/ai/status", "/ai/jobs", "/ai/history-learning", "/ai/errors", "/ai/role-card", "/ai/prompt-instructions", "/ai/reply-samples", "/ai/reply-conversation-samples", "/ai/reply-logs":
			return true
		}
		return aiJobPathPattern.MatchString(path) || aiReplySamplePathPattern.MatchString(path) || aiReplyConversationSamplePathPattern.MatchString(path)
	}
	if method == http.MethodPost {
		return path == "/ai/jobs" || path == "/ai/history-learning" || path == "/ai/reply-samples" || path == "/ai/reply-conversation-samples" || aiHistoryActionPattern.MatchString(path) || aiErrorActionPattern.MatchString(path) || aiReplyLogFeedbackPattern.MatchString(path)
	}
	if method == http.MethodPut {
		return path == "/ai/role-card" || path == "/ai/prompt-instructions"
	}
	if method == http.MethodDelete {
		return aiReplySamplePathPattern.MatchString(path) || aiReplyConversationSamplePathPattern.MatchString(path)
	}
	return false
}

func hasAnyMenu(menuKeys, required []string) bool {
	for _, wanted := range required {
		if containsString(menuKeys, wanted) {
			return true
		}
	}
	return false
}

func (h *Handler) ProxyWechatBotControl(c *gin.Context) {
	path := c.Param("path")
	if !wxbotControlAllowed(c.Request.Method, path) {
		fail(c, http.StatusNotFound, CodeParamInvalid, "wxbot endpoint not found")
		return
	}
	h.serveWechatBot(c, c.Request.URL.Path, nil)
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
	h.serveWechatBot(c, "/api"+path, &admin)
}

func (h *Handler) applyWechatBotHeaders(request *http.Request, adminID, username string) {
	request.Header.Set(wechatBotSecretHeader, h.wechatBotGatewaySecret())
	request.Header.Set("Authorization", "Bearer "+h.wechatBotGatewaySecret())
	request.Header.Set(wechatBotAdminIDHeader, adminID)
	request.Header.Set(wechatBotAdminUsernameHeader, username)
	request.Header.Set(wechatBotSummaryMaxHeader, strconv.Itoa(h.wechatSummaryMaxMessages()))
	setHeaderIfNotEmpty(request, wechatBotSummaryPromptHeader, h.wechatSummaryPrompt())
	setHeaderIfNotEmpty(request, wechatBotSummaryStyleHeader, h.wechatSummaryStyle())
	setHeaderIfNotEmpty(request, wechatBotSummaryModelHeader, h.wechatSummaryModel())
	setHeaderIfNotEmpty(request, wechatBotSummaryModelsHeader, h.wechatSummaryCompareModels())
	request.Header.Set(wechatBotSummarySendHeader, strconv.FormatBool(h.wechatSummaryAutoSend()))
}

func setHeaderIfNotEmpty(request *http.Request, header, value string) {
	if value != "" {
		request.Header.Set(header, value)
	}
}

func (h *Handler) serveWechatBot(c *gin.Context, path string, admin *model.AdminUser) {
	if h.wechatBotDB == nil {
		fail(c, http.StatusServiceUnavailable, CodeSystemError, "wechat bot database is not configured")
		return
	}
	request := c.Request.Clone(c.Request.Context())
	request.RequestURI = ""
	request.Header = c.Request.Header.Clone()
	urlCopy := *request.URL
	urlCopy.Path = path
	request.URL = &urlCopy
	if admin != nil {
		h.applyWechatBotHeaders(request, strconv.FormatUint(admin.ID, 10), admin.Username)
	}
	wechatadmin.NewServer(h.wechatBotAdminConfig(), h.wechatBotDB).ServeHTTP(c.Writer, request)
}

func (h *Handler) wechatBotAdminConfig() wechatadmin.Config {
	aiModel := firstNonEmptyString(h.wechatSummaryModel(), h.aiExtractModel(), "gpt-4o-mini")
	aiBaseURL := firstNonEmptyString(h.aiExtractBaseURL(), h.cfg.AIBaseURL, "https://api.openai.com/v1")
	aiAPIKey := firstNonEmptyString(h.aiExtractAPIKey(), h.cfg.AIAPIKey)
	return wechatadmin.Config{
		GatewaySharedSecret: h.wechatBotGatewaySecret(),
		AIAPIKey:            aiAPIKey,
		AIBaseURL:           aiBaseURL,
		AIModel:             aiModel,
		AITimeout:           h.cfg.WechatBotSummaryTimeout,
		SummaryMaxMessages:  h.wechatSummaryMaxMessages(),
		WechatHookAPIURL:    h.cfg.WechatHookAPIURL,
		WechatHookAPIToken:  h.cfg.WechatHookAPIToken,
		WxbotAPIToken:       firstNonEmptyString(h.cfg.WxbotAPIToken, h.cfg.WechatBotSharedSecret),
		Location:            h.cfg.Location,
	}
}

func (h *Handler) wechatBotGatewaySecret() string {
	if secret := strings.TrimSpace(h.cfg.WechatBotSharedSecret); secret != "" {
		return secret
	}
	return "in-process-wechat-bot-admin"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
