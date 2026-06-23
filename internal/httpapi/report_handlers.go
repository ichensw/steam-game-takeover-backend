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

	var memberCount int64
	if err := h.db.Table("ttw_takeover_member AS m").
		Joins("JOIN ttw_user AS u ON u.id = m.user_id").
		Where("m.takeover_id = ? AND m.user_id = ? AND m.member_state = ? AND u.is_blocked = ? AND u.is_deleted = ?", takeoverID, req.ReportedUserID, model.MemberStateJoined, false, false).
		Count(&memberCount).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if memberCount == 0 {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "reported user is not in takeover")
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

	var imageURLPtr *string
	if len(imageURLs) > 0 {
		imageURLPtr = &imageURLs[0]
	}

	report := model.TakeoverReport{
		TakeoverID:     takeoverID,
		ReporterUserID: user.ID,
		ReportedUserID: req.ReportedUserID,
		ReportContent:  content,
		ImageURL:       imageURLPtr,
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

func (h *Handler) AdminListReports(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("pageSize"), 20)
	if pageSize > 50 {
		pageSize = 50
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
		ReportContent    string
		ImageURL         *string
		ImageURLs        *string
		ReportState      uint8
		PenaltyScore     uint
		HandleNote       *string
		HandledAt        *time.Time
		GmtCreate        time.Time
	}

	query := h.db.Table("ttw_takeover_report AS r").
		Joins("JOIN ttw_takeover AS t ON t.id = r.takeover_id").
		Joins("JOIN ttw_user AS reporter ON reporter.id = r.reporter_user_id").
		Joins("JOIN ttw_user AS reported ON reported.id = r.reported_user_id")
	state := strings.TrimSpace(c.Query("state"))
	if state == "" || state == "pending" {
		query = query.Where("r.report_state = ?", model.ReportStatePending)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var rows []reportRow
	if err := query.Select("r.id, r.takeover_id, t.title AS takeover_title, r.reporter_user_id, reporter.nickname AS reporter_nickname, r.reported_user_id, reported.nickname AS reported_nickname, reported.steam_id AS reported_steam_id, reported.credit_score AS reported_credit, r.report_content, r.image_url, r.image_urls, r.report_state, r.penalty_score, r.handle_note, r.handled_at, r.gmt_create").
		Order("r.gmt_create DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	list := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		imageURLs := reportImageURLs(row.ImageURL, row.ImageURLs)
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
			"content":              row.ReportContent,
			"imageUrl":             firstReportImageURL(imageURLs),
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

	admin, _ := currentUser(c)
	now := time.Now()
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var report model.TakeoverReport
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", reportID).First(&report).Error; err != nil {
			return err
		}
		if report.ReportState != model.ReportStatePending {
			return errReportHandled
		}

		newState := model.ReportStateIgnored
		if req.PenaltyScore > 0 {
			newState = model.ReportStatePenalized

			var user model.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_deleted = ?", report.ReportedUserID, false).First(&user).Error; err != nil {
				return err
			}
			score := uint(0)
			if user.CreditScore > req.PenaltyScore {
				score = user.CreditScore - req.PenaltyScore
			}
			if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Update("credit_score", score).Error; err != nil {
				return err
			}
		}

		updates := map[string]interface{}{
			"report_state":        newState,
			"penalty_score":       req.PenaltyScore,
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
	if err != nil {
		switch {
		case errors.Is(err, errReportHandled):
			fail(c, http.StatusConflict, CodeParamInvalid, "report already handled")
		case isNotFound(err):
			fail(c, http.StatusNotFound, CodeParamInvalid, "report not found")
		default:
			fail(c, http.StatusInternalServerError, CodeSystemError, "report handle failed")
		}
		return
	}

	ok(c, "handled", nil)
}

var errReportHandled = errors.New("report already handled")

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

func reportImageURLs(legacyImageURL *string, imageURLsJSON *string) []string {
	if imageURLsJSON != nil && strings.TrimSpace(*imageURLsJSON) != "" {
		var imageURLs []string
		if err := json.Unmarshal([]byte(*imageURLsJSON), &imageURLs); err == nil && len(imageURLs) > 0 {
			return imageURLs
		}
	}
	legacy := strings.TrimSpace(stringValue(legacyImageURL))
	if legacy == "" {
		return []string{}
	}
	return []string{legacy}
}

func firstReportImageURL(imageURLs []string) string {
	if len(imageURLs) == 0 {
		return ""
	}
	return imageURLs[0]
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
