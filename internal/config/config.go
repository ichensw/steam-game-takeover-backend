package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr             string
	DBDSN            string
	JWTSecret        string
	UserTokenTTL     time.Duration
	AdminPassword    string
	AdminTokenSecret string
	AdminTokenTTL    time.Duration
	WXAppID          string
	WXAppSecret      string
	WXLoginMock      bool
}

func Load() Config {
	return Config{
		Addr:             env("APP_ADDR", ":8080"),
		DBDSN:            env("DB_DSN", "root:password@tcp(127.0.0.1:3306)/steam_takeover?charset=utf8mb4&parseTime=True&loc=Local"),
		JWTSecret:        env("JWT_SECRET", "change-me-user-token-secret"),
		UserTokenTTL:     durationHours("USER_TOKEN_TTL_HOURS", 24*30),
		AdminPassword:    env("ADMIN_PASSWORD", ""),
		AdminTokenSecret: env("ADMIN_TOKEN_SECRET", "change-me-admin-token-secret"),
		AdminTokenTTL:    durationHours("ADMIN_TOKEN_TTL_HOURS", 2),
		WXAppID:          env("WX_APP_ID", ""),
		WXAppSecret:      env("WX_APP_SECRET", ""),
		WXLoginMock:      envBool("WX_LOGIN_MOCK", false),
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
