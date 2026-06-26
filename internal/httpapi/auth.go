package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const (
	contextUserKey  = "current_user"
	contextAdminKey = "current_admin"
	tokenTypeUser   = "user"
	tokenTypeAdmin  = "admin"
)

var errTokenUserMissing = errors.New("token user missing")

type tokenClaims struct {
	TokenType string `json:"typ"`
	UserID    uint64 `json:"uid,omitempty"`
	AdminID   uint64 `json:"aid,omitempty"`
	jwt.RegisteredClaims
}

func (h *Handler) signUserToken(userID uint64) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		TokenType: tokenTypeUser,
		UserID:    userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.UserTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.JWTSecret))
}

func (h *Handler) UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, authErr := h.currentUserFromRequest(c)
		if authErr != nil {
			fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		if user.IsBanned {
			fail(c, http.StatusForbidden, CodeUserBanned, "user banned")
			c.Abort()
			return
		}
		c.Set(contextUserKey, user)
		c.Next()
	}
}

func (h *Handler) OptionalUserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if bearerToken(c) == "" {
			c.Next()
			return
		}
		user, err := h.currentUserFromRequest(c)
		if err == nil {
			c.Set(contextUserKey, user)
		}
		c.Next()
	}
}

func (h *Handler) AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, err := h.currentAdminFromRequest(c)
		if err != nil {
			fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Set(contextAdminKey, admin)
		c.Next()
	}
}

func (h *Handler) currentUserFromRequest(c *gin.Context) (model.User, error) {
	tokenValue := bearerToken(c)
	if tokenValue == "" {
		return model.User{}, errors.New("missing token")
	}
	claims, err := parseToken(tokenValue, h.cfg.JWTSecret)
	if err != nil || claims.TokenType != tokenTypeUser || claims.UserID == 0 {
		return model.User{}, errors.New("invalid token")
	}
	var user model.User
	if err := h.db.Where("id = ? AND is_deleted = ?", claims.UserID, false).First(&user).Error; err != nil {
		if isNotFound(err) {
			return model.User{}, errTokenUserMissing
		}
		return model.User{}, err
	}
	return user, nil
}

func currentUser(c *gin.Context) (model.User, bool) {
	user, okAuth := c.Get(contextUserKey)
	if !okAuth {
		return model.User{}, false
	}
	typed, okAuth := user.(model.User)
	return typed, okAuth
}

func currentAdmin(c *gin.Context) (model.AdminUser, bool) {
	admin, okAuth := c.Get(contextAdminKey)
	if !okAuth {
		return model.AdminUser{}, false
	}
	typed, okAuth := admin.(model.AdminUser)
	return typed, okAuth
}

func parseToken(tokenValue, secret string) (*tokenClaims, error) {
	claims := &tokenClaims{}
	parsed, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
