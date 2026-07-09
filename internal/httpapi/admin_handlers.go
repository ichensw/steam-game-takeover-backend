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
	if err := syncExpiredTakeovers(h.db, time.Now()); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
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
	if takeover.TakeoverState == model.TakeoverStateClosed {
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
	if err := h.fillKookInviteURL(&parsed); err != nil {
		fail(c, http.StatusBadGateway, CodeSystemError, "kook invite create failed")
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
			"kook_channel_id":   parsed.KookChannelID,
			"kook_channel_name": parsed.KookChannelName,
			"kook_invite_url":   parsed.KookInviteURL,
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
	takeover.Title = parsed.Title
	takeover.ParticipantLimit = parsed.ParticipantLimit
	takeover.ScheduleType = parsed.ScheduleType
	takeover.StartDate = parsed.StartDate
	takeover.EndDate = parsed.EndDate
	takeover.PlayTime = parsed.PlayTime
	takeover.Description = parsed.Description
	takeover.KookChannelID = parsed.KookChannelID
	takeover.KookChannelName = parsed.KookChannelName
	takeover.KookInviteURL = parsed.KookInviteURL
	ok(c, "saved", toTakeoverDTOWithCreator(h.db, takeover, joinedCount, false))
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
		if err := recordCreditLog(tx, user.ID, user.CreditScore, score, "admin_restore", "管理员恢复信誉分", nil, nil); err != nil {
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
	if pageSize > 100 {
		pageSize = 100
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

	query = applyWXUserSort(query, c.Query("sortField"), c.Query("sortOrder"))

	var users []model.User
	if err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	openIDs := make([]string, 0, len(users))
	steamIDs := make([]string, 0, len(users))
	for _, user := range users {
		if user.OpenID != "" {
			openIDs = append(openIDs, user.OpenID)
		}
		steamID := strings.TrimSpace(stringValue(user.SteamID))
		if steamID != "" {
			steamIDs = append(steamIDs, steamID)
		}
	}
	whitelist := make(map[string]bool, len(openIDs)+len(steamIDs))
	if len(openIDs) > 0 || len(steamIDs) > 0 {
		var rows []model.PublishTakeoverWhitelist
		query := h.db.Where("enabled = ?", true)
		if len(openIDs) > 0 && len(steamIDs) > 0 {
			query = query.Where("openid IN ? OR steam_id IN ?", openIDs, steamIDs)
		} else if len(openIDs) > 0 {
			query = query.Where("openid IN ?", openIDs)
		} else {
			query = query.Where("steam_id IN ?", steamIDs)
		}
		if err := query.Find(&rows).Error; err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		for _, row := range rows {
			openID := strings.TrimSpace(stringValue(row.OpenID))
			if openID != "" {
				whitelist[openID] = true
			}
			steamID := strings.TrimSpace(stringValue(row.SteamID))
			if steamID != "" {
				whitelist[steamID] = true
			}
		}
	}

	list := make([]adminWXUserDTO, 0, len(users))
	for _, user := range users {
		list = append(list, toAdminWXUserDTOWithPublishWhitelist(user, whitelist))
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

func applyWXUserSort(query *gorm.DB, field string, order string) *gorm.DB {
	if strings.TrimSpace(field) != "publishWhitelisted" {
		return applySort(query, field, order, wxUserSortFields, "gmt_create")
	}
	return query.Order(wxUserPublishWhitelistSortClause(order)).Order("gmt_create DESC")
}

func wxUserPublishWhitelistSortClause(order string) string {
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(order), "asc") {
		direction = "ASC"
	}
	return `EXISTS (
		SELECT 1 FROM ttw_publish_takeover_whitelist w
		WHERE w.enabled = TRUE
		  AND (
		    w.openid = ttw_user.openid
		    OR (ttw_user.steam_id IS NOT NULL AND ttw_user.steam_id <> '' AND w.steam_id = ttw_user.steam_id)
		  )
	) ` + direction
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
	ok(c, "success", toAdminWXUserDTO(user))
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

func (h *Handler) AdminBatchSetUserAdmin(c *gin.Context) {
	var req struct {
		UserIDs []uint64 `json:"userIds"`
		IsAdmin *bool    `json:"isAdmin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	if len(req.UserIDs) == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "userIds is required")
		return
	}
	if req.IsAdmin == nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "isAdmin is required")
		return
	}
	result := h.db.Model(&model.User{}).
		Where("id IN ? AND is_deleted = ?", req.UserIDs, false).
		Update("is_admin", *req.IsAdmin)
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "saved", gin.H{"count": result.RowsAffected})
}

func (h *Handler) AdminListTakeovers(c *gin.Context) {
	if err := syncExpiredTakeovers(h.db, time.Now()); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	query := h.db.Model(&model.Takeover{}).Where("is_deleted = ?", false)
	query = applyKeywordFilter(query, c.Query("keyword"))
	var err error
	query, err = applyTimeFilter(query, c)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	if state := strings.TrimSpace(c.Query("status")); state != "" {
		switch state {
		case "normal":
			query = applyTakeoverEndedFilter(query, false, time.Now())
		case "closed":
			query = applyTakeoverEndedFilter(query, true, time.Now())
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
		OpenIDs []string `json:"openids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	seen := make(map[string]bool, len(req.OpenIDs))
	openIDs := make([]string, 0, len(req.OpenIDs))
	for _, value := range req.OpenIDs {
		openID := strings.TrimSpace(value)
		if openID == "" || len([]rune(openID)) > 64 || seen[openID] {
			continue
		}
		seen[openID] = true
		openIDs = append(openIDs, openID)
	}
	if len(openIDs) == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "openids are required")
		return
	}

	var users []model.User
	if err := h.db.Where("openid IN ? AND is_deleted = ?", openIDs, false).Find(&users).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	count := 0
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, user := range users {
			steamID := normalizeSteamID64ToFriendCode(strings.TrimSpace(stringValue(user.SteamID)))
			var row model.PublishTakeoverWhitelist
			query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("openid = ?", user.OpenID)
			if steamID != "" {
				query = query.Or("steam_id = ?", steamID)
			}
			err := query.First(&row).Error
			updates := map[string]interface{}{
				"openid":   stringPtr(user.OpenID),
				"steam_id": optionalStringPtr(steamID),
				"enabled":  true,
			}
			if err == nil {
				if err := tx.Model(&model.PublishTakeoverWhitelist{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
					return err
				}
			} else {
				if !isNotFound(err) {
					return err
				}
				if err := tx.Create(&model.PublishTakeoverWhitelist{
					OpenID:  stringPtr(user.OpenID),
					SteamID: optionalStringPtr(steamID),
					Enabled: true,
				}).Error; err != nil {
					return err
				}
			}
			count++
		}
		return nil
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if count == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "no valid users for publish whitelist")
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
