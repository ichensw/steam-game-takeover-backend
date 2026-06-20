package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *Handler) ListTakeovers(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}

	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 10)
	if pageSize > 50 {
		pageSize = 50
	}

	query := h.db.Model(&model.Takeover{}).
		Where("is_deleted = ? AND takeover_state = ?", false, model.TakeoverStateNormal)
	query = applyKeywordFilter(query, c.Query("keyword"))
	var err error
	query, err = applyTimeFilter(query, c)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var takeovers []model.Takeover
	if err := query.Order("gmt_create DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&takeovers).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]takeoverDTO, 0, len(takeovers))
	for _, takeover := range takeovers {
		joinedCount, hasJoined, err := h.takeoverStats(takeover.ID, user.ID)
		if err != nil {
			fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
			return
		}
		dto := toTakeoverDTO(takeover, joinedCount, hasJoined)
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

func (h *Handler) GetTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, false) {
		return
	}
	h.getTakeoverDetail(c, false, user.ID)
}

func (h *Handler) AdminGetTakeover(c *gin.Context) {
	h.getTakeoverDetail(c, true, 0)
}

func (h *Handler) getTakeoverDetail(c *gin.Context, includeOpenID bool, currentUserID uint64) {
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

	joinedCount, hasJoined, err := h.takeoverStats(takeover.ID, currentUserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	members, err := h.takeoverMembers(takeover.ID, includeOpenID, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	dto := toTakeoverDTO(takeover, joinedCount, hasJoined)
	dto.Members = members
	ok(c, "success", dto)
}

func (h *Handler) CreateTakeover(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, true) {
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

	var takeover model.Takeover
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		takeover = model.Takeover{
			CreatorUserID:    user.ID,
			Title:            parsed.Title,
			ParticipantLimit: parsed.ParticipantLimit,
			ScheduleType:     parsed.ScheduleType,
			StartDate:        parsed.StartDate,
			EndDate:          parsed.EndDate,
			PlayTime:         parsed.PlayTime,
			Description:      parsed.Description,
			TakeoverState:    model.TakeoverStateNormal,
		}
		if err := tx.Create(&takeover).Error; err != nil {
			return err
		}

		member := model.TakeoverMember{
			TakeoverID:  takeover.ID,
			UserID:      user.ID,
			MemberState: model.MemberStateJoined,
		}
		return tx.Create(&member).Error
	}); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "create failed")
		return
	}
	ok(c, "created", gin.H{"id": takeover.ID, "hasJoined": true, "joinedCount": 1})
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

	var joinedCount int64
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var takeover model.Takeover
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = ? AND takeover_state = ?", takeoverID, false, model.TakeoverStateNormal).
			First(&takeover).Error; err != nil {
			return err
		}

		var member model.TakeoverMember
		err := tx.Where("takeover_id = ? AND user_id = ?", takeoverID, user.ID).First(&member).Error
		if err == nil && member.MemberState == model.MemberStateJoined {
			return errAlreadyJoined
		}
		if err != nil && !isNotFound(err) {
			return err
		}

		if err := tx.Model(&model.TakeoverMember{}).
			Where("takeover_id = ? AND member_state = ?", takeoverID, model.MemberStateJoined).
			Count(&joinedCount).Error; err != nil {
			return err
		}
		if uint(joinedCount) >= takeover.ParticipantLimit {
			return errTakeoverFull
		}

		if member.ID != 0 {
			if err := tx.Model(&model.TakeoverMember{}).Where("id = ?", member.ID).Update("member_state", model.MemberStateJoined).Error; err != nil {
				return err
			}
		} else {
			member = model.TakeoverMember{TakeoverID: takeoverID, UserID: user.ID, MemberState: model.MemberStateJoined}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
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
		case errors.Is(err, errTakeoverFull):
			fail(c, http.StatusConflict, CodeTakeoverFull, "takeover full")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "join failed")
		}
		return
	}

	ok(c, "joined", gin.H{"hasJoined": true, "joinedCount": joinedCount})
}

var (
	errAlreadyJoined = errors.New("already joined")
	errTakeoverFull  = errors.New("takeover full")
)

func ensureUserAllowed(c *gin.Context, user model.User, requireProfile bool) bool {
	if user.IsBlocked {
		fail(c, http.StatusForbidden, CodeUserBlocked, "user blocked")
		return false
	}
	if requireProfile && !user.IsProfileCompleted {
		fail(c, http.StatusForbidden, CodeProfileIncomplete, "profile incomplete")
		return false
	}
	return true
}

func (h *Handler) takeoverStats(takeoverID uint64, userID uint64) (int64, bool, error) {
	var joinedCount int64
	if err := h.db.Model(&model.TakeoverMember{}).
		Where("takeover_id = ? AND member_state = ?", takeoverID, model.MemberStateJoined).
		Count(&joinedCount).Error; err != nil {
		return 0, false, err
	}
	if userID == 0 {
		return joinedCount, false, nil
	}
	var hasJoinedCount int64
	if err := h.db.Model(&model.TakeoverMember{}).
		Where("takeover_id = ? AND user_id = ? AND member_state = ?", takeoverID, userID, model.MemberStateJoined).
		Count(&hasJoinedCount).Error; err != nil {
		return 0, false, err
	}
	return joinedCount, hasJoinedCount > 0, nil
}

func (h *Handler) takeoverMembers(takeoverID uint64, includeOpenID bool, limit int) ([]memberDTO, error) {
	var rows []memberRow
	query := h.db.Table("ttw_takeover_member AS m").
		Select("u.id AS user_id, u.openid, u.nickname, u.steam_id, u.gender, u.avatar_url, m.gmt_create AS joined_at").
		Joins("JOIN ttw_user AS u ON u.id = m.user_id").
		Where("m.takeover_id = ? AND m.member_state = ?", takeoverID, model.MemberStateJoined).
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

func applyKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return query
	}
	like := "%" + keyword + "%"
	return query.Where("title LIKE ? OR description LIKE ?", like, like)
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
			return nil, errors.New("startDate must be YYYY-MM-DD")
		}
		end, err := parseOptionalDate(stringPtr(c.Query("endDate")))
		if err != nil || end == nil {
			return nil, errors.New("endDate must be YYYY-MM-DD")
		}
		if end.Before(*start) {
			return nil, errors.New("endDate cannot be before startDate")
		}
		return applyRangeHit(query, *start, *end), nil
	default:
		return nil, errors.New("invalid timeFilter")
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
