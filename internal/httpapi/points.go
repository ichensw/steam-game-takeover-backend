package httpapi

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	pointUnitsPerPoint              = int64(10)
	riderPointUnits                 = int64(10)
	takeoverPointSettlementBatch    = 100
	takeoverPointSettlementInterval = time.Minute
)

const (
	pointReasonTakeoverCreator = "takeover_creator"
	pointReasonTakeoverRider   = "takeover_rider"
	pointReasonReportReversal  = "report_reversal"
	pointReasonDeleteReversal  = "delete_reversal"
	pointReasonAdminAdjustment = "admin_adjustment"
)

type pointLevelInfo struct {
	Name         string  `json:"name"`
	NextName     string  `json:"nextName"`
	Progress     int     `json:"progress"`
	NextAtPoints float64 `json:"nextAtPoints"`
}

type pointRankingItem struct {
	Rank      int64   `json:"rank"`
	UserID    uint64  `json:"userId"`
	Nickname  string  `json:"nickname"`
	AvatarURL string  `json:"avatarUrl"`
	Points    float64 `json:"points"`
	Level     string  `json:"level"`
}

type pointRankingRow struct {
	UserID      uint64
	Nickname    *string
	AvatarURL   *string
	Gender      *uint8
	PointsUnits int64
}

func pointsValue(units int64) float64 {
	return float64(units) / float64(pointUnitsPerPoint)
}

func creatorPointUnits(joinedCount int, participantLimit uint) int64 {
	if participantLimit == 0 {
		return pointUnitsPerPoint
	}
	if joinedCount < 0 {
		joinedCount = 0
	}
	if joinedCount > int(participantLimit) {
		joinedCount = int(participantLimit)
	}
	return pointUnitsPerPoint + int64(math.Round(12*float64(joinedCount)/float64(participantLimit)))
}

func pointValueToUnits(value float64) (int64, bool) {
	units := math.Round(value * float64(pointUnitsPerPoint))
	if value == 0 || math.Abs(value*float64(pointUnitsPerPoint)-units) > 0.000001 {
		return 0, false
	}
	return int64(units), true
}

func pointLevel(units uint64) pointLevelInfo {
	type level struct {
		name string
		min  uint64
	}
	levels := []level{
		{name: "新人", min: 0},
		{name: "常客", min: 100},
		{name: "活跃", min: 300},
		{name: "核心", min: 600},
		{name: "领队", min: 1200},
		{name: "传奇", min: 2400},
	}
	for index := len(levels) - 1; index >= 0; index-- {
		current := levels[index]
		if units < current.min {
			continue
		}
		if index == len(levels)-1 {
			return pointLevelInfo{Name: current.name, Progress: 100, NextAtPoints: pointsValue(int64(current.min))}
		}
		next := levels[index+1]
		progress := int((units - current.min) * 100 / (next.min - current.min))
		return pointLevelInfo{
			Name:         current.name,
			NextName:     next.name,
			Progress:     progress,
			NextAtPoints: pointsValue(int64(next.min)),
		}
	}
	return pointLevelInfo{Name: levels[0].name}
}

func pointBusinessKey(takeoverID, userID uint64, kind string) string {
	return fmt.Sprintf("takeover:%d:user:%d:%s", takeoverID, userID, kind)
}

func applyPointDelta(
	tx *gorm.DB,
	userID uint64,
	delta int64,
	reasonType string,
	reason string,
	takeoverID *uint64,
	adminID *uint64,
	reportID *uint64,
	businessKey *string,
	effectiveAt time.Time,
) error {
	var user model.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND is_deleted = ?", userID, false).
		First(&user).Error; err != nil {
		return err
	}
	if businessKey != nil {
		var count int64
		if err := tx.Model(&model.UserPointLog{}).Where("business_key = ?", *businessKey).Limit(1).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
	}

	before := user.PointsUnits
	after := before
	if delta >= 0 {
		after += uint64(delta)
	} else {
		decrease := uint64(-delta)
		if decrease >= before {
			after = 0
		} else {
			after -= decrease
		}
	}
	actualDelta := int64(after) - int64(before)
	if after != before {
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Update("points_units", after).Error; err != nil {
			return err
		}
	}
	return tx.Create(&model.UserPointLog{
		UserID:           user.ID,
		PointDeltaUnits:  actualDelta,
		PointBeforeUnits: before,
		PointAfterUnits:  after,
		ReasonType:       reasonType,
		Reason:           stringPtr(reason),
		TakeoverID:       takeoverID,
		OperatorAdminID:  adminID,
		RelatedReportID:  reportID,
		BusinessKey:      businessKey,
		EffectiveAt:      effectiveAt,
	}).Error
}

func settleTakeoverPoints(db *gorm.DB, takeoverID uint64, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var takeover model.Takeover
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = ?", takeoverID, false).
			First(&takeover).Error; err != nil {
			return err
		}
		if takeover.PointsSettledAt != nil || takeover.ScheduleType != model.ScheduleSpecifiedDate {
			return nil
		}
		if takeover.StartDate == nil {
			return nil
		}
		playAt, err := combineDateAndPlayTime(*takeover.StartDate, takeover.PlayTime)
		if err != nil {
			return err
		}
		if playAt.After(now) {
			return nil
		}

		var joined []struct {
			UserID uint64
		}
		if err := tx.Table("ttw_takeover_member AS m").
			Select("m.user_id").
			Joins("JOIN ttw_user AS u ON u.id = m.user_id AND u.is_deleted = ?", false).
			Where("m.takeover_id = ? AND m.member_state = ?", takeover.ID, model.MemberStateJoined).
			Order("m.user_id ASC").
			Scan(&joined).Error; err != nil {
			return err
		}

		var penalizedUserIDs []uint64
		if err := tx.Model(&model.TakeoverReport{}).
			Where("takeover_id = ? AND report_state = ?", takeover.ID, model.ReportStatePenalized).
			Distinct().
			Pluck("reported_user_id", &penalizedUserIDs).Error; err != nil {
			return err
		}
		penalized := make(map[uint64]bool, len(penalizedUserIDs))
		for _, userID := range penalizedUserIDs {
			penalized[userID] = true
		}

		hasJoinedRider := false
		riderIDs := make([]uint64, 0, len(joined))
		for _, member := range joined {
			if member.UserID == takeover.CreatorUserID {
				continue
			}
			hasJoinedRider = true
			if !penalized[member.UserID] {
				riderIDs = append(riderIDs, member.UserID)
			}
		}
		if hasJoinedRider {
			if !penalized[takeover.CreatorUserID] {
				key := pointBusinessKey(takeover.ID, takeover.CreatorUserID, "creator")
				err := applyPointDelta(
					tx,
					takeover.CreatorUserID,
					creatorPointUnits(len(joined), takeover.ParticipantLimit),
					pointReasonTakeoverCreator,
					"发车积分",
					nullableUint64(takeover.ID),
					nil,
					nil,
					&key,
					playAt,
				)
				if err != nil && !isNotFound(err) {
					return err
				}
			}
			for _, userID := range riderIDs {
				key := pointBusinessKey(takeover.ID, userID, "rider")
				if err := applyPointDelta(
					tx,
					userID,
					riderPointUnits,
					pointReasonTakeoverRider,
					"上车积分",
					nullableUint64(takeover.ID),
					nil,
					nil,
					&key,
					playAt,
				); err != nil {
					return err
				}
			}
		}

		return tx.Model(&model.Takeover{}).Where("id = ?", takeover.ID).Updates(map[string]interface{}{
			"takeover_state":    model.TakeoverStateClosed,
			"points_settled_at": now,
		}).Error
	})
}

func settleDueSpecifiedTakeovers(db *gorm.DB, now time.Time) error {
	date := now.Format("2006-01-02")
	clock := now.Format("15:04:05")
	for {
		var ids []uint64
		if err := db.Model(&model.Takeover{}).
			Where(
				"schedule_type = ? AND is_deleted = ? AND points_settled_at IS NULL AND start_date IS NOT NULL AND (start_date < ? OR (start_date = ? AND play_time <= ?))",
				model.ScheduleSpecifiedDate,
				false,
				date,
				date,
				clock,
			).
			Order("id ASC").
			Limit(takeoverPointSettlementBatch).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		for _, id := range ids {
			if err := settleTakeoverPoints(db, id, now); err != nil {
				return err
			}
		}
		if len(ids) < takeoverPointSettlementBatch {
			return nil
		}
	}
}

func reverseTakeoverPointAward(
	tx *gorm.DB,
	takeoverID uint64,
	userID uint64,
	reasonType string,
	reason string,
	adminID *uint64,
	reportID *uint64,
) error {
	var award model.UserPointLog
	err := tx.Where(
		"takeover_id = ? AND user_id = ? AND reason_type IN ? AND point_delta_units > 0",
		takeoverID,
		userID,
		[]string{pointReasonTakeoverCreator, pointReasonTakeoverRider},
	).Order("id ASC").First(&award).Error
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	key := pointBusinessKey(takeoverID, userID, "reversal")
	return applyPointDelta(
		tx,
		userID,
		-award.PointDeltaUnits,
		reasonType,
		reason,
		nullableUint64(takeoverID),
		adminID,
		reportID,
		&key,
		award.EffectiveAt,
	)
}

func reverseAllTakeoverPointAwards(tx *gorm.DB, takeoverID uint64, adminID *uint64) error {
	var userIDs []uint64
	if err := tx.Model(&model.UserPointLog{}).
		Where("takeover_id = ? AND reason_type IN ? AND point_delta_units > 0", takeoverID, []string{pointReasonTakeoverCreator, pointReasonTakeoverRider}).
		Distinct().
		Order("user_id ASC").
		Pluck("user_id", &userIDs).Error; err != nil {
		return err
	}
	for _, userID := range userIDs {
		if err := reverseTakeoverPointAward(tx, takeoverID, userID, pointReasonDeleteReversal, "接龙删除撤回积分", adminID, nil); err != nil && !isNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteTakeoverWithPointReversal(tx *gorm.DB, takeoverID uint64, adminID *uint64) error {
	var takeover model.Takeover
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND is_deleted = ?", takeoverID, false).
		First(&takeover).Error; err != nil {
		return err
	}
	if err := reverseAllTakeoverPointAwards(tx, takeover.ID, adminID); err != nil {
		return err
	}
	return tx.Model(&model.Takeover{}).Where("id = ?", takeover.ID).Update("is_deleted", true).Error
}

func (h *Handler) StartTakeoverPointWorker(ctx context.Context) {
	run := func() {
		if err := settleDueSpecifiedTakeovers(h.db, time.Now()); err != nil {
			log.Printf("settle takeover points: %v", err)
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(takeoverPointSettlementInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func loadPointRankings(db *gorm.DB, period string, now time.Time) ([]pointRankingRow, error) {
	var rows []pointRankingRow
	if period == "month" {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, 0)
		err := db.Table("ttw_user_point_log AS l").
			Select("u.id AS user_id, u.nickname, u.avatar_url, u.gender, SUM(l.point_delta_units) AS points_units").
			Joins("JOIN ttw_user AS u ON u.id = l.user_id").
			Where("u.is_deleted = ? AND u.is_banned = ? AND l.effective_at >= ? AND l.effective_at < ?", false, false, start, end).
			Group("u.id, u.nickname, u.avatar_url, u.gender").
			Having("SUM(l.point_delta_units) > 0").
			Order("points_units DESC, user_id ASC").
			Scan(&rows).Error
		return rows, err
	}
	err := db.Model(&model.User{}).
		Select("id AS user_id, nickname, avatar_url, gender, points_units").
		Where("is_deleted = ? AND is_banned = ? AND points_units > 0", false, false).
		Order("points_units DESC, id ASC").
		Scan(&rows).Error
	return rows, err
}

func rankedPointItems(rows []pointRankingRow) []pointRankingItem {
	items := make([]pointRankingItem, 0, len(rows))
	var rank int64
	var previous int64 = -1
	for index, row := range rows {
		if row.PointsUnits != previous {
			rank = int64(index + 1)
			previous = row.PointsUnits
		}
		units := uint64(row.PointsUnits)
		items = append(items, pointRankingItem{
			Rank:      rank,
			UserID:    row.UserID,
			Nickname:  stringValue(row.Nickname),
			AvatarURL: normalizeAvatarURL(stringValue(row.AvatarURL), row.Gender),
			Points:    pointsValue(row.PointsUnits),
			Level:     pointLevel(units).Name,
		})
	}
	return items
}

func pointRankForUser(rows []pointRankingRow, userID uint64) int64 {
	var rank int64
	var previous int64 = -1
	for index, row := range rows {
		if row.PointsUnits != previous {
			rank = int64(index + 1)
			previous = row.PointsUnits
		}
		if row.UserID == userID {
			return rank
		}
	}
	return 0
}

func (h *Handler) pointRanksForUser(userID uint64, now time.Time) (int64, int64, error) {
	monthly, err := loadPointRankings(h.db, "month", now)
	if err != nil {
		return 0, 0, err
	}
	all, err := loadPointRankings(h.db, "all", now)
	if err != nil {
		return 0, 0, err
	}
	return pointRankForUser(monthly, userID), pointRankForUser(all, userID), nil
}

func (h *Handler) GetPointRankings(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if period != "month" && period != "all" {
		period = "month"
	}
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	rows, err := loadPointRankings(h.db, period, time.Now())
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	items := rankedPointItems(rows)
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	ok(c, "success", gin.H{
		"period":   period,
		"page":     page,
		"pageSize": pageSize,
		"total":    len(items),
		"list":     items[start:end],
	})
}

func pointLogDTO(row model.UserPointLog) gin.H {
	return gin.H{
		"id":              row.ID,
		"pointsDelta":     pointsValue(row.PointDeltaUnits),
		"pointsBefore":    pointsValue(int64(row.PointBeforeUnits)),
		"pointsAfter":     pointsValue(int64(row.PointAfterUnits)),
		"reasonType":      row.ReasonType,
		"reason":          stringValue(row.Reason),
		"takeoverId":      row.TakeoverID,
		"relatedReportId": row.RelatedReportID,
		"effectiveAt":     row.EffectiveAt.Format("2006-01-02 15:04:05"),
		"createdAt":       row.GmtCreate.Format("2006-01-02 15:04:05"),
	}
}

func (h *Handler) listPointLogs(c *gin.Context, userID uint64) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(firstNonEmpty(c.Query("page_size"), c.Query("pageSize")), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	query := h.db.Model(&model.UserPointLog{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	var rows []model.UserPointLog
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, pointLogDTO(row))
	}
	ok(c, "success", gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) ListMyPointLogs(c *gin.Context) {
	user, _ := currentUser(c)
	h.listPointLogs(c, user.ID)
}

func (h *Handler) AdminListUserPointLogs(c *gin.Context) {
	userID, okID := pathUint64(c, "userId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	h.listPointLogs(c, userID)
}

func (h *Handler) AdminAdjustUserPoints(c *gin.Context) {
	userID, okID := pathUint64(c, "userId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	admin, _ := currentAdmin(c)
	var req struct {
		Delta  float64 `json:"delta"`
		Reason string  `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	units, valid := pointValueToUnits(req.Delta)
	reason := strings.TrimSpace(req.Reason)
	if !valid || math.Abs(req.Delta) > 10000 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "delta must be non-zero, within 10000, and use at most one decimal")
		return
	}
	if reason == "" || len([]rune(reason)) > 255 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "reason is required and must be at most 255 characters")
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		return applyPointDelta(tx, userID, units, pointReasonAdminAdjustment, reason, nil, nullableUint64(admin.ID), nil, nil, time.Now())
	}); err != nil {
		if isNotFound(err) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "user not found")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "points adjustment failed")
		return
	}
	ok(c, "adjusted", nil)
}
