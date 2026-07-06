package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errFeedbackTypeInvalid    = errors.New("feedback_type invalid")
	errFeedbackContentInvalid = errors.New("feedback content invalid")
	errFeedbackContactInvalid = errors.New("feedback contact invalid")
	errFeedbackImagesInvalid  = errors.New("feedback images invalid")
	errFeedbackStatusInvalid  = errors.New("feedback status invalid")
)

type userFeedbackInput struct {
	FeedbackType string   `json:"feedback_type"`
	Content      string   `json:"content"`
	Contact      string   `json:"contact"`
	Images       []string `json:"images"`
}

type normalizedUserFeedback struct {
	FeedbackType string
	Content      string
	Contact      string
	Images       []string
}

type adminFeedbackFilters struct {
	Status       *uint8
	FeedbackType string
	Keyword      string
}

func (h *Handler) UploadUserFeedbackImage(c *gin.Context) {
	user, _ := currentUser(c)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "file is required")
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxUploadImageSize {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "image must be between 1 byte and 5 MB")
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	ext := imageExt(fileHeader, contentType)
	if ext == "" || ext == ".gif" {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "only jpg, png, and webp images are allowed")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "open upload failed")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "open upload failed")
		return
	}
	if err := h.checkImageSecurity(contentSecurityTarget{
		User:        user,
		ContentType: "feedback_image",
		Scene:       contentSceneProfile,
	}, fileHeader.Filename, data); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "content security reject")
		return
	}

	bucket, err := h.ossBucket()
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "oss not configured")
		return
	}
	objectKey := feedbackUploadObjectKey(user.ID, ext)
	if err := bucket.PutObject(objectKey, bytes.NewReader(data), oss.ContentType(contentType)); err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "upload failed")
		return
	}
	ok(c, "uploaded", gin.H{"url": h.ossObjectURL(objectKey)})
}

func (h *Handler) SubmitUserFeedback(c *gin.Context) {
	user, _ := currentUser(c)
	var req userFeedbackInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	feedback, err := normalizeUserFeedbackInput(req)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	images := feedbackImagesJSON(feedback.Images)
	record := model.UserFeedback{
		UserID:       user.ID,
		FeedbackType: feedback.FeedbackType,
		Content:      feedback.Content,
		Contact:      feedback.Contact,
		Images:       images,
		Status:       model.FeedbackStatusPending,
	}
	if err := h.db.Create(&record).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "反馈已提交，感谢你的建议", nil)
}

func (h *Handler) AdminListUserFeedbacks(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(firstNonEmpty(c.Query("page_size"), c.Query("pageSize")), 20)
	if pageSize > 50 {
		pageSize = 50
	}
	filters, err := normalizeAdminFeedbackFilters(c.Query("status"), c.Query("feedback_type"), c.Query("keyword"))
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	query := h.userFeedbackBaseQuery(filters)
	query, err = applyFeedbackTimeFilter(query, c.Query("start_time"), c.Query("end_time"))
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	var rows []userFeedbackRow
	if err := query.Select(userFeedbackListSelect()).
		Order("f.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	items := make([]userFeedbackDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, toUserFeedbackDTO(row, false))
	}
	ok(c, "success", gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminGetUserFeedback(c *gin.Context) {
	feedbackID, okID := pathUint64(c, "feedbackId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid feedback id")
		return
	}
	var row userFeedbackRow
	if err := h.userFeedbackBaseQuery(adminFeedbackFilters{}).
		Select(userFeedbackDetailSelect()).
		Where("f.id = ?", feedbackID).
		Scan(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if row.ID == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "feedback not found")
		return
	}
	ok(c, "success", toUserFeedbackDTO(row, true))
}

func (h *Handler) AdminUpdateUserFeedbackStatus(c *gin.Context) {
	feedbackID, okID := pathUint64(c, "feedbackId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid feedback id")
		return
	}
	var req struct {
		Status uint8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	status, err := normalizeFeedbackStatus(req.Status)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	result := h.db.Model(&model.UserFeedback{}).Where("id = ?", feedbackID).Update("status", status)
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "feedback not found")
		return
	}
	ok(c, "反馈状态已更新", nil)
}

func normalizeUserFeedbackInput(req userFeedbackInput) (normalizedUserFeedback, error) {
	feedbackType := strings.TrimSpace(req.FeedbackType)
	if !validFeedbackType(feedbackType) {
		return normalizedUserFeedback{}, errFeedbackTypeInvalid
	}
	content := strings.TrimSpace(req.Content)
	if content == "" || len([]rune(content)) > 500 {
		return normalizedUserFeedback{}, errFeedbackContentInvalid
	}
	contact := strings.TrimSpace(req.Contact)
	if len([]rune(contact)) > 100 {
		return normalizedUserFeedback{}, errFeedbackContactInvalid
	}
	images, err := normalizeFeedbackImages(req.Images)
	if err != nil {
		return normalizedUserFeedback{}, err
	}
	return normalizedUserFeedback{FeedbackType: feedbackType, Content: content, Contact: contact, Images: images}, nil
}

func normalizeFeedbackImages(images []string) ([]string, error) {
	if len(images) > 3 {
		return nil, errFeedbackImagesInvalid
	}
	result := make([]string, 0, len(images))
	for _, image := range images {
		value := strings.TrimSpace(image)
		parsed, err := url.ParseRequestURI(value)
		if value == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, errFeedbackImagesInvalid
		}
		result = append(result, value)
	}
	return result, nil
}

func normalizeFeedbackStatus(status uint8) (uint8, error) {
	switch status {
	case model.FeedbackStatusPending, model.FeedbackStatusAccepted, model.FeedbackStatusIgnored:
		return status, nil
	default:
		return 0, errFeedbackStatusInvalid
	}
}

func normalizeAdminFeedbackFilters(statusText, feedbackType, keyword string) (adminFeedbackFilters, error) {
	var filters adminFeedbackFilters
	statusText = strings.TrimSpace(statusText)
	if statusText != "" {
		value, err := strconv.ParseUint(statusText, 10, 8)
		if err != nil {
			return filters, errFeedbackStatusInvalid
		}
		status, err := normalizeFeedbackStatus(uint8(value))
		if err != nil {
			return filters, err
		}
		filters.Status = &status
	}
	feedbackType = strings.TrimSpace(feedbackType)
	if feedbackType != "" {
		if !validFeedbackType(feedbackType) {
			return filters, errFeedbackTypeInvalid
		}
		filters.FeedbackType = feedbackType
	}
	filters.Keyword = strings.TrimSpace(keyword)
	return filters, nil
}

func (h *Handler) userFeedbackBaseQuery(filters adminFeedbackFilters) *gorm.DB {
	query := h.db.Table("ttw_user_feedback AS f").Joins("LEFT JOIN ttw_user AS u ON u.id = f.user_id")
	if filters.Status != nil {
		query = query.Where("f.status = ?", *filters.Status)
	}
	if filters.FeedbackType != "" {
		query = query.Where("f.feedback_type = ?", filters.FeedbackType)
	}
	if filters.Keyword != "" {
		like := "%" + filters.Keyword + "%"
		query = query.Where("f.content LIKE ? OR f.contact LIKE ? OR u.nickname LIKE ?", like, like, like)
	}
	return query
}

func applyFeedbackTimeFilter(query *gorm.DB, startTimeText, endTimeText string) (*gorm.DB, error) {
	startTime, err := parseOptionalDateTime(startTimeText)
	if err != nil {
		return nil, err
	}
	if startTime != nil {
		query = query.Where("f.created_at >= ?", *startTime)
	}
	endTime, err := parseOptionalDateTime(endTimeText)
	if err != nil {
		return nil, err
	}
	if endTime != nil {
		if startTime != nil && endTime.Before(*startTime) {
			return nil, errors.New("end_time cannot be before start_time")
		}
		query = query.Where("f.created_at <= ?", *endTime)
	}
	return query, nil
}

func parseOptionalDateTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid time: %s", value)
}

func validFeedbackType(value string) bool {
	switch value {
	case "suggestion", "problem", "experience", "other":
		return true
	default:
		return false
	}
}

func feedbackImagesJSON(images []string) *string {
	if len(images) == 0 {
		return nil
	}
	data, _ := json.Marshal(images)
	value := string(data)
	return &value
}

func feedbackImages(value *string) []string {
	if value == nil || *value == "" {
		return []string{}
	}
	var images []string
	if err := json.Unmarshal([]byte(*value), &images); err != nil {
		return []string{}
	}
	return images
}

func feedbackStatusLabel(status uint8) string {
	switch status {
	case model.FeedbackStatusAccepted:
		return "已采纳"
	case model.FeedbackStatusIgnored:
		return "不理睬"
	default:
		return "待采纳"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func feedbackUploadObjectKey(userID uint64, ext string) string {
	now := time.Now()
	return fmt.Sprintf("miniapp/feedback/%04d/%02d/%d-%d-%s%s", now.Year(), now.Month(), userID, now.UnixNano(), randomHex(6), ext)
}

type userFeedbackRow struct {
	ID           uint64
	UserID       uint64
	Nickname     *string
	AvatarURL    *string
	SteamID      *string
	FeedbackType string
	Content      string
	Contact      string
	Images       *string
	Status       uint8
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type userFeedbackDTO struct {
	ID           uint64   `json:"id"`
	UserID       uint64   `json:"user_id"`
	Nickname     string   `json:"nickname"`
	AvatarURL    string   `json:"avatar_url"`
	SteamID      string   `json:"steam_id,omitempty"`
	FeedbackType string   `json:"feedback_type"`
	Content      string   `json:"content"`
	Contact      string   `json:"contact"`
	Images       []string `json:"images"`
	Status       uint8    `json:"status"`
	StatusLabel  string   `json:"status_label"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

func toUserFeedbackDTO(row userFeedbackRow, detail bool) userFeedbackDTO {
	dto := userFeedbackDTO{
		ID:           row.ID,
		UserID:       row.UserID,
		Nickname:     stringValue(row.Nickname),
		AvatarURL:    stringValue(row.AvatarURL),
		FeedbackType: row.FeedbackType,
		Content:      row.Content,
		Contact:      row.Contact,
		Images:       feedbackImages(row.Images),
		Status:       row.Status,
		StatusLabel:  feedbackStatusLabel(row.Status),
		CreatedAt:    row.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if detail {
		dto.SteamID = stringValue(row.SteamID)
		dto.UpdatedAt = row.UpdatedAt.Format("2006-01-02 15:04:05")
	}
	return dto
}

func userFeedbackListSelect() string {
	return "f.id, f.user_id, u.nickname, u.avatar_url, f.feedback_type, f.content, f.contact, f.images, f.status, f.created_at"
}

func userFeedbackDetailSelect() string {
	return "f.id, f.user_id, u.nickname, u.avatar_url, u.steam_id, f.feedback_type, f.content, f.contact, f.images, f.status, f.created_at, f.updated_at"
}
