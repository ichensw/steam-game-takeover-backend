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

func (h *Handler) AdminUpdateTakeover(c *gin.Context) {
	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	var takeover model.Takeover
	if err := h.db.Where("id = ? AND is_deleted = ?", takeoverID, false).First(&takeover).Error; err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if isTakeoverExpired(takeover) {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "ended takeover cannot be modified")
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
	var creator model.User
	if err := h.db.Where("id = ? AND is_deleted = ?", takeover.CreatorUserID, false).First(&creator).Error; err == nil {
		if err := h.checkTextSecurity(contentSecurityTarget{
			User:        creator,
			ContentType: "takeover",
			TargetID:    takeoverID,
			Scene:       contentScenePost,
		}, takeoverSecurityText(parsed)); err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, "content security reject")
			return
		}
	}

	joinedCount, err := countValidJoinedMembers(h.db, takeoverID)
	if err != nil {
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
	var takeover model.Takeover
	if err := h.db.Where("id = ? AND is_deleted = ?", takeoverID, false).First(&takeover).Error; err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
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

func (h *Handler) AdminRestoreUserCredit(c *gin.Context) {
	userID, okID := pathUint64(c, "userId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	var req struct {
		Delta   uint `json:"delta"`
		ToScore uint `json:"toScore"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_deleted = ?", userID, false).First(&user).Error; err != nil {
			return err
		}
		score := req.ToScore
		if score == 0 {
			score = user.CreditScore + req.Delta
		}
		if score > model.DefaultCreditScore {
			score = model.DefaultCreditScore
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("credit_score", score).Error; err != nil {
			return err
		}
		content := "restore credit"
		return tx.Create(&model.AdminOperateLog{
			OperateType:    "USER_CREDIT_RESTORE",
			TargetType:     "user",
			TargetID:       user.ID,
			OperateContent: &content,
		}).Error
	})
	if err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "credit restore failed")
		return
	}
	ok(c, "restored", nil)
}

func (h *Handler) AdminDashboardSummary(c *gin.Context) {
	var takeoverTotal int64
	if err := h.db.Model(&model.Takeover{}).Where("is_deleted = ?", false).Count(&takeoverTotal).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	var userTotal int64
	if err := h.db.Model(&model.User{}).Where("is_deleted = ?", false).Count(&userTotal).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	var pendingReportTotal int64
	if err := h.adminReportBaseQuery("pending").Count(&pendingReportTotal).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", gin.H{
		"takeoverTotal":      takeoverTotal,
		"userTotal":          userTotal,
		"pendingReportTotal": pendingReportTotal,
	})
}

// AdminUserSummary returns user totals for the admin dashboard.
func (h *Handler) AdminUserSummary(c *gin.Context) {
	var wxUserTotal int64
	if err := h.db.Model(&model.User{}).Where("is_deleted = ? AND is_banned = ?", false, false).Count(&wxUserTotal).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	var adminUserTotal int64
	if err := h.db.Model(&model.AdminUser{}).Where("enabled = ?", true).Count(&adminUserTotal).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	var bannedUserTotal int64
	if err := h.db.Model(&model.User{}).Where("is_deleted = ? AND is_banned = ?", false, true).Count(&bannedUserTotal).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", gin.H{
		"wxUserTotal":     wxUserTotal,
		"adminUserTotal":  adminUserTotal,
		"bannedUserTotal": bannedUserTotal,
		"totalUserTotal":  wxUserTotal + adminUserTotal + bannedUserTotal,
	})
}

func (h *Handler) AdminListUsers(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 50 {
		pageSize = 50
	}

	query := h.db.Model(&model.User{}).Where("is_deleted = ?", false)
	switch strings.TrimSpace(c.Query("status")) {
	case "banned":
		query = query.Where("is_banned = ?", true)
	case "normal":
		query = query.Where("is_banned = ?", false)
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("nickname LIKE ? OR steam_id LIKE ? OR openid LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	query = applySort(query, c.Query("sortField"), c.Query("sortOrder"), wxUserSortFields, "gmt_create")

	var users []model.User
	if err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	steamIDs := make([]string, 0, len(users))
	for _, user := range users {
		steamID := strings.TrimSpace(stringValue(user.SteamID))
		if steamID != "" {
			steamIDs = append(steamIDs, steamID)
		}
	}
	whitelist := make(map[string]bool, len(steamIDs))
	if len(steamIDs) > 0 {
		var rows []model.PublishTakeoverWhitelist
		if err := h.db.Where("steam_id IN ? AND enabled = ?", steamIDs, true).Find(&rows).Error; err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		for _, row := range rows {
			whitelist[row.SteamID] = true
		}
	}

	list := make([]userDTO, 0, len(users))
	for _, user := range users {
		list = append(list, toUserDTOWithPublishWhitelist(user, whitelist))
	}
	ok(c, "success", gin.H{
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
		"list":     list,
	})
}

var wxUserSortFields = map[string]string{
	"id":            "id",
	"nickname":      "nickname",
	"steamId":       "steam_id",
	"isBanned":      "is_banned",
	"creditScore":   "credit_score",
	"lastLoginTime": "last_login_time",
	"createdAt":     "gmt_create",
}

func (h *Handler) AdminGetUser(c *gin.Context) {
	userID, okID := pathUint64(c, "userId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	var user model.User
	if err := h.db.Where("id = ? AND is_deleted = ?", userID, false).First(&user).Error; err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", toUserDTO(user))
}

func (h *Handler) AdminBanUser(c *gin.Context) {
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
		fail(c, http.StatusBadRequest, CodeParamInvalid, "ban reason must be at most 255 characters")
		return
	}
	admin, _ := currentAdmin(c)
	now := time.Now()
	result := h.db.Model(&model.User{}).Where("id = ? AND is_deleted = ?", userID, false).Updates(map[string]interface{}{
		"is_banned":          true,
		"ban_reason":         nullableString(reason),
		"banned_at":          now,
		"banned_by_admin_id": admin.ID,
	})
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "ban failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
		return
	}
	ok(c, "banned", nil)
}

func (h *Handler) AdminUnbanUser(c *gin.Context) {
	userID, okID := pathUint64(c, "userId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	result := h.db.Model(&model.User{}).Where("id = ? AND is_deleted = ?", userID, false).Updates(map[string]interface{}{
		"is_banned":          false,
		"ban_reason":         nil,
		"banned_at":          nil,
		"banned_by_admin_id": nil,
	})
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "unban failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
		return
	}
	ok(c, "unbanned", nil)
}

func (h *Handler) AdminListTakeovers(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 50 {
		pageSize = 50
	}

	query := h.db.Model(&model.Takeover{}).Where("is_deleted = ?", false)
	query = applyKeywordFilter(query, c.Query("keyword"))
	if state := strings.TrimSpace(c.Query("status")); state != "" {
		switch state {
		case "normal":
			query = query.Where("takeover_state = ?", model.TakeoverStateNormal)
		case "closed":
			query = query.Where("takeover_state = ?", model.TakeoverStateClosed)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	query = applySort(query, c.Query("sortField"), c.Query("sortOrder"), takeoverSortFields, "gmt_create")

	var takeovers []model.Takeover
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&takeovers).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	list := make([]takeoverDTO, 0, len(takeovers))
	for _, takeover := range takeovers {
		joined, _, err := h.takeoverStats(takeover.ID, 0)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto := toTakeoverDTOWithCreator(h.db, takeover, joined, false)
		members, err := h.takeoverMembers(takeover.ID, true, 5)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto.PreviewMembers = members
		list = append(list, dto)
	}
	ok(c, "success", gin.H{"page": page, "pageSize": pageSize, "total": total, "list": list})
}

var takeoverSortFields = map[string]string{
	"id":               "id",
	"title":            "title",
	"participantLimit": "participant_limit",
	"scheduleType":     "schedule_type",
	"startDate":        "start_date",
	"endDate":          "end_date",
	"playTime":         "play_time",
	"status":           "takeover_state",
	"createdAt":        "gmt_create",
}

func (h *Handler) AdminBatchPublishWhitelist(c *gin.Context) {
	var req struct {
		SteamIDs []string `json:"steamIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	count := 0
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, value := range req.SteamIDs {
			steamID := strings.TrimSpace(value)
			if steamID == "" || len([]rune(steamID)) > 64 {
				continue
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "steam_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"enabled": true}),
			}).Create(&model.PublishTakeoverWhitelist{SteamID: steamID, Enabled: true}).Error; err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "saved", gin.H{"count": count})
}

func (h *Handler) writeAdminLog(operateType, targetType string, targetID uint64, content *string) error {
	return h.db.Create(&model.AdminOperateLog{
		OperateType:    operateType,
		TargetType:     targetType,
		TargetID:       targetID,
		OperateContent: content,
	}).Error
}
