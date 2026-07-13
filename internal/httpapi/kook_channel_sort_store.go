package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const kookChannelSortConfigID = 1

type kookChannelSortConfigDTO struct {
	Enabled      bool                      `json:"enabled"`
	GroupIDs     []string                  `json:"groupIds"`
	ScheduleType string                    `json:"scheduleType"`
	Weekday      *int                      `json:"weekday"`
	Monthday     *int                      `json:"monthday"`
	Hour         int                       `json:"hour"`
	NextRunAt    *time.Time                `json:"nextRunAt"`
	LatestRun    *model.KookChannelSortRun `json:"latestRun,omitempty"`
}

func validateKookChannelSortConfig(config kookChannelSortConfigDTO) error {
	if _, err := normalizeKookChannelSortGroupIDs(config.GroupIDs); err != nil {
		return err
	}
	if !config.Enabled && config.ScheduleType == "" {
		return nil
	}
	if config.Enabled && len(config.GroupIDs) == 0 {
		return fmt.Errorf("at least one KOOK group must be selected")
	}
	schedule := kookChannelSortSchedule{
		ScheduleType: config.ScheduleType,
		Hour:         config.Hour,
	}
	if config.Weekday != nil {
		schedule.Weekday = *config.Weekday
	}
	if config.Monthday != nil {
		schedule.Monthday = *config.Monthday
	}
	return validateKookChannelSortSchedule(schedule)
}

func normalizeKookChannelSortGroupIDs(groupIDs []string) ([]string, error) {
	normalized := make([]string, 0, len(groupIDs))
	seen := make(map[string]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			return nil, fmt.Errorf("KOOK group ID cannot be empty")
		}
		if _, exists := seen[groupID]; exists {
			return nil, fmt.Errorf("KOOK group %q is selected more than once", groupID)
		}
		seen[groupID] = struct{}{}
		normalized = append(normalized, groupID)
	}
	return normalized, nil
}

func prepareKookChannelSortConfig(config kookChannelSortConfigDTO, now time.Time, location *time.Location) (model.KookChannelSortConfig, error) {
	if !config.Enabled && config.ScheduleType == "" {
		config.ScheduleType = kookChannelSortScheduleDaily
	}
	if err := validateKookChannelSortConfig(config); err != nil {
		return model.KookChannelSortConfig{}, err
	}
	groupIDs, err := normalizeKookChannelSortGroupIDs(config.GroupIDs)
	if err != nil {
		return model.KookChannelSortConfig{}, err
	}
	encoded, err := json.Marshal(groupIDs)
	if err != nil {
		return model.KookChannelSortConfig{}, fmt.Errorf("encode KOOK group IDs: %w", err)
	}
	row := model.KookChannelSortConfig{
		ID:           kookChannelSortConfigID,
		Enabled:      config.Enabled,
		GroupIDs:     string(encoded),
		ScheduleType: config.ScheduleType,
		Weekday:      config.Weekday,
		Monthday:     config.Monthday,
		Hour:         config.Hour,
	}
	if config.Enabled {
		nextRunAt, err := nextKookChannelSortRun(now, kookChannelSortSchedule{
			ScheduleType: config.ScheduleType,
			Weekday:      intValue(config.Weekday),
			Monthday:     intValue(config.Monthday),
			Hour:         config.Hour,
		}, location)
		if err != nil {
			return model.KookChannelSortConfig{}, err
		}
		row.NextRunAt = &nextRunAt
	}
	return row, nil
}

func kookChannelSortConfigFromModel(row model.KookChannelSortConfig) (kookChannelSortConfigDTO, error) {
	groupIDs := []string{}
	if err := json.Unmarshal([]byte(row.GroupIDs), &groupIDs); err != nil {
		return kookChannelSortConfigDTO{}, fmt.Errorf("decode KOOK group IDs: %w", err)
	}
	return kookChannelSortConfigDTO{
		Enabled:      row.Enabled,
		GroupIDs:     groupIDs,
		ScheduleType: row.ScheduleType,
		Weekday:      row.Weekday,
		Monthday:     row.Monthday,
		Hour:         row.Hour,
		NextRunAt:    row.NextRunAt,
	}, nil
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func (h *Handler) loadKookChannelSortConfig() (model.KookChannelSortConfig, error) {
	var row model.KookChannelSortConfig
	err := h.db.Where("id = ?", kookChannelSortConfigID).First(&row).Error
	return row, err
}

func (h *Handler) getKookChannelSortConfig() (kookChannelSortConfigDTO, error) {
	row, err := h.loadKookChannelSortConfig()
	if err != nil {
		return kookChannelSortConfigDTO{}, err
	}
	dto, err := kookChannelSortConfigFromModel(row)
	if err != nil {
		return kookChannelSortConfigDTO{}, err
	}
	var latest model.KookChannelSortRun
	if err := h.db.Order("id DESC").First(&latest).Error; err == nil {
		dto.LatestRun = &latest
	} else if err != gorm.ErrRecordNotFound {
		return kookChannelSortConfigDTO{}, err
	}
	return dto, nil
}

func (h *Handler) saveKookChannelSortConfig(config kookChannelSortConfigDTO, now time.Time, location *time.Location) (kookChannelSortConfigDTO, error) {
	row, err := prepareKookChannelSortConfig(config, now, location)
	if err != nil {
		return kookChannelSortConfigDTO{}, err
	}
	updates := map[string]interface{}{
		"enabled":       row.Enabled,
		"group_ids":     row.GroupIDs,
		"schedule_type": row.ScheduleType,
		"weekday":       row.Weekday,
		"monthday":      row.Monthday,
		"hour":          row.Hour,
		"next_run_at":   row.NextRunAt,
		"gmt_modified":  gorm.Expr("NOW()"),
	}
	if err := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&row).Error; err != nil {
		return kookChannelSortConfigDTO{}, err
	}
	return h.getKookChannelSortConfig()
}

func (h *Handler) listKookChannelSortRuns(page, pageSize int) ([]model.KookChannelSortRun, int64, error) {
	query := h.db.Model(&model.KookChannelSortRun{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.KookChannelSortRun
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func kookChannelSortLeaseAcquireUpdate(db *gorm.DB, owner string, ttl time.Duration) *gorm.DB {
	return db.Model(&model.KookChannelSortConfig{}).
		Where("id = ? AND (locked_until IS NULL OR locked_until < NOW())", kookChannelSortConfigID).
		Updates(map[string]interface{}{
			"lock_token":   owner,
			"locked_until": gorm.Expr("DATE_ADD(NOW(), INTERVAL ? MICROSECOND)", ttl.Microseconds()),
		})
}

func (h *Handler) acquireKookChannelSortLease(owner string, ttl time.Duration) (bool, error) {
	if strings.TrimSpace(owner) == "" || ttl <= 0 {
		return false, fmt.Errorf("KOOK channel sort lease owner and TTL are required")
	}
	result := kookChannelSortLeaseAcquireUpdate(h.db, owner, ttl)
	return result.RowsAffected == 1, result.Error
}

func (h *Handler) renewKookChannelSortLease(owner string, ttl time.Duration) (bool, error) {
	if strings.TrimSpace(owner) == "" || ttl <= 0 {
		return false, fmt.Errorf("KOOK channel sort lease owner and TTL are required")
	}
	result := h.db.Model(&model.KookChannelSortConfig{}).
		Where("id = ? AND lock_token = ? AND locked_until >= NOW()", kookChannelSortConfigID, owner).
		Updates(map[string]interface{}{
			"locked_until": gorm.Expr("DATE_ADD(NOW(), INTERVAL ? MICROSECOND)", ttl.Microseconds()),
		})
	return result.RowsAffected == 1, result.Error
}

func (h *Handler) releaseKookChannelSortLease(owner string) error {
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("KOOK channel sort lease owner is required")
	}
	return h.db.Model(&model.KookChannelSortConfig{}).
		Where("id = ? AND lock_token = ?", kookChannelSortConfigID, owner).
		Updates(map[string]interface{}{"lock_token": nil, "locked_until": nil}).Error
}
