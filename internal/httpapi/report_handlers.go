package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type takeoverReportInput struct {
	ReportedUserID uint64   `json:"reportedUserId"`
	ReportType     string   `json:"reportType"`
	Content        string   `json:"content"`
	ImageURLs      []string `json:"imageUrls"`
}

func (h *Handler) ReportTakeoverMember(c *gin.Context) {
	user, _ := currentUser(c)
	if !ensureUserAllowed(c, user, true) {
		return
	}

	takeoverID, okID := pathUint64(c, "takeoverId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}

	var req takeoverReportInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}

	content := strings.TrimSpace(req.Content)
	reportType, reportTypeErr := normalizeReportType(req.ReportType)
	imageURLs, imageErr := normalizeReportImageURLs(req.ImageURLs)
	if req.ReportedUserID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	if req.ReportedUserID == user.ID {
		fail(c, http.StatusBadRequest, CodeCannotReportSelf, "不能举报自己")
		return
	}
	if content == "" || len([]rune(content)) > 500 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "report content is required and must be at most 500 characters")
		return
	}
	if reportTypeErr != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, reportTypeErr.Error())
		return
	}
	if imageErr != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, imageErr.Error())
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

	var reporterCount int64
	if err := h.db.Model(&model.TakeoverMember{}).
		Where("takeover_id = ? AND user_id = ? AND member_state = ?", takeoverID, user.ID, model.MemberStateJoined).
		Count(&reporterCount).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if reporterCount == 0 {
		fail(c, http.StatusForbidden, CodeReporterNotInTakeover, "只有当前队伍成员可以举报")
		return
	}

	var memberCount int64
	if err := h.db.Table("ttw_takeover_member AS m").
		Joins("JOIN ttw_user AS u ON u.id = m.user_id").
		Where("m.takeover_id = ? AND m.user_id = ? AND u.is_deleted = ?", takeoverID, req.ReportedUserID, false).
		Count(&memberCount).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if memberCount == 0 {
		fail(c, http.StatusBadRequest, CodeReportedUserNotInTakeover, "被举报用户不在该接龙中")
		return
	}

	var reportCount int64
	if err := h.db.Model(&model.TakeoverReport{}).
		Where("takeover_id = ? AND reporter_user_id = ? AND reported_user_id = ?", takeoverID, user.ID, req.ReportedUserID).
		Count(&reportCount).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if reportCount > 0 {
		fail(c, http.StatusConflict, CodeReportAlreadyExists, "已举报过该用户")
		return
	}

	report := model.TakeoverReport{
		TakeoverID:     takeoverID,
		ReporterUserID: user.ID,
		ReportedUserID: req.ReportedUserID,
		ReportType:     reportType,
		ReportContent:  content,
		ImageURLs:      reportImageURLsJSON(imageURLs),
		ReportState:    model.ReportStatePending,
	}
	if err := h.db.Create(&report).Error; err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			fail(c, http.StatusConflict, CodeReportAlreadyExists, "已举报过该用户")
			return
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "report failed")
		return
	}

	ok(c, "reported", nil)
}

func (h *Handler) AdminCreateReport(c *gin.Context) {
	var req struct {
		TakeoverID     uint64 `json:"takeoverId"`
		ReporterUserID uint64 `json:"reporterUserId"`
		takeoverReportInput
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}

	content := strings.TrimSpace(req.Content)
	reportType, reportTypeErr := normalizeReportType(req.ReportType)
	imageURLs, imageErr := normalizeReportImageURLs(req.ImageURLs)
	if req.TakeoverID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid takeover id")
		return
	}
	if req.ReporterUserID == 0 || req.ReportedUserID == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid user id")
		return
	}
	if req.ReporterUserID == req.ReportedUserID {
		fail(c, http.StatusBadRequest, CodeCannotReportSelf, "不能举报自己")
		return
	}
	if content == "" || len([]rune(content)) > 500 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "report content is required and must be at most 500 characters")
		return
	}
	if reportTypeErr != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, reportTypeErr.Error())
		return
	}
	if imageErr != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, imageErr.Error())
		return
	}

	report := model.TakeoverReport{
		TakeoverID:     req.TakeoverID,
		ReporterUserID: req.ReporterUserID,
		ReportedUserID: req.ReportedUserID,
		ReportType:     reportType,
		ReportContent:  content,
		ImageURLs:      reportImageURLsJSON(imageURLs),
		ReportState:    model.ReportStatePending,
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var takeoverCount int64
		if err := tx.Model(&model.Takeover{}).Where("id = ? AND is_deleted = ?", req.TakeoverID, false).Count(&takeoverCount).Error; err != nil {
			return err
		}
		if takeoverCount == 0 {
			return gorm.ErrRecordNotFound
		}
		var memberCount int64
		if err := tx.Table("ttw_takeover_member AS m").
			Joins("JOIN ttw_user AS u ON u.id = m.user_id").
			Where("m.takeover_id = ? AND m.user_id IN ? AND u.is_deleted = ?", req.TakeoverID, []uint64{req.ReporterUserID, req.ReportedUserID}, false).
			Distinct("m.user_id").
			Count(&memberCount).Error; err != nil {
			return err
		}
		if memberCount != 2 {
			return errReportedUserNotInTakeover
		}
		var reportCount int64
		if err := tx.Model(&model.TakeoverReport{}).
			Where("takeover_id = ? AND reporter_user_id = ? AND reported_user_id = ?", req.TakeoverID, req.ReporterUserID, req.ReportedUserID).
			Count(&reportCount).Error; err != nil {
			return err
		}
		if reportCount > 0 {
			return errReportAlreadyExists
		}
		if err := tx.Create(&report).Error; err != nil {
			if strings.Contains(err.Error(), "Duplicate entry") {
				return errReportAlreadyExists
			}
			return err
		}
		content := "create report"
		return tx.Create(&model.AdminOperateLog{
			OperateType:    "REPORT_CREATE",
			TargetType:     "report",
			TargetID:       report.ID,
			OperateContent: &content,
		}).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errReportAlreadyExists):
			fail(c, http.StatusConflict, CodeReportAlreadyExists, "已举报过该用户")
		case errors.Is(err, errReportedUserNotInTakeover):
			fail(c, http.StatusBadRequest, CodeReportedUserNotInTakeover, "举报人和被举报用户必须都在该接龙中")
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeTakeoverNotFound, "takeover not found")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "report failed")
		}
		return
	}

	ok(c, "created", gin.H{"id": report.ID})
}

func (h *Handler) AdminListReports(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	type reportRow struct {
		ID               uint64
		TakeoverID       uint64
		TakeoverTitle    string
		ReporterUserID   uint64
		ReporterNickname *string
		ReportedUserID   uint64
		ReportedNickname *string
		ReportedSteamID  *string
		ReportedCredit   uint
		ReportType       string
		ReportContent    string
		ImageURLs        *string
		ReportState      uint8
		PenaltyScore     uint
		HandleNote       *string
		HandledAt        *time.Time
		GmtCreate        time.Time
	}

	query := h.adminReportBaseQuery(c.Query("state"))
	reportTypeText := strings.TrimSpace(c.Query("reportType"))
	if reportTypeText != "" {
		reportType, err := normalizeReportType(reportTypeText)
		if err != nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
			return
		}
		query = query.Where("r.report_type = ?", reportType)
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword != "" {
		query = query.Where("r.report_content LIKE ? OR r.report_type LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	startDate, err := parseOptionalDate(stringPtr(c.Query("startDate")))
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	if startDate != nil {
		query = query.Where("r.gmt_create >= ?", truncateDate(*startDate))
	}
	endDate, err := parseOptionalDate(stringPtr(c.Query("endDate")))
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	if endDate != nil {
		if startDate != nil && endDate.Before(*startDate) {
			fail(c, http.StatusBadRequest, CodeParamInvalid, "结束日期不能早于开始日期")
			return
		}
		query = query.Where("r.gmt_create < ?", truncateDate(*endDate).AddDate(0, 0, 1))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var rows []reportRow
	if err := query.Select("r.id, r.takeover_id, t.title AS takeover_title, r.reporter_user_id, reporter.nickname AS reporter_nickname, r.reported_user_id, reported.nickname AS reported_nickname, reported.steam_id AS reported_steam_id, reported.credit_score AS reported_credit, r.report_type, r.report_content, r.image_urls, r.report_state, r.penalty_score, r.handle_note, r.handled_at, r.gmt_create").
		Order("r.gmt_create DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		imageURLs := reportImageURLs(row.ImageURLs)
		list = append(list, gin.H{
			"id":                   row.ID,
			"takeoverId":           row.TakeoverID,
			"takeoverTitle":        row.TakeoverTitle,
			"reporterUserId":       row.ReporterUserID,
			"reporterNickname":     stringValue(row.ReporterNickname),
			"reportedUserId":       row.ReportedUserID,
			"reportedNickname":     stringValue(row.ReportedNickname),
			"reportedSteamId":      stringValue(row.ReportedSteamID),
			"reportedCreditScore":  row.ReportedCredit,
			"reportedCreditStatus": creditStatus(row.ReportedCredit),
			"reportType":           row.ReportType,
			"reportTypeLabel":      reportTypeLabel(row.ReportType),
			"content":              row.ReportContent,
			"imageUrls":            imageURLs,
			"state":                row.ReportState,
			"penaltyScore":         row.PenaltyScore,
			"handleNote":           stringValue(row.HandleNote),
			"handledAt":            timeString(row.HandledAt),
			"createdAt":            row.GmtCreate.Format("2006-01-02 15:04:05"),
		})
	}

	ok(c, "success", gin.H{"page": page, "pageSize": pageSize, "total": total, "list": list})
}

func (h *Handler) adminReportBaseQuery(state string) *gorm.DB {
	query := h.db.Table("ttw_takeover_report AS r").
		Joins("JOIN ttw_takeover AS t ON t.id = r.takeover_id").
		Joins("JOIN ttw_user AS reporter ON reporter.id = r.reporter_user_id").
		Joins("JOIN ttw_user AS reported ON reported.id = r.reported_user_id")
	if reportState, ok := reportStateFilter(state); ok {
		query = query.Where("r.report_state = ?", reportState)
	}
	return query
}

func reportStateFilter(state string) (uint8, bool) {
	switch strings.TrimSpace(state) {
	case "", "pending":
		return model.ReportStatePending, true
	case "approved":
		return model.ReportStatePenalized, true
	case "rejected":
		return model.ReportStateIgnored, true
	default:
		return 0, false
	}
}

func (h *Handler) AdminGetReport(c *gin.Context) {
	reportID, okID := pathUint64(c, "reportId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid report id")
		return
	}

	type reportDetailRow struct {
		ID               uint64
		TakeoverID       uint64
		TakeoverTitle    string
		ReporterUserID   uint64
		ReporterNickname *string
		ReporterSteamID  *string
		ReportedUserID   uint64
		ReportedNickname *string
		ReportedSteamID  *string
		ReportedCredit   uint
		ReportType       string
		ReportContent    string
		ImageURLs        *string
		ReportState      uint8
		PenaltyScore     uint
		HandleNote       *string
		HandledAt        *time.Time
		GmtCreate        time.Time
	}

	var row reportDetailRow
	if err := h.db.Table("ttw_takeover_report AS r").
		Select("r.id, r.takeover_id, t.title AS takeover_title, r.reporter_user_id, reporter.nickname AS reporter_nickname, reporter.steam_id AS reporter_steam_id, r.reported_user_id, reported.nickname AS reported_nickname, reported.steam_id AS reported_steam_id, reported.credit_score AS reported_credit, r.report_type, r.report_content, r.image_urls, r.report_state, r.penalty_score, r.handle_note, r.handled_at, r.gmt_create").
		Joins("JOIN ttw_takeover AS t ON t.id = r.takeover_id").
		Joins("JOIN ttw_user AS reporter ON reporter.id = r.reporter_user_id").
		Joins("JOIN ttw_user AS reported ON reported.id = r.reported_user_id").
		Where("r.id = ?", reportID).
		Scan(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if row.ID == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "report not found")
		return
	}
	ok(c, "success", gin.H{
		"id":                   row.ID,
		"takeoverId":           row.TakeoverID,
		"takeoverTitle":        row.TakeoverTitle,
		"reporterUserId":       row.ReporterUserID,
		"reporterNickname":     stringValue(row.ReporterNickname),
		"reporterSteamId":      stringValue(row.ReporterSteamID),
		"reportedUserId":       row.ReportedUserID,
		"reportedNickname":     stringValue(row.ReportedNickname),
		"reportedSteamId":      stringValue(row.ReportedSteamID),
		"reportedCreditScore":  row.ReportedCredit,
		"reportedCreditStatus": creditStatus(row.ReportedCredit),
		"reportType":           row.ReportType,
		"reportTypeLabel":      reportTypeLabel(row.ReportType),
		"content":              row.ReportContent,
		"imageUrls":            reportImageURLs(row.ImageURLs),
		"state":                row.ReportState,
		"penaltyScore":         row.PenaltyScore,
		"handleNote":           stringValue(row.HandleNote),
		"handledAt":            timeString(row.HandledAt),
		"createdAt":            row.GmtCreate.Format("2006-01-02 15:04:05"),
	})
}

func (h *Handler) AdminApproveReport(c *gin.Context) {
	reportID, okID := pathUint64(c, "reportId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid report id")
		return
	}
	var req struct {
		Content      string `json:"content"`
		PenaltyScore uint   `json:"penaltyScore"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	note := strings.TrimSpace(req.Content)
	if len([]rune(note)) > 500 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "handle note must be at most 500 characters")
		return
	}
	if req.PenaltyScore == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "penalty score is required")
		return
	}
	if err := h.handleReport(c, reportID, model.ReportStatePenalized, req.PenaltyScore, note); err != nil {
		h.failReportHandle(c, err)
		return
	}
	ok(c, "approved", nil)
}

func (h *Handler) AdminRejectReport(c *gin.Context) {
	reportID, okID := pathUint64(c, "reportId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid report id")
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
	if len([]rune(reason)) > 500 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "handle note must be at most 500 characters")
		return
	}
	if err := h.handleReport(c, reportID, model.ReportStateIgnored, 0, reason); err != nil {
		h.failReportHandle(c, err)
		return
	}
	ok(c, "rejected", nil)
}

func (h *Handler) AdminHandleReport(c *gin.Context) {
	reportID, okID := pathUint64(c, "reportId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid report id")
		return
	}

	var req struct {
		PenaltyScore uint   `json:"penaltyScore"`
		HandleNote   string `json:"handleNote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	note := strings.TrimSpace(req.HandleNote)
	if len([]rune(note)) > 500 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "handle note must be at most 500 characters")
		return
	}
	if req.PenaltyScore != 0 && req.PenaltyScore != 5 && req.PenaltyScore != 10 && req.PenaltyScore != 20 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "penalty score must be 0, 5, 10, or 20")
		return
	}

	newState := uint8(model.ReportStateIgnored)
	if req.PenaltyScore > 0 {
		newState = model.ReportStatePenalized
	}
	if err := h.handleReport(c, reportID, newState, req.PenaltyScore, note); err != nil {
		h.failReportHandle(c, err)
		return
	}

	ok(c, "handled", nil)
}

func (h *Handler) failReportHandle(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errReportHandled):
		fail(c, http.StatusConflict, CodeParamInvalid, "report already handled")
	case isNotFound(err):
		fail(c, http.StatusNotFound, CodeParamInvalid, "report not found")
	default:
		fail(c, http.StatusInternalServerError, CodeSystemError, "report handle failed")
	}
}

func (h *Handler) handleReport(c *gin.Context, reportID uint64, state uint8, penaltyScore uint, note string) error {
	admin, _ := currentAdmin(c)
	now := time.Now()
	return h.db.Transaction(func(tx *gorm.DB) error {
		var report model.TakeoverReport
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", reportID).First(&report).Error; err != nil {
			return err
		}
		if report.ReportState != model.ReportStatePending {
			return errReportHandled
		}

		if state == model.ReportStatePenalized {
			var user model.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_deleted = ?", report.ReportedUserID, false).First(&user).Error; err != nil {
				return err
			}
			score := creditScoreAfterPenalty(user.CreditScore, penaltyScore)
			if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Update("credit_score", score).Error; err != nil {
				return err
			}
			if err := recordCreditLog(tx, user.ID, user.CreditScore, score, "report_penalty", firstNonEmpty(note, "举报核实扣分"), nullableUint64(admin.ID), nullableUint64(report.ID)); err != nil {
				return err
			}
		}

		updates := map[string]interface{}{
			"report_state":        state,
			"penalty_score":       penaltyScore,
			"handle_note":         nullableString(note),
			"handled_by_admin_id": nullableUint64(admin.ID),
			"handled_at":          now,
		}
		if err := tx.Model(&model.TakeoverReport{}).Where("id = ?", report.ID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminOperateLog{
			OperateType:    "REPORT_HANDLE",
			TargetType:     "report",
			TargetID:       report.ID,
			OperateContent: stringPtr(note),
		}).Error
	})
}

var (
	errReportHandled             = errors.New("report already handled")
	errReportAlreadyExists       = errors.New("report already exists")
	errReportedUserNotInTakeover = errors.New("reported user not in takeover")
)

func creditScoreAfterPenalty(score, penaltyScore uint) uint {
	if score > penaltyScore {
		return score - penaltyScore
	}
	return 0
}

func normalizeReportImageURLs(imageURLs []string) ([]string, error) {
	if len(imageURLs) > 9 {
		return nil, errors.New("report images must be at most 9")
	}

	result := make([]string, 0, len(imageURLs))
	for _, value := range imageURLs {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, errors.New("report image url is required")
		}
		if len([]rune(trimmed)) > 512 {
			return nil, errors.New("report image url must be at most 512 characters")
		}
		result = append(result, trimmed)
	}
	return result, nil
}

func normalizeReportType(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "":
		return model.ReportTypeOther, nil
	case model.ReportTypeNoShow:
		return model.ReportTypeNoShow, nil
	case model.ReportTypeLeaveEarly:
		return model.ReportTypeLeaveEarly, nil
	case model.ReportTypeDisruptive:
		return model.ReportTypeDisruptive, nil
	case model.ReportTypeOffensive:
		return model.ReportTypeOffensive, nil
	case model.ReportTypeOther:
		return model.ReportTypeOther, nil
	default:
		return "", errors.New("report type invalid")
	}
}

func reportTypeLabel(value string) string {
	switch value {
	case model.ReportTypeNoShow:
		return "到点不来"
	case model.ReportTypeLeaveEarly:
		return "中途跳车"
	case model.ReportTypeDisruptive:
		return "消极捣乱"
	case model.ReportTypeOffensive:
		return "言语攻击"
	default:
		return "其他"
	}
}

func reportImageURLsJSON(imageURLs []string) *string {
	if len(imageURLs) == 0 {
		return nil
	}
	data, err := json.Marshal(imageURLs)
	if err != nil {
		return nil
	}
	value := string(data)
	return &value
}

func reportImageURLs(imageURLsJSON *string) []string {
	if imageURLsJSON != nil && strings.TrimSpace(*imageURLsJSON) != "" {
		var imageURLs []string
		if err := json.Unmarshal([]byte(*imageURLsJSON), &imageURLs); err == nil && len(imageURLs) > 0 {
			return imageURLs
		}
	}
	return []string{}
}

func timeString(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableUint64(value uint64) *uint64 {
	if value == 0 {
		return nil
	}
	return &value
}
