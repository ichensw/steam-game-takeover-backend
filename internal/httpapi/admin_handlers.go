package httpapi

import (
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *Handler) AdminLogin(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "password is required")
		return
	}
	if h.cfg.AdminPassword == "" {
		fail(c, http.StatusInternalServerError, CodeSystemError, "admin password is not configured")
		return
	}
	if req.Password != h.cfg.AdminPassword {
		fail(c, http.StatusUnauthorized, CodeAdminPasswordInvalid, "invalid admin password")
		return
	}

	token, err := h.signAdminToken()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "token sign failed")
		return
	}
	_ = h.writeAdminLog("ADMIN_LOGIN", "admin", 0, nil)
	ok(c, "logged in", gin.H{
		"adminToken": token,
		"expiresIn":  int(h.cfg.AdminTokenTTL.Seconds()),
	})
}

func (h *Handler) AdminUpdateTakeover(c *gin.Context) {
	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	var req takeoverInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	parsed, err := validateTakeoverInput(req, false)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}

	var joinedCount int64
	if err := h.db.Model(&model.TakeoverMember{}).
		Where("takeover_id = ? AND member_state = ?", takeoverID, model.MemberStateJoined).
		Count(&joinedCount).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if uint(joinedCount) > parsed.ParticipantLimit {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "participantLimit cannot be lower than joinedCount")
		return
	}

	result := h.db.Model(&model.Takeover{}).
		Where("id = ? AND is_deleted = ?", takeoverID, false).
		Updates(map[string]interface{}{
			"title":             parsed.Title,
			"participant_limit": parsed.ParticipantLimit,
			"schedule_type":     parsed.ScheduleType,
			"start_date":        parsed.StartDate,
			"end_date":          parsed.EndDate,
			"play_time":         parsed.PlayTime,
			"description":       parsed.Description,
		})
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		return
	}
	content := "update takeover: " + parsed.Title
	_ = h.writeAdminLog("TAKEOVER_UPDATE", "takeover", takeoverID, &content)
	ok(c, "saved", nil)
}

func (h *Handler) AdminDeleteTakeover(c *gin.Context) {
	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	result := h.db.Model(&model.Takeover{}).
		Where("id = ? AND is_deleted = ?", takeoverID, false).
		Update("is_deleted", true)
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "delete failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		return
	}
	_ = h.writeAdminLog("TAKEOVER_DELETE", "takeover", takeoverID, nil)
	ok(c, "deleted", nil)
}

func (h *Handler) AdminBlockUser(c *gin.Context) {
	userID, okID := pathUint64(c, "userId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 255 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "reason must be at most 255 characters")
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		block := model.BlockUser{
			UserID:       user.ID,
			OpenID:       user.OpenID,
			NicknameSnap: user.Nickname,
			SteamIDSnap:  user.SteamID,
			BlockReason:  stringPtr(reason),
			IsDeleted:    false,
		}
		now := time.Now()
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"openid":            user.OpenID,
				"nickname_snapshot": user.Nickname,
				"steam_id_snapshot": user.SteamID,
				"block_reason":      stringPtr(reason),
				"is_deleted":        false,
				"gmt_modified":      now,
			}),
		}).Create(&block).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Update("is_blocked", true).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminOperateLog{
			OperateType:    "USER_BLOCK",
			TargetType:     "user",
			TargetID:       user.ID,
			OperateContent: stringPtr(reason),
		}).Error
	})
	if err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "block failed")
		return
	}
	ok(c, "blocked", nil)
}

func (h *Handler) AdminUnblockUser(c *gin.Context) {
	userID, okID := pathUint64(c, "userId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BlockUser{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
			"is_deleted": true,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("is_blocked", false).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminOperateLog{
			OperateType: "USER_UNBLOCK",
			TargetType:  "user",
			TargetID:    user.ID,
		}).Error
	})
	if err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "unblock failed")
		return
	}
	ok(c, "unblocked", nil)
}

func (h *Handler) AdminBlockedUsers(c *gin.Context) {
	type blockedUserDTO struct {
		UserID    uint64 `json:"userId"`
		OpenID    string `json:"openid"`
		Nickname  string `json:"nickname"`
		SteamID   string `json:"steamId"`
		Reason    string `json:"reason"`
		BlockedAt string `json:"blockedAt"`
	}
	type blockedUserRow struct {
		UserID    uint64
		OpenID    string
		Nickname  *string
		SteamID   *string
		Reason    *string
		BlockedAt time.Time
	}

	var rows []blockedUserRow
	if err := h.db.Table("ttw_block_user AS b").
		Select("b.user_id, b.openid, b.nickname_snapshot AS nickname, b.steam_id_snapshot AS steam_id, b.block_reason AS reason, b.gmt_create AS blocked_at").
		Where("b.is_deleted = ?", false).
		Order("b.gmt_create DESC").
		Scan(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	list := make([]blockedUserDTO, 0, len(rows))
	for _, row := range rows {
		list = append(list, blockedUserDTO{
			UserID:    row.UserID,
			OpenID:    row.OpenID,
			Nickname:  stringValue(row.Nickname),
			SteamID:   stringValue(row.SteamID),
			Reason:    stringValue(row.Reason),
			BlockedAt: row.BlockedAt.Format("2006-01-02 15:04:05"),
		})
	}
	ok(c, "success", gin.H{"list": list})
}

func (h *Handler) writeAdminLog(operateType, targetType string, targetID uint64, content *string) error {
	return h.db.Create(&model.AdminOperateLog{
		OperateType:    operateType,
		TargetType:     targetType,
		TargetID:       targetID,
		OperateContent: content,
	}).Error
}
