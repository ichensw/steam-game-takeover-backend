package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const duplicateCreateWindow = 10 * time.Second
const soonTakeoverWindow = 2 * time.Hour

type takeoverListRow struct {
	model.Takeover `gorm:"embedded"`
	JoinedCount    int64 `gorm:"column:joined_count"`
	HasJoined      int   `gorm:"column:has_joined"`
}

func (h *Handler) ListTakeovers(c *gin.Context) {
	user, _ := currentUser(c)
	now := time.Now()

	if err := syncExpiredTakeovers(h.db, now); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 10)
	if pageSize > 100 {
		pageSize = 100
	}

	countQuery := h.db.Model(&model.Takeover{}).
		Where("is_deleted = ? AND takeover_state = ?", false, model.TakeoverStateNormal)
	listQuery := h.takeoverListQuery(user.ID).
		Where("is_deleted = ? AND takeover_state = ?", false, model.TakeoverStateNormal)
	countQuery = applyKeywordFilter(countQuery, c.Query("keyword"))
	listQuery = applyKeywordFilter(listQuery, c.Query("keyword"))
	var err error
	countQuery, err = applyTimeFilter(countQuery, c)
	if err == nil {
		listQuery, err = applyTimeFilter(listQuery, c)
	}
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var rows []takeoverListRow
	if err := applyTakeoverRecommendOrder(listQuery, now).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]takeoverDTO, 0, len(rows))
	for _, row := range rows {
		takeover := row.Takeover
		hasJoined := row.HasJoined == 1
		dto := toTakeoverDTOWithCreator(h.db, takeover, row.JoinedCount, hasJoined)
		dto.IsCreator = isTakeoverCreator(user, takeover)
		dto.CanManage = canManageTakeover(user, takeover)
		dto.RecommendTags = takeoverRecommendTags(takeover, row.JoinedCount, hasJoined, now)
		members, err := h.takeoverMembers(takeover.ID, false, 5)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto.PreviewMembers = members
		list = append(list, dto)
	}

	ok(c, "success", gin.H{
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
		"list":     list,
	})
}

func (h *Handler) takeoverListQuery(userID uint64) *gorm.DB {
	return h.db.Table("ttw_takeover").
		Select("ttw_takeover.*, COALESCE(j.joined_count, 0) AS joined_count, IF(hj.user_id IS NULL, 0, 1) AS has_joined").
		Joins("LEFT JOIN (SELECT m.takeover_id, COUNT(*) AS joined_count FROM ttw_takeover_member AS m JOIN ttw_user AS u ON u.id = m.user_id AND u.is_deleted = ? WHERE m.member_state = ? GROUP BY m.takeover_id) AS j ON j.takeover_id = ttw_takeover.id", false, model.MemberStateJoined).
		Joins("LEFT JOIN ttw_takeover_member AS hj ON hj.takeover_id = ttw_takeover.id AND hj.user_id = ? AND hj.member_state = ?", userID, model.MemberStateJoined)
}

func applyTakeoverRecommendOrder(query *gorm.DB, now time.Time) *gorm.DB {
	full := "participant_limit > 0 AND COALESCE(j.joined_count, 0) >= participant_limit"
	today := now.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	clock := now.Format("15:04:05")
	nextPlayDate := `CASE
WHEN schedule_type = ? AND play_time < ? THEN ?
WHEN schedule_type = ? THEN ?
WHEN schedule_type = ? AND start_date > ? THEN start_date
WHEN schedule_type = ? AND play_time < ? THEN ?
WHEN schedule_type = ? THEN ?
ELSE start_date END`
	orderSQL := "CASE WHEN " + full + " THEN 1 ELSE 0 END ASC, " + nextPlayDate + " ASC, play_time ASC, ttw_takeover.id DESC"

	return query.
		Order(clause.OrderBy{Expression: clause.Expr{
			SQL: orderSQL,
			Vars: []interface{}{
				model.ScheduleDaily, clock, tomorrow,
				model.ScheduleDaily, today,
				model.ScheduleDateRange, today,
				model.ScheduleDateRange, clock, tomorrow,
				model.ScheduleDateRange, today,
			},
			WithoutParentheses: true,
		}})
}

func takeoverRecommendTags(t model.Takeover, joinedCount int64, hasJoined bool, now time.Time) []recommendTagDTO {
	tags := make([]recommendTagDTO, 0, 3)
	if hasJoined {
		tags = append(tags, recommendTagDTO{Type: "joined", Label: "我已加入", Tone: "primary"})
	}
	if t.TakeoverState == model.TakeoverStateClosed {
		return appendRecommendTag(tags, recommendTagDTO{Type: "ended", Label: "已结束", Tone: "muted"})
	}
	if isTakeoverSoon(t, now) {
		tags = appendRecommendTag(tags, recommendTagDTO{Type: "soon", Label: "快开始", Tone: "hot"})
	} else if isTakeoverUpcomingToday(t, now) {
		tags = appendRecommendTag(tags, recommendTagDTO{Type: "today", Label: "今日开局", Tone: "cool"})
	}

	remaining := int64(t.ParticipantLimit) - joinedCount
	switch {
	case remaining > 0 && remaining <= 2:
		tags = appendRecommendTag(tags, recommendTagDTO{Type: "almostFull", Label: "差" + strconv.FormatInt(remaining, 10) + "人", Tone: "warm"})
	case t.ParticipantLimit > 0 && remaining <= 0:
		tags = appendRecommendTag(tags, recommendTagDTO{Type: "full", Label: "已满员", Tone: "muted"})
	}
	return tags
}

func appendRecommendTag(tags []recommendTagDTO, tag recommendTagDTO) []recommendTagDTO {
	if len(tags) >= 3 {
		return tags
	}
	return append(tags, tag)
}

func isTakeoverSoon(t model.Takeover, now time.Time) bool {
	playAt, ok := nextTakeoverPlayAt(t, now)
	return ok && !playAt.Before(now) && !playAt.After(now.Add(soonTakeoverWindow))
}

func isTakeoverUpcomingToday(t model.Takeover, now time.Time) bool {
	playAt, ok := todayTakeoverPlayAt(t, now)
	return ok && !playAt.Before(now)
}

func todayTakeoverPlayAt(t model.Takeover, now time.Time) (time.Time, bool) {
	today := truncateDate(now)
	switch t.ScheduleType {
	case model.ScheduleDaily:
	case model.ScheduleSpecifiedDate:
		if t.StartDate == nil || !sameDate(*t.StartDate, today) {
			return time.Time{}, false
		}
	case model.ScheduleDateRange:
		if t.StartDate == nil || t.EndDate == nil || truncateDate(*t.StartDate).After(today) || truncateDate(*t.EndDate).Before(today) {
			return time.Time{}, false
		}
	default:
		return time.Time{}, false
	}
	playAt, err := combineDateAndPlayTime(today, t.PlayTime)
	return playAt, err == nil
}

func (h *Handler) GetTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	h.getTakeoverDetail(c, false, user)
}

func (h *Handler) ListTakeoverMemberActivities(c *gin.Context) {
	user, _ := currentUser(c)
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

	_, hasJoined, err := h.takeoverStats(takeover.ID, user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if takeover.TakeoverState == model.TakeoverStateClosed && !hasJoined && !isTakeoverCreator(user, takeover) && !canManageTakeover(user, takeover) {
		fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		return
	}

	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 50 {
		pageSize = 50
	}
	countQuery := applyMemberActivityKeywordFilter(h.memberActivityBaseQuery(takeover.ID), c.Query("keyword"))
	listQuery := applyMemberActivityKeywordFilter(h.memberActivityBaseQuery(takeover.ID), c.Query("keyword"))

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var rows []memberActivityRow
	if err := listQuery.
		Select("a.id, u.id AS user_id, u.openid, u.nickname, u.steam_id, u.gender, u.avatar_url, a.remark, a.action, a.gmt_create AS created_at").
		Order("a.gmt_create DESC, a.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]memberActivityDTO, 0, len(rows))
	for _, row := range rows {
		list = append(list, toMemberActivityDTO(row, false))
	}
	if user.ID != 0 {
		reportedUserIDs, err := h.reportedUserIDs(takeover.ID, user.ID)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		for index := range list {
			list[index].IsSelf = list[index].UserID == user.ID
			list[index].HasReported = reportedUserIDs[list[index].UserID]
		}
	}

	ok(c, "success", gin.H{
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
		"list":     list,
	})
}

func (h *Handler) AdminGetTakeover(c *gin.Context) {
	h.getTakeoverDetail(c, true, model.User{IsAdmin: true})
}

func (h *Handler) getTakeoverDetail(c *gin.Context, includeOpenID bool, user model.User) {
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

	joinedCount, hasJoined, err := h.takeoverStats(takeover.ID, user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if takeover.TakeoverState == model.TakeoverStateClosed && !hasJoined && !isTakeoverCreator(user, takeover) && !canManageTakeover(user, takeover) {
		fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		return
	}
	members, err := h.takeoverMembers(takeover.ID, includeOpenID, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	activities, err := h.takeoverMemberActivities(takeover.ID, includeOpenID, 100)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if user.ID != 0 {
		reportedUserIDs, err := h.reportedUserIDs(takeover.ID, user.ID)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		for index := range members {
			members[index].IsSelf = members[index].UserID == user.ID
			members[index].HasReported = reportedUserIDs[members[index].UserID]
		}
		for index := range activities {
			activities[index].IsSelf = activities[index].UserID == user.ID
			activities[index].HasReported = reportedUserIDs[activities[index].UserID]
		}
	}
	dto := toTakeoverDTOWithCreator(h.db, takeover, joinedCount, hasJoined)
	dto.IsCreator = isTakeoverCreator(user, takeover)
	dto.CanManage = canManageTakeover(user, takeover)
	dto.Members = members
	dto.MemberActivities = activities
	ok(c, "success", dto)
}

func (h *Handler) UpdateTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

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
	if !canManageTakeover(user, takeover) {
		fail(c, http.StatusForbidden, CodeAdminUnauthorized, "admin unauthorized")
		return
	}
	if err := syncExpiredTakeovers(h.db, time.Now()); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if err := h.db.Where("id = ? AND is_deleted = ?", takeoverID, false).First(&takeover).Error; err != nil {
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
	if err := h.checkTextSecurity(contentSecurityTarget{
		User:        user,
		ContentType: "takeover",
		TargetID:    takeoverID,
		Scene:       contentScenePost,
	}, takeoverSecurityText(parsed)); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "content security reject")
		return
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
	ok(c, "saved", toTakeoverDTOWithCreator(h.db, takeover, joinedCount, true))
}

func (h *Handler) DeleteTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

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
	if !canManageTakeover(user, takeover) {
		fail(c, http.StatusForbidden, CodeAdminUnauthorized, "admin unauthorized")
		return
	}
	if err := syncExpiredTakeovers(h.db, time.Now()); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if err := h.db.Where("id = ? AND is_deleted = ?", takeoverID, false).First(&takeover).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if takeover.TakeoverState == model.TakeoverStateClosed {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "ended takeover cannot be deleted")
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

func (h *Handler) CreateTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, true) {
		return
	}
	if !h.canPublishTakeover(user) {
		fail(c, http.StatusForbidden, CodeParamInvalid, "publish takeover disabled")
		return
	}

	var req takeoverInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	parsed, err := validateTakeoverInput(req, true)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	if err := h.checkTextSecurity(contentSecurityTarget{
		User:        user,
		ContentType: "takeover",
		Scene:       contentScenePost,
	}, takeoverSecurityText(parsed)); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "content security reject")
		return
	}
	if err := h.fillKookInviteURL(&parsed); err != nil {
		fail(c, http.StatusBadGateway, CodeSystemError, "kook invite create failed")
		return
	}

	var takeover model.Takeover
	var joinedCount int64
	hasJoined := false
	deduplicated := false
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		freshUser, err := h.lockProfileUser(tx, user.ID)
		if err != nil {
			return err
		}
		if freshUser.CreditScore < model.MinCreateCreditScore {
			return errCreditTooLowForCreate
		}
		if err := syncExpiredTakeovers(tx, time.Now()); err != nil {
			return err
		}
		existing, err := h.findRecentDuplicateTakeover(tx, freshUser.ID, parsed)
		if err != nil {
			return err
		}
		if existing.ID != 0 {
			takeover = existing
			joinedCount, hasJoined, err = h.takeoverStats(takeover.ID, freshUser.ID)
			if err != nil {
				return err
			}
			deduplicated = true
			return nil
		}
		takeover = model.Takeover{
			CreatorUserID:    freshUser.ID,
			Title:            parsed.Title,
			ParticipantLimit: parsed.ParticipantLimit,
			ScheduleType:     parsed.ScheduleType,
			StartDate:        parsed.StartDate,
			EndDate:          parsed.EndDate,
			PlayTime:         parsed.PlayTime,
			Description:      parsed.Description,
			KookChannelID:    parsed.KookChannelID,
			KookChannelName:  parsed.KookChannelName,
			KookInviteURL:    parsed.KookInviteURL,
			TakeoverState:    model.TakeoverStateNormal,
		}
		conflict, err := h.hasActiveScheduleConflict(tx, freshUser.ID, takeover)
		if err != nil {
			return err
		}
		if conflict {
			return errTakeoverTimeConflict
		}
		if err := tx.Create(&takeover).Error; err != nil {
			return err
		}
		member := model.TakeoverMember{TakeoverID: takeover.ID, UserID: freshUser.ID, MemberState: model.MemberStateJoined}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		if err := recordTakeoverMemberActivity(tx, takeover.ID, freshUser.ID, model.MemberActionJoin, nil); err != nil {
			return err
		}
		joinedCount = 1
		hasJoined = true
		return nil
	}); err != nil {
		if errors.Is(err, errProfileRequired) {
			fail(c, http.StatusBadRequest, CodeProfileIncomplete, "请先补充资料")
			return
		}
		if errors.Is(err, errCreditTooLowForCreate) {
			fail(c, http.StatusForbidden, CodeParamInvalid, "credit too low for create")
			return
		}
		if errors.Is(err, errTakeoverTimeConflict) {
			fail(c, http.StatusConflict, CodeTakeoverTimeConflict, "takeover time conflict")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "create failed")
		return
	}
	ok(c, "created", gin.H{"id": takeover.ID, "hasJoined": hasJoined, "joinedCount": joinedCount, "deduplicated": deduplicated})
}

func (h *Handler) fillKookInviteURL(parsed *parsedTakeoverInput) error {
	if parsed.KookChannelID == nil {
		parsed.KookInviteURL = nil
		return nil
	}
	inviteURL, err := h.createKookInviteURL(*parsed.KookChannelID)
	if err != nil {
		return err
	}
	parsed.KookInviteURL = optionalStringPtr(inviteURL)
	return nil
}

func (h *Handler) findRecentDuplicateTakeover(tx *gorm.DB, creatorUserID uint64, parsed parsedTakeoverInput) (model.Takeover, error) {
	var takeover model.Takeover
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"creator_user_id = ? AND title = ? AND participant_limit = ? AND schedule_type = ? AND play_time = ? AND is_deleted = ? AND takeover_state = ? AND gmt_create >= ?",
		creatorUserID,
		parsed.Title,
		parsed.ParticipantLimit,
		parsed.ScheduleType,
		parsed.PlayTime,
		false,
		model.TakeoverStateNormal,
		time.Now().Add(-duplicateCreateWindow),
	)
	query = whereNullableTime(query, "start_date", parsed.StartDate)
	query = whereNullableTime(query, "end_date", parsed.EndDate)
	query = whereNullableString(query, "description", parsed.Description)

	if err := query.Order("id DESC").First(&takeover).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Takeover{}, nil
		}
		return model.Takeover{}, err
	}
	return takeover, nil
}

func whereNullableTime(query *gorm.DB, column string, value *time.Time) *gorm.DB {
	if value == nil {
		return query.Where(column + " IS NULL")
	}
	return query.Where(column+" = ?", value)
}

func whereNullableString(query *gorm.DB, column string, value *string) *gorm.DB {
	if value == nil {
		return query.Where(column + " IS NULL")
	}
	return query.Where(column+" = ?", *value)
}

func (h *Handler) JoinTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, true) {
		return
	}

	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	remark, valid := bindOptionalMemberRemark(c)
	if !valid {
		return
	}

	var joinedCount int64
	err := h.db.Transaction(func(tx *gorm.DB) error {
		freshUser, err := h.lockProfileUser(tx, user.ID)
		if err != nil {
			return err
		}
		if freshUser.CreditScore < model.MinJoinCreditScore {
			return errCreditTooLowForJoin
		}
		if err := syncExpiredTakeovers(tx, time.Now()); err != nil {
			return err
		}

		var takeover model.Takeover
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = ?", takeoverID, false).
			First(&takeover).Error; err != nil {
			return err
		}
		if takeover.TakeoverState == model.TakeoverStateClosed {
			return errTakeoverEnded
		}

		var member model.TakeoverMember
		err = tx.Where("takeover_id = ? AND user_id = ?", takeoverID, freshUser.ID).First(&member).Error
		if err == nil && member.MemberState == model.MemberStateJoined {
			return errAlreadyJoined
		}
		if err != nil && !isNotFound(err) {
			return err
		}
		conflict, err := h.hasActiveScheduleConflict(tx, freshUser.ID, takeover)
		if err != nil {
			return err
		}
		if conflict {
			return errTakeoverTimeConflict
		}

		count, err := countValidJoinedMembers(tx, takeoverID)
		if err != nil {
			return err
		}
		joinedCount = count
		if uint(joinedCount) >= takeover.ParticipantLimit {
			return errTakeoverFull
		}

		if member.ID != 0 {
			if err := tx.Model(&model.TakeoverMember{}).Where("id = ?", member.ID).Updates(map[string]interface{}{
				"member_state": model.MemberStateJoined,
				"remark":       remark,
			}).Error; err != nil {
				return err
			}
		} else {
			member = model.TakeoverMember{TakeoverID: takeoverID, UserID: freshUser.ID, MemberState: model.MemberStateJoined, Remark: remark}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}
		if err := recordTakeoverMemberActivity(tx, takeoverID, freshUser.ID, model.MemberActionJoin, remark); err != nil {
			return err
		}
		joinedCount++
		return nil
	})
	if err != nil {
		switch {
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		case errors.Is(err, errAlreadyJoined):
			fail(c, http.StatusConflict, CodeAlreadyJoined, "already joined")
		case errors.Is(err, errTakeoverTimeConflict):
			fail(c, http.StatusConflict, CodeTakeoverTimeConflict, "takeover time conflict")
		case errors.Is(err, errTakeoverEnded):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "ended takeover cannot be joined")
		case errors.Is(err, errTakeoverFull):
			fail(c, http.StatusConflict, CodeTakeoverFull, "takeover full")
		case errors.Is(err, errProfileRequired):
			fail(c, http.StatusBadRequest, CodeProfileIncomplete, "请先补充资料")
		case errors.Is(err, errCreditTooLowForJoin):
			fail(c, http.StatusForbidden, CodeParamInvalid, "credit too low for join")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "join failed")
		}
		return
	}

	ok(c, "joined", gin.H{"hasJoined": true, "joinedCount": joinedCount})
}

func (h *Handler) UpdateMemberRemark(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	remark, valid := bindOptionalMemberRemark(c)
	if !valid {
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := syncExpiredTakeovers(tx, time.Now()); err != nil {
			return err
		}
		var takeover model.Takeover
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = ? AND takeover_state = ?", takeoverID, false, model.TakeoverStateNormal).
			First(&takeover).Error; err != nil {
			return err
		}
		result := tx.Model(&model.TakeoverMember{}).
			Where("takeover_id = ? AND user_id = ? AND member_state = ?", takeoverID, user.ID, model.MemberStateJoined).
			Update("remark", remark)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errNotJoined
		}
		return nil
	})
	if err != nil {
		switch {
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		case errors.Is(err, errTakeoverEnded):
			fail(c, http.StatusBadRequest, CodeParamInvalid, "ended takeover cannot be modified")
		case errors.Is(err, errNotJoined):
			fail(c, http.StatusConflict, CodeParamInvalid, "not joined")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		}
		return
	}

	ok(c, "saved", gin.H{"remark": stringValue(remark)})
}

func (h *Handler) LeaveTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, true) {
		return
	}

	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}

	var joinedCount int64
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var takeover model.Takeover
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = ? AND takeover_state = ?", takeoverID, false, model.TakeoverStateNormal).
			First(&takeover).Error; err != nil {
			return err
		}
		result := tx.Model(&model.TakeoverMember{}).
			Where("takeover_id = ? AND user_id = ? AND member_state = ?", takeoverID, user.ID, model.MemberStateJoined).
			Update("member_state", model.MemberStateExited)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errNotJoined
		}
		if err := recordTakeoverMemberActivity(tx, takeoverID, user.ID, model.MemberActionLeave, nil); err != nil {
			return err
		}

		count, err := countValidJoinedMembers(tx, takeoverID)
		if err != nil {
			return err
		}
		joinedCount = count
		return nil
	})
	if err != nil {
		switch {
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		case errors.Is(err, errNotJoined):
			fail(c, http.StatusConflict, CodeParamInvalid, "not joined")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "leave failed")
		}
		return
	}

	ok(c, "left", gin.H{"hasJoined": false, "joinedCount": joinedCount})
}

var (
	errAlreadyJoined         = errors.New("already joined")
	errTakeoverTimeConflict  = errors.New("takeover time conflict")
	errTakeoverEnded         = errors.New("ended takeover cannot be joined")
	errTakeoverFull          = errors.New("takeover full")
	errNotJoined             = errors.New("not joined")
	errProfileRequired       = errors.New("profile required")
	errCreditTooLowForCreate = errors.New("credit too low for create")
	errCreditTooLowForJoin   = errors.New("credit too low for join")
	errRemarkTooLong         = errors.New("remark too long")
)

func (h *Handler) hasActiveScheduleConflict(tx *gorm.DB, userID uint64, target model.Takeover) (bool, error) {
	var takeovers []model.Takeover
	if err := tx.Table("ttw_takeover AS t").
		Select("t.*").
		Joins("JOIN ttw_takeover_member AS m ON m.takeover_id = t.id").
		Where("m.user_id = ? AND m.member_state = ? AND t.id <> ? AND t.is_deleted = ? AND t.takeover_state = ? AND t.play_time = ?",
			userID,
			model.MemberStateJoined,
			target.ID,
			false,
			model.TakeoverStateNormal,
			target.PlayTime,
		).
		Find(&takeovers).Error; err != nil {
		return false, err
	}

	for _, takeover := range takeovers {
		if schedulesConflict(target, takeover) {
			return true, nil
		}
	}
	return false, nil
}

func schedulesConflict(a, b model.Takeover) bool {
	if shortTime(a.PlayTime) != shortTime(b.PlayTime) {
		return false
	}
	if a.ScheduleType == model.ScheduleDaily || b.ScheduleType == model.ScheduleDaily {
		return true
	}

	aStart, aEnd, okA := takeoverDateRange(a)
	bStart, bEnd, okB := takeoverDateRange(b)
	if !okA || !okB {
		return false
	}
	return !aEnd.Before(bStart) && !bEnd.Before(aStart)
}

func takeoverDateRange(t model.Takeover) (time.Time, time.Time, bool) {
	switch t.ScheduleType {
	case model.ScheduleSpecifiedDate:
		if t.StartDate == nil {
			return time.Time{}, time.Time{}, false
		}
		day := truncateDate(*t.StartDate)
		return day, day, true
	case model.ScheduleDateRange:
		if t.StartDate == nil || t.EndDate == nil {
			return time.Time{}, time.Time{}, false
		}
		return truncateDate(*t.StartDate), truncateDate(*t.EndDate), true
	default:
		return time.Time{}, time.Time{}, false
	}
}

func (h *Handler) lockProfileUser(tx *gorm.DB, userID uint64) (model.User, error) {
	var user model.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_deleted = ?", userID, false).First(&user).Error; err != nil {
		if isNotFound(err) {
			return model.User{}, errProfileRequired
		}
		return model.User{}, err
	}
	if !hasUserProfileFields(user) {
		return model.User{}, errProfileRequired
	}
	return user, nil
}

func ensureUserAllowed(c *gin.Context, user model.User, requireProfile bool) bool {
	if user.IsDeleted {
		fail(c, http.StatusBadRequest, CodeProfileIncomplete, "请先补充资料")
		return false
	}
	if requireProfile && !hasUserProfileFields(user) {
		fail(c, http.StatusBadRequest, CodeProfileIncomplete, "请先补充资料")
		return false
	}
	return true
}

func canManageTakeover(user model.User, takeover model.Takeover) bool {
	return user.IsAdmin || isTakeoverCreator(user, takeover)
}

func isTakeoverCreator(user model.User, takeover model.Takeover) bool {
	return user.ID != 0 && takeover.CreatorUserID == user.ID
}

func (h *Handler) takeoverStats(takeoverID uint64, userID uint64) (int64, bool, error) {
	joinedCount, err := countValidJoinedMembers(h.db, takeoverID)
	if err != nil {
		return 0, false, err
	}
	if userID == 0 {
		return joinedCount, false, nil
	}
	var hasJoinedCount int64
	if err := h.db.Table("ttw_takeover_member AS m").
		Joins("JOIN ttw_user AS u ON u.id = m.user_id").
		Where("m.takeover_id = ? AND m.user_id = ? AND m.member_state = ? AND u.is_deleted = ?", takeoverID, userID, model.MemberStateJoined, false).
		Count(&hasJoinedCount).Error; err != nil {
		return 0, false, err
	}
	return joinedCount, hasJoinedCount > 0, nil
}

func countValidJoinedMembers(db *gorm.DB, takeoverID uint64) (int64, error) {
	var joinedCount int64
	err := db.Table("ttw_takeover_member AS m").
		Joins("JOIN ttw_user AS u ON u.id = m.user_id").
		Where("m.takeover_id = ? AND m.member_state = ? AND u.is_deleted = ?", takeoverID, model.MemberStateJoined, false).
		Count(&joinedCount).Error
	return joinedCount, err
}

func (h *Handler) takeoverMembers(takeoverID uint64, includeOpenID bool, limit int) ([]memberDTO, error) {
	var rows []memberRow
	query := h.db.Table("ttw_takeover_member AS m").
		Select("u.id AS user_id, u.openid, u.nickname, u.steam_id, u.gender, u.avatar_url, m.remark, u.credit_score, m.gmt_create AS joined_at").
		Joins("JOIN ttw_user AS u ON u.id = m.user_id").
		Where("m.takeover_id = ? AND m.member_state = ? AND u.is_deleted = ?", takeoverID, model.MemberStateJoined, false).
		Order("m.gmt_create ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	members := make([]memberDTO, 0, len(rows))
	for _, row := range rows {
		members = append(members, toMemberDTO(row, includeOpenID))
	}
	return members, nil
}

func (h *Handler) takeoverMemberActivities(takeoverID uint64, includeOpenID bool, limit int) ([]memberActivityDTO, error) {
	var rows []memberActivityRow
	query := h.memberActivityBaseQuery(takeoverID).
		Select("a.id, u.id AS user_id, u.openid, u.nickname, u.steam_id, u.gender, u.avatar_url, a.remark, a.action, a.gmt_create AS created_at").
		Order("a.gmt_create DESC, a.id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	activities := make([]memberActivityDTO, 0, len(rows))
	for _, row := range rows {
		activities = append(activities, toMemberActivityDTO(row, includeOpenID))
	}
	return activities, nil
}

func (h *Handler) memberActivityBaseQuery(takeoverID uint64) *gorm.DB {
	return h.db.Table("ttw_takeover_member_activity AS a").
		Joins("JOIN ttw_user AS u ON u.id = a.user_id").
		Where("a.takeover_id = ? AND u.is_deleted = ?", takeoverID, false)
}

func recordTakeoverMemberActivity(tx *gorm.DB, takeoverID, userID uint64, action uint8, remark *string) error {
	return tx.Create(&model.TakeoverMemberActivity{
		TakeoverID: takeoverID,
		UserID:     userID,
		Action:     action,
		Remark:     remark,
	}).Error
}

func bindOptionalMemberRemark(c *gin.Context) (*string, bool) {
	var req struct {
		Remark string `json:"remark"`
	}
	err := c.ShouldBindJSON(&req)
	if err != nil && !errors.Is(err, io.EOF) {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return nil, false
	}
	remark, err := normalizeMemberRemark(req.Remark)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "remark must be at most 100 characters")
		return nil, false
	}
	return remark, true
}

func normalizeMemberRemark(value string) (*string, error) {
	remark := strings.TrimSpace(value)
	if len([]rune(remark)) > 100 {
		return nil, errRemarkTooLong
	}
	return optionalStringPtr(remark), nil
}

func (h *Handler) reportedUserIDs(takeoverID uint64, reporterUserID uint64) (map[uint64]bool, error) {
	var reportRows []struct {
		ReportedUserID uint64
	}
	if err := h.db.Model(&model.TakeoverReport{}).
		Select("reported_user_id").
		Where("takeover_id = ? AND reporter_user_id = ?", takeoverID, reporterUserID).
		Scan(&reportRows).Error; err != nil {
		return nil, err
	}

	result := make(map[uint64]bool, len(reportRows))
	for _, row := range reportRows {
		result[row.ReportedUserID] = true
	}
	return result, nil
}

func applyKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return query
	}
	like := "%" + keyword + "%"
	return query.Where(
		"title LIKE ? OR description LIKE ? OR EXISTS (SELECT 1 FROM ttw_user cu WHERE cu.id = ttw_takeover.creator_user_id AND cu.is_deleted = ? AND cu.nickname LIKE ?) OR EXISTS (SELECT 1 FROM ttw_takeover_member km JOIN ttw_user ku ON ku.id = km.user_id WHERE km.takeover_id = ttw_takeover.id AND km.member_state = ? AND ku.is_deleted = ? AND ku.nickname LIKE ?)",
		like,
		like,
		false,
		like,
		model.MemberStateJoined,
		false,
		like,
	)
}

func applyMemberActivityKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return query
	}
	like := "%" + keyword + "%"
	actionFilters := memberActivityActionFilters(keyword)
	if len(actionFilters) > 0 {
		return query.Where("u.nickname LIKE ? OR u.steam_id LIKE ? OR a.remark LIKE ? OR a.action IN ?", like, like, like, actionFilters)
	}
	return query.Where("u.nickname LIKE ? OR u.steam_id LIKE ? OR a.remark LIKE ?", like, like, like)
}

func memberActivityActionFilters(keyword string) []uint8 {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	actions := make([]uint8, 0, 2)
	if strings.Contains(keyword, "join") || strings.Contains(keyword, "加入") || strings.Contains(keyword, "上车") {
		actions = append(actions, model.MemberActionJoin)
	}
	if strings.Contains(keyword, "leave") || strings.Contains(keyword, "退出") || strings.Contains(keyword, "跳车") || strings.Contains(keyword, "下车") {
		actions = append(actions, model.MemberActionLeave)
	}
	return actions
}

func applyTimeFilter(query *gorm.DB, c *gin.Context) (*gorm.DB, error) {
	filter := c.DefaultQuery("timeFilter", "all")
	today := truncateDate(time.Now())
	switch filter {
	case "", "all":
		return query, nil
	case "today":
		return applyDateHit(query, today), nil
	case "tomorrow":
		return applyDateHit(query, today.AddDate(0, 0, 1)), nil
	case "this_week":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := today.AddDate(0, 0, 1-weekday)
		end := start.AddDate(0, 0, 6)
		return applyRangeHit(query, start, end), nil
	case "daily":
		return query.Where("schedule_type = ?", model.ScheduleDaily), nil
	case "date_range":
		return query.Where("schedule_type = ?", model.ScheduleDateRange), nil
	case "custom_range":
		start, err := parseOptionalDate(stringPtr(c.Query("startDate")))
		if err != nil || start == nil {
			return nil, errors.New("开始日期格式不正确")
		}
		end, err := parseOptionalDate(stringPtr(c.Query("endDate")))
		if err != nil || end == nil {
			return nil, errors.New("结束日期格式不正确")
		}
		if end.Before(*start) {
			return nil, errors.New("结束日期不能早于开始日期")
		}
		return applyRangeHit(query, *start, *end), nil
	default:
		return nil, errors.New("筛选条件不正确")
	}
}

func applyDateHit(query *gorm.DB, day time.Time) *gorm.DB {
	date := day.Format("2006-01-02")
	return query.Where(
		"(schedule_type = ? AND start_date = ?) OR schedule_type = ? OR (schedule_type = ? AND start_date <= ? AND end_date >= ?)",
		model.ScheduleSpecifiedDate, date,
		model.ScheduleDaily,
		model.ScheduleDateRange, date, date,
	)
}

func applyRangeHit(query *gorm.DB, start, end time.Time) *gorm.DB {
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")
	return query.Where(
		"(schedule_type = ? AND start_date BETWEEN ? AND ?) OR schedule_type = ? OR (schedule_type = ? AND start_date <= ? AND end_date >= ?)",
		model.ScheduleSpecifiedDate, startStr, endStr,
		model.ScheduleDaily,
		model.ScheduleDateRange, endStr, startStr,
	)
}

func applyTakeoverEndedFilter(query *gorm.DB, ended bool, _ time.Time) *gorm.DB {
	if ended {
		return query.Where("takeover_state = ?", model.TakeoverStateClosed)
	}
	return query.Where("takeover_state = ?", model.TakeoverStateNormal)
}

func syncExpiredTakeovers(db *gorm.DB, now time.Time) error {
	expired, args := takeoverExpiredWhere(now)
	params := append([]interface{}{false, model.TakeoverStateNormal}, args...)
	return db.Model(&model.Takeover{}).
		Where("is_deleted = ? AND takeover_state = ? AND ("+expired+")", params...).
		Update("takeover_state", model.TakeoverStateClosed).Error
}

func takeoverExpiredWhere(now time.Time) (string, []interface{}) {
	date := now.Format("2006-01-02")
	clock := now.Format("15:04:05")
	return "(schedule_type = ? AND (start_date < ? OR (start_date = ? AND play_time < ?))) OR (schedule_type = ? AND (end_date < ? OR (end_date = ? AND play_time < ?)))", []interface{}{
		model.ScheduleSpecifiedDate, date, date, clock,
		model.ScheduleDateRange, date, date, clock,
	}
}

func pathUint64(c *gin.Context, name string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	return value, err == nil && value > 0
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
