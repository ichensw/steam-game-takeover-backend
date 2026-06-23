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
	contextUserKey = "current_user"
	tokenTypeUser  = "user"
	tokenTypeAdmin = "admin"
)

var errTokenUserMissing = errors.New("token user missing")

type tokenClaims struct {
	TokenType string `json:"typ"`
	UserID    uint64 `json:"uid,omitempty"`
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

func (h *Handler) signAdminToken() (string, error) {
	now := time.Now()
	claims := tokenClaims{
		TokenType: tokenTypeAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.AdminTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.AdminTokenSecret))
}

func (h *Handler) UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, authErr := h.currentUserFromRequest(c)
		if errors.Is(authErr, errTokenUserMissing) {
			fail(c, http.StatusForbidden, CodeProfileIncomplete, "profile incomplete")
			c.Abort()
			return
		}
		if authErr != nil {
			fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Set(contextUserKey, user)
		c.Next()
	}
}

func (h *Handler) AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, err := h.currentUserFromRequest(c); err == nil {
			if user.IsAdmin {
				c.Set(contextUserKey, user)
				c.Next()
				return
			}
			fail(c, http.StatusForbidden, CodeAdminUnauthorized, "admin unauthorized")
			c.Abort()
			return
		}

		tokenValue := bearerToken(c)
		if tokenValue == "" {
			fail(c, http.StatusUnauthorized, CodeAdminUnauthorized, "admin unauthorized")
			c.Abort()
			return
		}
		claims, err := parseToken(tokenValue, h.cfg.AdminTokenSecret)
		if err != nil || claims.TokenType != tokenTypeAdmin {
			fail(c, http.StatusUnauthorized, CodeAdminUnauthorized, "invalid admin token")
			c.Abort()
			return
		}
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
