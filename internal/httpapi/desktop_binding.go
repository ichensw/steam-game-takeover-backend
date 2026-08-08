package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const desktopBindingTTL = 10 * time.Minute

func (h *Handler) CreateDesktopBinding(c *gin.Context) {
	var req struct {
		DeviceName string `json:"deviceName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}

	sessionID, err := newDesktopBindingSecret()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "create binding failed")
		return
	}
	claimSecret, err := newDesktopBindingSecret()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "create binding failed")
		return
	}

	expiresAt := time.Now().Add(desktopBindingTTL)
	binding := model.DesktopBinding{
		SessionID:       sessionID,
		ClaimSecretHash: desktopBindingSecretHash(claimSecret),
		DeviceName:      desktopDeviceName(req.DeviceName),
		ExpiresAt:       expiresAt,
	}
	if err := h.db.Create(&binding).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "create binding failed")
		return
	}

	ok(c, "created", gin.H{
		"sessionId":   sessionID,
		"claimSecret": claimSecret,
		"bindingUrl":  desktopBindingURL(sessionID),
		"expiresAt":   expiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) ApproveDesktopBinding(c *gin.Context) {
	user, _ := currentUser(c)
	sessionID := strings.TrimSpace(c.Param("sessionId"))
	if sessionID == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "session id is required")
		return
	}

	now := time.Now()
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var binding model.DesktopBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_id = ?", sessionID).First(&binding).Error; err != nil {
			return err
		}
		if !binding.ExpiresAt.After(now) {
			return errDesktopBindingExpired
		}
		if binding.ClaimedAt != nil || binding.UserID != nil {
			return errDesktopBindingUsed
		}
		return tx.Model(&binding).Updates(map[string]any{
			"user_id":      user.ID,
			"approved_at":  now,
			"gmt_modified": now,
		}).Error
	})
	if err != nil {
		switch {
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeParamInvalid, "binding not found")
		case errors.Is(err, errDesktopBindingExpired):
			fail(c, http.StatusGone, CodeParamInvalid, "binding expired")
		case errors.Is(err, errDesktopBindingUsed):
			fail(c, http.StatusConflict, CodeParamInvalid, "binding already used")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "approve binding failed")
		}
		return
	}

	ok(c, "approved", gin.H{"approved": true})
}

func (h *Handler) GetDesktopBinding(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("sessionId"))
	if sessionID == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "session id is required")
		return
	}

	var binding model.DesktopBinding
	if err := h.db.Where("session_id = ?", sessionID).First(&binding).Error; err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "binding not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query binding failed")
		return
	}
	if !binding.ExpiresAt.After(time.Now()) {
		fail(c, http.StatusGone, CodeParamInvalid, "binding expired")
		return
	}
	if binding.ClaimedAt != nil || binding.UserID != nil {
		fail(c, http.StatusConflict, CodeParamInvalid, "binding already used")
		return
	}

	ok(c, "ready", gin.H{
		"deviceName": binding.DeviceName,
		"expiresAt":   binding.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) ClaimDesktopBinding(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("sessionId"))
	var req struct {
		ClaimSecret string `json:"claimSecret"`
	}
	if sessionID == "" || c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.ClaimSecret) == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}

	now := time.Now()
	var token string
	var state string
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var binding model.DesktopBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_id = ?", sessionID).First(&binding).Error; err != nil {
			return err
		}
		if binding.ClaimSecretHash != desktopBindingSecretHash(req.ClaimSecret) {
			return errDesktopBindingSecret
		}
		if !binding.ExpiresAt.After(now) {
			return errDesktopBindingExpired
		}
		if binding.ClaimedAt != nil {
			return errDesktopBindingUsed
		}
		if binding.UserID == nil {
			state = "pending"
			return nil
		}

		expiresAt := now.Add(h.cfg.DesktopTokenTTL)
		device := model.DesktopDevice{
			UserID:     *binding.UserID,
			DeviceName: binding.DeviceName,
			ExpiresAt:  expiresAt,
		}
		if err := tx.Create(&device).Error; err != nil {
			return err
		}
		var err error
		token, err = h.signDesktopToken(device.UserID, device.ID, expiresAt)
		if err != nil {
			return err
		}
		if err := tx.Model(&binding).Updates(map[string]any{
			"claimed_at":   now,
			"gmt_modified": now,
		}).Error; err != nil {
			return err
		}
		state = "approved"
		return nil
	})
	if err != nil {
		switch {
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeParamInvalid, "binding not found")
		case errors.Is(err, errDesktopBindingSecret):
			fail(c, http.StatusUnauthorized, CodeUnauthorized, "invalid claim secret")
		case errors.Is(err, errDesktopBindingExpired):
			fail(c, http.StatusGone, CodeParamInvalid, "binding expired")
		case errors.Is(err, errDesktopBindingUsed):
			fail(c, http.StatusConflict, CodeParamInvalid, "binding already used")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "claim binding failed")
		}
		return
	}
	if state == "pending" {
		ok(c, "pending", gin.H{"state": state})
		return
	}
	ok(c, "claimed", gin.H{"state": state, "token": token})
}

func (h *Handler) RevokeCurrentDesktopDevice(c *gin.Context) {
	deviceID := currentDesktopDeviceID(c)
	if deviceID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "desktop token is required")
		return
	}

	user, _ := currentUser(c)
	now := time.Now()
	result := h.db.Model(&model.DesktopDevice{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", deviceID, user.ID).
		Updates(map[string]any{"revoked_at": now, "gmt_modified": now})
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "revoke device failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "device not found")
		return
	}
	ok(c, "revoked", gin.H{"revoked": true})
}

func newDesktopBindingSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func desktopBindingSecretHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func desktopDeviceName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return "Rabbit Nest Toolbox"
	}
	runes := []rune(name)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return name
}

func desktopBindingURL(sessionID string) string {
	return "https://www.rabbits.ink/desktop-bind?session=" + sessionID
}

var (
	errDesktopBindingExpired = errors.New("desktop binding expired")
	errDesktopBindingUsed    = errors.New("desktop binding used")
	errDesktopBindingSecret  = errors.New("desktop binding secret invalid")
)
