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

var (
	errAnnouncementTitleInvalid   = errors.New("announcement title invalid")
	errAnnouncementContentInvalid = errors.New("announcement content invalid")
	errAnnouncementImageInvalid   = errors.New("announcement image invalid")
	errAnnouncementStatusInvalid  = errors.New("announcement status invalid")
	errAnnouncementTimeInvalid    = errors.New("announcement time invalid")
)

type announcementInput struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	ImageURL  string `json:"image_url"`
	Status    uint8  `json:"status"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type normalizedAnnouncement struct {
	Title     string
	Content   string
	ImageURL  *string
	Status    uint8
	StartTime time.Time
	EndTime   *time.Time
}

func (h *Handler) GetCurrentAnnouncement(c *gin.Context) {
	user, _ := currentUser(c)
	now := time.Now()
	var announcement model.Announcement
	err := h.db.Table("ttw_announcement AS a").
		Where("a.status = ?", model.AnnouncementStatusEnabled).
		Where("a.start_time <= ?", now).
		Where("a.end_time IS NULL OR a.end_time >= ?", now).
		Where("NOT EXISTS (?)",
			h.db.Table("ttw_user_announcement_read AS r").
				Select("1").
				Where("r.user_id = ? AND r.announcement_id = a.id", user.ID),
		).
		Order("a.gmt_create DESC, a.id DESC").
		Limit(1).
		Scan(&announcement).Error
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if announcement.ID == 0 {
		ok(c, "success", nil)
		return
	}
	ok(c, "success", toAnnouncementDTO(announcement))
}

func (h *Handler) MarkAnnouncementRead(c *gin.Context) {
	user, _ := currentUser(c)
	announcementID, okID := pathUint64(c, "announcementId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid announcement id")
		return
	}
	var count int64
	if err := h.db.Model(&model.Announcement{}).Where("id = ?", announcementID).Count(&count).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	if count == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "announcement not found")
		return
	}
	record := model.UserAnnouncementRead{UserID: user.ID, AnnouncementID: announcementID}
	if err := h.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "success", nil)
}

func (h *Handler) AdminListAnnouncements(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(firstNonEmpty(c.Query("page_size"), c.Query("pageSize")), 20)
	if pageSize > 50 {
		pageSize = 50
	}
	query, err := h.adminAnnouncementQuery(c.Query("status"), c.Query("keyword"))
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	var rows []model.Announcement
	if err := query.Order("gmt_create DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	items := make([]announcementDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, toAnnouncementDTO(row))
	}
	ok(c, "success", gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminCreateAnnouncement(c *gin.Context) {
	var req announcementInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	input, err := normalizeAnnouncementInput(req, time.Now())
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	announcement := model.Announcement{
		Title:     input.Title,
		Content:   input.Content,
		ImageURL:  input.ImageURL,
		Status:    input.Status,
		StartTime: input.StartTime,
		EndTime:   input.EndTime,
	}
	if err := h.db.Create(&announcement).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	ok(c, "success", toAnnouncementDTO(announcement))
}

func (h *Handler) AdminGetAnnouncement(c *gin.Context) {
	announcement, okFind := h.findAnnouncement(c)
	if !okFind {
		return
	}
	ok(c, "success", toAnnouncementDTO(announcement))
}

func (h *Handler) AdminUpdateAnnouncement(c *gin.Context) {
	announcement, okFind := h.findAnnouncement(c)
	if !okFind {
		return
	}
	var req announcementInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid request")
		return
	}
	input, err := normalizeAnnouncementInput(req, announcement.StartTime)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeParamInvalid, err.Error())
		return
	}
	updates := map[string]interface{}{
		"title":      input.Title,
		"content":    input.Content,
		"image_url":  input.ImageURL,
		"status":     input.Status,
		"start_time": input.StartTime,
		"end_time":   input.EndTime,
	}
	if err := h.db.Model(&announcement).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	h.db.First(&announcement, announcement.ID)
	ok(c, "success", toAnnouncementDTO(announcement))
}

func (h *Handler) AdminSetAnnouncementEnabled(c *gin.Context) {
	h.updateAnnouncementStatus(c, model.AnnouncementStatusEnabled)
}

func (h *Handler) AdminSetAnnouncementDisabled(c *gin.Context) {
	h.updateAnnouncementStatus(c, model.AnnouncementStatusDisabled)
}

func (h *Handler) AdminDeleteAnnouncement(c *gin.Context) {
	announcementID, okID := pathUint64(c, "announcementId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid announcement id")
		return
	}
	result := h.db.Delete(&model.Announcement{}, announcementID)
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "delete failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "announcement not found")
		return
	}
	ok(c, "success", nil)
}

func (h *Handler) updateAnnouncementStatus(c *gin.Context, status uint8) {
	announcementID, okID := pathUint64(c, "announcementId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid announcement id")
		return
	}
	result := h.db.Model(&model.Announcement{}).Where("id = ?", announcementID).Update("status", status)
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "save failed")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, CodeParamInvalid, "announcement not found")
		return
	}
	ok(c, "success", nil)
}

func (h *Handler) findAnnouncement(c *gin.Context) (model.Announcement, bool) {
	announcementID, okID := pathUint64(c, "announcementId")
	if !okID {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "invalid announcement id")
		return model.Announcement{}, false
	}
	var announcement model.Announcement
	if err := h.db.First(&announcement, announcementID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, CodeParamInvalid, "announcement not found")
			return model.Announcement{}, false
		}
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return model.Announcement{}, false
	}
	return announcement, true
}

func (h *Handler) adminAnnouncementQuery(statusText, keyword string) (*gorm.DB, error) {
	query := h.db.Model(&model.Announcement{})
	statusText = strings.TrimSpace(statusText)
	if statusText != "" {
		value, err := strconv.ParseUint(statusText, 10, 8)
		if err != nil {
			return nil, errAnnouncementStatusInvalid
		}
		status, err := normalizeAnnouncementStatus(uint8(value))
		if err != nil {
			return nil, err
		}
		query = query.Where("status = ?", status)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}
	return query, nil
}

func normalizeAnnouncementInput(req announcementInput, defaultStart time.Time) (normalizedAnnouncement, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" || len([]rune(title)) > 80 {
		return normalizedAnnouncement{}, errAnnouncementTitleInvalid
	}
	content := strings.TrimSpace(req.Content)
	if content == "" || len([]rune(content)) > 1000 {
		return normalizedAnnouncement{}, errAnnouncementContentInvalid
	}
	imageURL := strings.TrimSpace(req.ImageURL)
	if len([]rune(imageURL)) > 255 {
		return normalizedAnnouncement{}, errAnnouncementImageInvalid
	}
	status, err := normalizeAnnouncementStatus(req.Status)
	if err != nil {
		return normalizedAnnouncement{}, err
	}
	startTime, err := parseOptionalDateTime(req.StartTime)
	if err != nil {
		return normalizedAnnouncement{}, errAnnouncementTimeInvalid
	}
	start := defaultStart
	if startTime != nil {
		start = *startTime
	}
	endTime, err := parseOptionalDateTime(req.EndTime)
	if err != nil {
		return normalizedAnnouncement{}, errAnnouncementTimeInvalid
	}
	if endTime != nil && endTime.Before(start) {
		return normalizedAnnouncement{}, errAnnouncementTimeInvalid
	}
	var imagePtr *string
	if imageURL != "" {
		imagePtr = &imageURL
	}
	return normalizedAnnouncement{Title: title, Content: content, ImageURL: imagePtr, Status: status, StartTime: start, EndTime: endTime}, nil
}

func normalizeAnnouncementStatus(status uint8) (uint8, error) {
	switch status {
	case 0, model.AnnouncementStatusEnabled:
		return model.AnnouncementStatusEnabled, nil
	case model.AnnouncementStatusDisabled:
		return model.AnnouncementStatusDisabled, nil
	default:
		return 0, errAnnouncementStatusInvalid
	}
}

type announcementDTO struct {
	ID          uint64 `json:"id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	ImageURL    string `json:"image_url"`
	Status      uint8  `json:"status"`
	StatusLabel string `json:"status_label"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toAnnouncementDTO(announcement model.Announcement) announcementDTO {
	return announcementDTO{
		ID:          announcement.ID,
		Title:       announcement.Title,
		Content:     announcement.Content,
		ImageURL:    stringValue(announcement.ImageURL),
		Status:      announcement.Status,
		StatusLabel: announcementStatusLabel(announcement.Status),
		StartTime:   announcement.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:     timeValue(announcement.EndTime),
		CreatedAt:   announcement.GmtCreate.Format("2006-01-02 15:04:05"),
		UpdatedAt:   announcement.GmtModified.Format("2006-01-02 15:04:05"),
	}
}

func announcementStatusLabel(status uint8) string {
	if status == model.AnnouncementStatusDisabled {
		return "停用"
	}
	return "启用"
}

func timeValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}
