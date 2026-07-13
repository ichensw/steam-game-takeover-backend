package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr                    string
	DBDSN                   string
	JWTSecret               string
	UserTokenTTL            time.Duration
	AdminTokenSecret        string
	AdminTokenTTL           time.Duration
	WXAppID                 string
	WXAppSecret             string
	WXLoginMock             bool
	ReminderTemplateID      string
	ReminderMinutes         int
	ContentSecurityEnabled  bool
	BotQueryEnabled         bool
	BotQuerySteamID         string
	BotQueryNickname        string
	BotQueryGender          uint8
	BotQueryAvatarURL       string
	OSSEndpoint             string
	OSSBucket               string
	OSSAccessKeyID          string
	OSSAccessKeySecret      string
	OSSBaseURL              string
	WechatBotAdminURL       string
	WechatBotSharedSecret   string
	WechatBotProxyTimeout   time.Duration
	WechatBotSummaryTimeout time.Duration
}

func Load() Config {
	loadEnv()

	wxLoginMock := envBool("WX_LOGIN_MOCK", false)

	return Config{
		Addr:                    env("APP_ADDR", ":8081"),
		DBDSN:                   env("DB_DSN", "root:password@tcp(127.0.0.1:3306)/steam_takeover?charset=utf8mb4&parseTime=True&loc=Local"),
		JWTSecret:               env("JWT_SECRET", "change-me-user-token-secret"),
		UserTokenTTL:            durationHours("USER_TOKEN_TTL_HOURS", 24*30),
		AdminTokenSecret:        env("ADMIN_TOKEN_SECRET", "change-me-admin-token-secret"),
		AdminTokenTTL:           durationHours("ADMIN_TOKEN_TTL_HOURS", 2),
		WXAppID:                 env("WX_APP_ID", ""),
		WXAppSecret:             env("WX_APP_SECRET", ""),
		WXLoginMock:             wxLoginMock,
		ReminderTemplateID:      env("TAKEOVER_REMINDER_TEMPLATE_ID", "7ag6n1mjOoMCpyAE0E9SXpx72vwc_dij8HqD9kB-NeY"),
		ReminderMinutes:         intValue("TAKEOVER_REMINDER_MINUTES", 15),
		ContentSecurityEnabled:  envBool("CONTENT_SECURITY_ENABLED", !wxLoginMock),
		BotQueryEnabled:         envBool("BOT_QUERY_ENABLED", true),
		BotQuerySteamID:         env("BOT_QUERY_STEAM_ID", "wechat-bot-query"),
		BotQueryNickname:        env("BOT_QUERY_NICKNAME", "WeChat Bot"),
		BotQueryGender:          uint8Value("BOT_QUERY_GENDER", 1),
		BotQueryAvatarURL:       env("BOT_QUERY_AVATAR_URL", ""),
		OSSEndpoint:             env("OSS_ENDPOINT", ""),
		OSSBucket:               env("OSS_BUCKET", ""),
		OSSAccessKeyID:          env("OSS_ACCESS_KEY_ID", ""),
		OSSAccessKeySecret:      env("OSS_ACCESS_KEY_SECRET", ""),
		OSSBaseURL:              env("OSS_BASE_URL", ""),
		WechatBotAdminURL:       strings.TrimRight(env("WECHAT_BOT_ADMIN_URL", "http://127.0.0.1:8091/api"), "/"),
		WechatBotSharedSecret:   os.Getenv("WECHAT_BOT_GATEWAY_SHARED_SECRET"),
		WechatBotProxyTimeout:   time.Duration(intValue("WECHAT_BOT_PROXY_TIMEOUT_SECONDS", 20)) * time.Second,
		WechatBotSummaryTimeout: time.Duration(intValue("WECHAT_BOT_SUMMARY_TIMEOUT_SECONDS", 75)) * time.Second,
	}
}

func loadEnv() {
	paths := []string{".env"}
	if exePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), ".env"))
	}

	seen := map[string]struct{}{}
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		_ = godotenv.Load(path)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationHours(key string, fallback int) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return time.Duration(fallback) * time.Hour
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return time.Duration(fallback) * time.Hour
	}
	return time.Duration(parsed) * time.Hour
}

func uint8Value(key string, fallback uint8) uint8 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || parsed > 255 {
		return fallback
	}
	return uint8(parsed)
}

func intValue(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
