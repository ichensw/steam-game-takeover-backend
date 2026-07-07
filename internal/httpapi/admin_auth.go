package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (h *Handler) signAdminToken(adminID uint64, tokenID string) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		TokenType: tokenTypeAdmin,
		AdminID:   adminID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.AdminTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.cfg.AdminTokenSecret))
}

func (h *Handler) currentAdminFromRequest(c *gin.Context) (model.AdminUser, error) {
	tokenValue := bearerToken(c)
	if tokenValue == "" {
		return model.AdminUser{}, errors.New("missing token")
	}
	claims, err := parseToken(tokenValue, h.cfg.AdminTokenSecret)
	if err != nil || claims.TokenType != tokenTypeAdmin || claims.AdminID == 0 || claims.ID == "" {
		return model.AdminUser{}, errors.New("invalid token")
	}

	now := time.Now()
	var token model.AdminToken
	if err := h.db.Where("token_id = ? AND admin_user_id = ? AND is_revoked = ? AND expires_at > ?", claims.ID, claims.AdminID, false, now).First(&token).Error; err != nil {
		return model.AdminUser{}, err
	}

	var admin model.AdminUser
	if err := h.db.Where("id = ? AND enabled = ?", claims.AdminID, true).First(&admin).Error; err != nil {
		return model.AdminUser{}, err
	}
	return admin, nil
}

func (h *Handler) AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "username and password are required")
		return
	}

	var admin model.AdminUser
	if err := h.db.Where("username = ? AND enabled = ?", username, true).First(&admin).Error; err != nil {
		fail(c, http.StatusUnauthorized, CodeUnauthorized, "invalid username or password")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		fail(c, http.StatusUnauthorized, CodeUnauthorized, "invalid username or password")
		return
	}

	tokenID, err := randomTokenID()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "token sign failed")
		return
	}
	tokenValue, err := h.signAdminToken(admin.ID, tokenID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "token sign failed")
		return
	}

	now := time.Now()
	expiresAt := now.Add(h.cfg.AdminTokenTTL)
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.AdminToken{
			AdminUserID: admin.ID,
			TokenID:     tokenID,
			ExpiresAt:   expiresAt,
			IsRevoked:   false,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.AdminUser{}).Where("id = ?", admin.ID).Update("last_login_time", now).Error
	}); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "login failed")
		return
	}
	admin.LastLoginTime = &now

	ok(c, "logged in", gin.H{
		"token":     tokenValue,
		"expiresIn": int(h.cfg.AdminTokenTTL.Seconds()),
		"admin":     toAdminUserDTO(admin),
	})
}

func (h *Handler) AdminLogout(c *gin.Context) {
	tokenValue := bearerToken(c)
	claims, err := parseToken(tokenValue, h.cfg.AdminTokenSecret)
	if err != nil || claims.TokenType != tokenTypeAdmin || claims.ID == "" {
		fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	if err := h.db.Model(&model.AdminToken{}).Where("token_id = ?", claims.ID).Update("is_revoked", true).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "logout failed")
		return
	}
	ok(c, "logged out", nil)
}

func (h *Handler) AdminGetMe(c *gin.Context) {
	admin, _ := currentAdmin(c)
	ok(c, "success", toAdminUserDTO(admin))
}

func (h *Handler) AdminUpdateMe(c *gin.Context) {
	admin, _ := currentAdmin(c)
	var req struct {
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatarUrl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	avatarURL := strings.TrimSpace(req.AvatarURL)
	if len([]rune(nickname)) > 64 || len([]rune(avatarURL)) > 255 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid admin profile")
		return
	}
	updates := map[string]interface{}{
		"nickname":   nullableString(nickname),
		"avatar_url": nullableString(avatarURL),
	}
	if err := h.db.Model(&model.AdminUser{}).Where("id = ?", admin.ID).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	admin.Nickname = nullableString(nickname)
	admin.AvatarURL = nullableString(avatarURL)
	ok(c, "saved", toAdminUserDTO(admin))
}

func (h *Handler) AdminUpdateMePassword(c *gin.Context) {
	admin, _ := currentAdmin(c)
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	oldPassword := strings.TrimSpace(req.OldPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if oldPassword == "" || len([]rune(newPassword)) < 6 || len([]rune(newPassword)) > 64 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid admin password")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(oldPassword)); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "old password invalid")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AdminUser{}).Where("id = ?", admin.ID).Update("password_hash", string(hash)).Error; err != nil {
			return err
		}
		return tx.Model(&model.AdminToken{}).Where("admin_user_id = ?", admin.ID).Update("is_revoked", true).Error
	}); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "password updated", nil)
}

func (h *Handler) AdminCreateAdminUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	nickname := strings.TrimSpace(req.Nickname)
	if username == "" || len([]rune(username)) > 64 || password == "" || len([]rune(password)) < 6 || len([]rune(nickname)) > 64 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid admin user")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	admin := model.AdminUser{
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nullableString(nickname),
		Enabled:      true,
	}
	if err := h.db.Create(&admin).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "created", toAdminUserDTO(admin))
}

func (h *Handler) AdminListAdminUsers(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 50 {
		pageSize = 50
	}

	query := h.db.Model(&model.AdminUser{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR nickname LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	query = applySort(query, c.Query("sortField"), c.Query("sortOrder"), adminUserSortFields, "gmt_create")

	var admins []model.AdminUser
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&admins).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]adminUserDTO, 0, len(admins))
	for _, admin := range admins {
		list = append(list, toAdminUserDTO(admin))
	}
	ok(c, "success", gin.H{"page": page, "pageSize": pageSize, "total": total, "list": list})
}

var adminUserSortFields = map[string]string{
	"id":            "id",
	"username":      "username",
	"nickname":      "nickname",
	"enabled":       "enabled",
	"lastLoginTime": "last_login_time",
	"createdAt":     "gmt_create",
}

func randomTokenID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
