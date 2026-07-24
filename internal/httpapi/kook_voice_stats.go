package httpapi

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type kookVoiceSessionRow struct {
	ID              uint64
	GuildID         string
	ChannelID       string
	KookUserID      string
	Username        *string
	Nickname        *string
	JoinedAt        time.Time
	ExitedAt        *time.Time
	DurationSeconds uint
	Status          string
	Source          string
}

type kookVoiceUsageDTO struct {
	GuildID                 string  `json:"guildId"`
	ChannelID               string  `json:"channelId"`
	ChannelName             string  `json:"channelName,omitempty"`
	KookUserID              string  `json:"kookUserId"`
	Username                string  `json:"username,omitempty"`
	Nickname                string  `json:"nickname,omitempty"`
	Date                    string  `json:"date,omitempty"`
	DurationSeconds         int64   `json:"durationSeconds"`
	DurationText            string  `json:"durationText"`
	OccupiedDurationSeconds int64   `json:"occupiedDurationSeconds,omitempty"`
	OccupiedDurationText    string  `json:"occupiedDurationText,omitempty"`
	IdleDurationSeconds     int64   `json:"idleDurationSeconds,omitempty"`
	IdleDurationText        string  `json:"idleDurationText,omitempty"`
	SessionCount            int     `json:"sessionCount"`
	LastJoinedAt            *string `json:"lastJoinedAt,omitempty"`
}

type kookVoiceSessionDTO struct {
	ID              uint64  `json:"id"`
	GuildID         string  `json:"guildId"`
	ChannelID       string  `json:"channelId"`
	ChannelName     string  `json:"channelName,omitempty"`
	KookUserID      string  `json:"kookUserId"`
	Username        string  `json:"username,omitempty"`
	Nickname        string  `json:"nickname,omitempty"`
	JoinedAt        string  `json:"joinedAt"`
	ExitedAt        *string `json:"exitedAt"`
	DurationSeconds int64   `json:"durationSeconds"`
	DurationText    string  `json:"durationText"`
	Status          string  `json:"status"`
	Source          string  `json:"source"`
}

type kookVoiceChannelUsageDTO struct {
	ChannelID               string `json:"channelId"`
	DurationSeconds         int64  `json:"durationSeconds"`
	DurationText            string `json:"durationText"`
	OccupiedDurationSeconds int64  `json:"occupiedDurationSeconds"`
	OccupiedDurationText    string `json:"occupiedDurationText"`
	IdleDurationSeconds     int64  `json:"idleDurationSeconds"`
	IdleDurationText        string `json:"idleDurationText"`
	SessionCount            int64  `json:"sessionCount"`
	ActiveUserCount         int64  `json:"activeUserCount"`
}

func (h *Handler) AdminKookVoiceStats(c *gin.Context) {
	start, end, okRange := kookVoiceStatsRange(c)
	if !okRange {
		return
	}
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(firstNonEmpty(c.Query("page_size"), c.Query("pageSize")), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	channelID := strings.TrimSpace(c.Query("channelId"))
	userID := strings.TrimSpace(c.Query("userId"))

	channelNames := h.kookVoiceChannelNames()
	userStats, err := h.kookVoiceUserStats(start, end, channelID, userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	channelStats, err := h.kookVoiceChannelStats(start, end, channelID, userID, channelNames)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	dailyRanking, err := h.kookVoiceDailyRanking(start, end, channelID, userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	total, pageRows, err := h.kookVoiceSessionPage(start, end, channelID, userID, page, pageSize)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}

	ok(c, "success", gin.H{
		"range": gin.H{
			"startTime": start.Format("2006-01-02 15:04:05"),
			"endTime":   end.Format("2006-01-02 15:04:05"),
		},
		"userStats":    userStats,
		"channelStats": channelStats,
		"dailyRanking": dailyRanking,
		"sessions":     kookVoiceSessionDTOs(pageRows, start, end, channelNames),
		"total":        total,
		"page":         page,
		"pageSize":     pageSize,
	})
}

func (h *Handler) AdminKookVoiceChannelUsageSummary(c *gin.Context) {
	start, end, okRange := kookVoiceStatsRange(c)
	if !okRange {
		return
	}
	list, err := h.kookVoiceChannelUsageSummary(start, end)
	if err != nil {
		fail(c, http.StatusInternalServerError, CodeSystemError, "query failed")
		return
	}
	ok(c, "success", gin.H{
		"range": gin.H{
			"startTime": start.Format("2006-01-02 15:04:05"),
			"endTime":   end.Format("2006-01-02 15:04:05"),
		},
		"list": list,
	})
}

func kookVoiceStatsRange(c *gin.Context) (time.Time, time.Time, bool) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	end := now
	if raw := strings.TrimSpace(c.Query("startTime")); raw != "" {
		parsed, err := parseOptionalDateTime(raw)
		if err != nil || parsed == nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, "startTime invalid")
			return time.Time{}, time.Time{}, false
		}
		start = *parsed
	}
	if raw := strings.TrimSpace(c.Query("endTime")); raw != "" {
		parsed, err := parseOptionalDateTime(raw)
		if err != nil || parsed == nil {
			fail(c, http.StatusBadRequest, CodeParamInvalid, "endTime invalid")
			return time.Time{}, time.Time{}, false
		}
		end = *parsed
	}
	if !end.After(start) {
		fail(c, http.StatusBadRequest, CodeParamInvalid, "endTime must be after startTime")
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func (h *Handler) kookVoiceSessions(start, end time.Time, channelID, userID string, limit int) ([]kookVoiceSessionRow, error) {
	rows := []kookVoiceSessionRow{}
	query := h.db.Table("ttw_kook_voice_session s").
		Select("s.id, s.guild_id, s.channel_id, s.kook_user_id, m.username, m.nickname, s.joined_at, s.exited_at, s.duration_seconds, s.status, s.source").
		Joins("LEFT JOIN ttw_kook_member m ON m.guild_id = s.guild_id AND m.kook_user_id = s.kook_user_id").
		Where("s.joined_at < ? AND COALESCE(s.exited_at, NOW()) > ?", end, start)
	if channelID != "" {
		query = query.Where("s.channel_id = ?", channelID)
	}
	if userID != "" {
		query = query.Where("s.kook_user_id = ?", userID)
	}
	return rows, query.Order("s.joined_at DESC").Limit(limit).Scan(&rows).Error
}

func (h *Handler) kookVoiceSessionPage(start, end time.Time, channelID, userID string, page, pageSize int) (int64, []kookVoiceSessionRow, error) {
	base := h.db.Table("ttw_kook_voice_session s").Where("s.joined_at < ? AND (s.exited_at IS NULL OR s.exited_at > ?)", end, start)
	if channelID != "" {
		base = base.Where("s.channel_id = ?", channelID)
	}
	if userID != "" {
		base = base.Where("s.kook_user_id = ?", userID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	rows := []kookVoiceSessionRow{}
	err := base.Select("s.id, s.guild_id, s.channel_id, s.kook_user_id, m.username, m.nickname, s.joined_at, s.exited_at, s.duration_seconds, s.status, s.source").
		Joins("LEFT JOIN ttw_kook_member m ON m.guild_id = s.guild_id AND m.kook_user_id = s.kook_user_id").
		Order("s.joined_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error
	return total, rows, err
}

func (h *Handler) kookVoiceUserStats(start, end time.Time, channelID, userID string) ([]kookVoiceUsageDTO, error) {
	rows := []struct {
		GuildID         string
		KookUserID      string
		Username        *string
		Nickname        *string
		DurationSeconds int64
		SessionCount    int
		LastJoinedAt    *time.Time
	}{}
	query := h.db.Table("ttw_kook_voice_session s").
		Select("s.guild_id, s.kook_user_id, MAX(m.username) AS username, MAX(m.nickname) AS nickname, SUM(GREATEST(0, TIMESTAMPDIFF(SECOND, GREATEST(s.joined_at, ?), LEAST(IFNULL(s.exited_at, ?), ?)))) AS duration_seconds, COUNT(*) AS session_count, MAX(s.joined_at) AS last_joined_at", start, minTime(time.Now(), end), end).
		Joins("LEFT JOIN ttw_kook_member m ON m.guild_id = s.guild_id AND m.kook_user_id = s.kook_user_id").
		Where("s.joined_at < ? AND (s.exited_at IS NULL OR s.exited_at > ?)", end, start)
	if channelID != "" {
		query = query.Where("s.channel_id = ?", channelID)
	}
	if userID != "" {
		query = query.Where("s.kook_user_id = ?", userID)
	}
	if err := query.Group("s.guild_id, s.kook_user_id").Order("duration_seconds DESC, session_count DESC").Limit(100).Scan(&rows).Error; err != nil {
		return nil, err
	}
	list := make([]kookVoiceUsageDTO, 0, len(rows))
	for _, row := range rows {
		var lastJoinedAt *string
		if row.LastJoinedAt != nil {
			text := row.LastJoinedAt.Format("2006-01-02 15:04:05")
			lastJoinedAt = &text
		}
		list = append(list, kookVoiceUsageDTO{
			GuildID:         row.GuildID,
			KookUserID:      row.KookUserID,
			Username:        stringValue(row.Username),
			Nickname:        stringValue(row.Nickname),
			DurationSeconds: row.DurationSeconds,
			DurationText:    durationText(row.DurationSeconds),
			SessionCount:    row.SessionCount,
			LastJoinedAt:    lastJoinedAt,
		})
	}
	return list, nil
}

func (h *Handler) kookVoiceChannelStats(start, end time.Time, channelID, userID string, channelNames map[string]string) ([]kookVoiceUsageDTO, error) {
	rows := []struct {
		GuildID         string
		ChannelID       string
		DurationSeconds int64
		SessionCount    int
	}{}
	query := h.db.Table("ttw_kook_voice_session s").
		Select("s.guild_id, s.channel_id, SUM(GREATEST(0, TIMESTAMPDIFF(SECOND, GREATEST(s.joined_at, ?), LEAST(IFNULL(s.exited_at, ?), ?)))) AS duration_seconds, COUNT(*) AS session_count", start, minTime(time.Now(), end), end).
		Where("s.joined_at < ? AND (s.exited_at IS NULL OR s.exited_at > ?)", end, start)
	if channelID != "" {
		query = query.Where("s.channel_id = ?", channelID)
	}
	if userID != "" {
		query = query.Where("s.kook_user_id = ?", userID)
	}
	if err := query.Group("s.guild_id, s.channel_id").Order("duration_seconds DESC, session_count DESC").Limit(100).Scan(&rows).Error; err != nil {
		return nil, err
	}
	list := make([]kookVoiceUsageDTO, 0, len(rows))
	for _, row := range rows {
		list = append(list, kookVoiceUsageDTO{
			GuildID:         row.GuildID,
			ChannelID:       row.ChannelID,
			ChannelName:     channelNames[row.ChannelID],
			DurationSeconds: row.DurationSeconds,
			DurationText:    durationText(row.DurationSeconds),
			SessionCount:    row.SessionCount,
		})
	}
	effectiveEnd := minTime(time.Now(), end)
	if effectiveEnd.After(start) && len(list) > 0 {
		channelIDs := make([]string, 0, len(list))
		seen := make(map[string]struct{}, len(list))
		for _, item := range list {
			if _, ok := seen[item.ChannelID]; ok {
				continue
			}
			seen[item.ChannelID] = struct{}{}
			channelIDs = append(channelIDs, item.ChannelID)
		}
		intervals := []kookVoiceInterval{}
		intervalQuery := h.db.Table("ttw_kook_voice_session").
			Select("guild_id, channel_id, joined_at, exited_at").
			Where("channel_id IN ? AND joined_at < ? AND (exited_at IS NULL OR exited_at > ?)", channelIDs, effectiveEnd, start)
		if err := intervalQuery.Scan(&intervals).Error; err != nil {
			return nil, err
		}
		attachKookVoiceUsageOccupancy(list, mergeKookVoiceIntervals(intervals, start, effectiveEnd), int64(effectiveEnd.Sub(start).Seconds()))
	}
	return list, nil
}

func (h *Handler) kookVoiceDailyRanking(start, end time.Time, channelID, userID string) ([]kookVoiceUsageDTO, error) {
	effectiveEnd := minTime(time.Now(), end)
	if !effectiveEnd.After(start) {
		return []kookVoiceUsageDTO{}, nil
	}
	sql := `WITH RECURSIVE days(day_start) AS (
  SELECT TIMESTAMP(DATE(?))
  UNION ALL
  SELECT DATE_ADD(day_start, INTERVAL 1 DAY) FROM days WHERE DATE_ADD(day_start, INTERVAL 1 DAY) < ?
)
SELECT DATE_FORMAT(days.day_start, '%Y-%m-%d') AS date,
       s.guild_id,
       s.kook_user_id,
       MAX(m.username) AS username,
       MAX(m.nickname) AS nickname,
       SUM(GREATEST(0, TIMESTAMPDIFF(SECOND, GREATEST(s.joined_at, ?, days.day_start), LEAST(IFNULL(s.exited_at, ?), ?, DATE_ADD(days.day_start, INTERVAL 1 DAY))))) AS duration_seconds,
       COUNT(*) AS session_count
FROM days
JOIN ttw_kook_voice_session s ON s.joined_at < LEAST(DATE_ADD(days.day_start, INTERVAL 1 DAY), ?) AND (s.exited_at IS NULL OR s.exited_at > days.day_start)
LEFT JOIN ttw_kook_member m ON m.guild_id = s.guild_id AND m.kook_user_id = s.kook_user_id
WHERE s.joined_at < ? AND (s.exited_at IS NULL OR s.exited_at > ?)`
	args := []any{start, effectiveEnd, start, effectiveEnd, effectiveEnd, effectiveEnd, end, start}
	if channelID != "" {
		sql += " AND s.channel_id = ?"
		args = append(args, channelID)
	}
	if userID != "" {
		sql += " AND s.kook_user_id = ?"
		args = append(args, userID)
	}
	sql += " GROUP BY date, s.guild_id, s.kook_user_id ORDER BY duration_seconds DESC, session_count DESC LIMIT 100"

	rows := []struct {
		Date            string
		GuildID         string
		KookUserID      string
		Username        *string
		Nickname        *string
		DurationSeconds int64
		SessionCount    int
	}{}
	if err := h.db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	list := make([]kookVoiceUsageDTO, 0, len(rows))
	for _, row := range rows {
		list = append(list, kookVoiceUsageDTO{
			Date:            row.Date,
			GuildID:         row.GuildID,
			KookUserID:      row.KookUserID,
			Username:        stringValue(row.Username),
			Nickname:        stringValue(row.Nickname),
			DurationSeconds: row.DurationSeconds,
			DurationText:    durationText(row.DurationSeconds),
			SessionCount:    row.SessionCount,
		})
	}
	return list, nil
}

func (h *Handler) kookVoiceChannelUsageSummary(start, end time.Time) ([]kookVoiceChannelUsageDTO, error) {
	type usageRow struct {
		ChannelID       string
		DurationSeconds int64
		SessionCount    int64
	}
	usageRows := []usageRow{}
	if err := h.db.Table("ttw_kook_voice_session").
		Select("channel_id, SUM(GREATEST(0, TIMESTAMPDIFF(SECOND, GREATEST(joined_at, ?), LEAST(COALESCE(exited_at, NOW()), ?)))) AS duration_seconds, COUNT(*) AS session_count", start, end).
		Where("joined_at < ? AND (exited_at IS NULL OR exited_at > ?)", end, start).
		Group("channel_id").
		Scan(&usageRows).Error; err != nil {
		return nil, err
	}

	type activeRow struct {
		ChannelID       string
		ActiveUserCount int64
	}
	activeRows := []activeRow{}
	if err := h.db.Table("ttw_kook_voice_session").
		Select("channel_id, COUNT(DISTINCT kook_user_id) AS active_user_count").
		Where("exited_at IS NULL").
		Group("channel_id").
		Scan(&activeRows).Error; err != nil {
		return nil, err
	}

	activeByChannel := make(map[string]int64, len(activeRows))
	for _, row := range activeRows {
		activeByChannel[row.ChannelID] = row.ActiveUserCount
	}
	list := make([]kookVoiceChannelUsageDTO, 0, len(usageRows)+len(activeRows))
	seen := make(map[string]struct{}, len(usageRows)+len(activeRows))
	for _, row := range usageRows {
		seen[row.ChannelID] = struct{}{}
		list = append(list, kookVoiceChannelUsageDTO{
			ChannelID:       row.ChannelID,
			DurationSeconds: row.DurationSeconds,
			DurationText:    durationText(row.DurationSeconds),
			SessionCount:    row.SessionCount,
			ActiveUserCount: activeByChannel[row.ChannelID],
		})
	}
	for _, row := range activeRows {
		if _, ok := seen[row.ChannelID]; ok {
			continue
		}
		list = append(list, kookVoiceChannelUsageDTO{
			ChannelID:       row.ChannelID,
			DurationText:    "0秒",
			ActiveUserCount: row.ActiveUserCount,
		})
	}
	guildID := h.kookGuildID()
	effectiveEnd := minTime(time.Now(), end)
	intervals := []kookVoiceInterval{}
	if guildID != "" && effectiveEnd.After(start) {
		if err := h.db.Table("ttw_kook_voice_session").
			Select("guild_id, channel_id, joined_at, exited_at").
			Where("guild_id = ? AND joined_at < ? AND (exited_at IS NULL OR exited_at > ?)", guildID, effectiveEnd, start).
			Scan(&intervals).Error; err != nil {
			return nil, err
		}
	}
	attachKookVoiceOccupancy(list, guildID, mergeKookVoiceIntervals(intervals, start, effectiveEnd), int64(effectiveEnd.Sub(start).Seconds()))
	sort.Slice(list, func(i, j int) bool {
		if list[i].ActiveUserCount == list[j].ActiveUserCount {
			return list[i].DurationSeconds > list[j].DurationSeconds
		}
		return list[i].ActiveUserCount > list[j].ActiveUserCount
	})
	return list, nil
}

func (h *Handler) kookVoiceChannelNames() map[string]string {
	h.kookChannelNamesMu.Lock()
	result := make(map[string]string, len(h.kookChannelNames))
	for id, name := range h.kookChannelNames {
		result[id] = name
	}
	needsReload := time.Now().After(h.kookChannelNamesUntil) && !h.kookChannelNamesReload
	if needsReload {
		h.kookChannelNamesReload = true
	}
	h.kookChannelNamesMu.Unlock()
	if needsReload {
		go h.reloadKookVoiceChannelNames()
	}
	return result
}

func (h *Handler) reloadKookVoiceChannelNames() {
	result := map[string]string{}
	channels, _, err := h.fetchKookChannels()
	if err == nil {
		for _, channel := range channels {
			result[channel.ID] = channel.Name
		}
	}
	h.kookChannelNamesMu.Lock()
	defer h.kookChannelNamesMu.Unlock()
	h.kookChannelNamesReload = false
	if err != nil {
		return
	}
	h.kookChannelNames = result
	h.kookChannelNamesUntil = time.Now().Add(10 * time.Minute)
}

func kookVoiceUserStatsFromRows(rows []kookVoiceSessionRow, start, end time.Time, channelNames map[string]string) []kookVoiceUsageDTO {
	stats := map[string]*kookVoiceUsageDTO{}
	for _, row := range rows {
		duration := kookVoiceOverlapSeconds(row, start, end)
		if duration <= 0 {
			continue
		}
		item := stats[row.GuildID+"|"+row.KookUserID]
		if item == nil {
			item = &kookVoiceUsageDTO{
				GuildID:    row.GuildID,
				KookUserID: row.KookUserID,
				Username:   stringValue(row.Username),
				Nickname:   stringValue(row.Nickname),
			}
			stats[row.GuildID+"|"+row.KookUserID] = item
		}
		item.DurationSeconds += duration
		item.SessionCount++
		item.DurationText = durationText(item.DurationSeconds)
		if item.LastJoinedAt == nil || row.JoinedAt.After(parseDTOTime(*item.LastJoinedAt)) {
			text := row.JoinedAt.Format("2006-01-02 15:04:05")
			item.LastJoinedAt = &text
		}
		_ = channelNames
	}
	return sortedUsage(stats)
}

func kookVoiceChannelStatsFromRows(rows []kookVoiceSessionRow, start, end time.Time, channelNames map[string]string) []kookVoiceUsageDTO {
	stats := map[string]*kookVoiceUsageDTO{}
	for _, row := range rows {
		duration := kookVoiceOverlapSeconds(row, start, end)
		if duration <= 0 {
			continue
		}
		item := stats[row.GuildID+"|"+row.ChannelID]
		if item == nil {
			item = &kookVoiceUsageDTO{
				GuildID:     row.GuildID,
				ChannelID:   row.ChannelID,
				ChannelName: channelNames[row.ChannelID],
			}
			stats[row.GuildID+"|"+row.ChannelID] = item
		}
		item.DurationSeconds += duration
		item.SessionCount++
		item.DurationText = durationText(item.DurationSeconds)
	}
	return sortedUsage(stats)
}

func kookVoiceDailyRankingFromRows(rows []kookVoiceSessionRow, start, end time.Time, channelNames map[string]string) []kookVoiceUsageDTO {
	stats := map[string]*kookVoiceUsageDTO{}
	for _, row := range rows {
		for dayStart := maxTime(row.JoinedAt, start); dayStart.Before(minTime(kookVoiceExitTime(row), end)); {
			nextDay := time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day()+1, 0, 0, 0, 0, time.Local)
			dayEnd := minTime(nextDay, minTime(kookVoiceExitTime(row), end))
			duration := int64(dayEnd.Sub(dayStart).Seconds())
			if duration > 0 {
				date := dayStart.Format("2006-01-02")
				key := date + "|" + row.GuildID + "|" + row.KookUserID
				item := stats[key]
				if item == nil {
					item = &kookVoiceUsageDTO{
						Date:       date,
						GuildID:    row.GuildID,
						KookUserID: row.KookUserID,
						Username:   stringValue(row.Username),
						Nickname:   stringValue(row.Nickname),
					}
					stats[key] = item
				}
				item.DurationSeconds += duration
				item.SessionCount++
				item.DurationText = durationText(item.DurationSeconds)
			}
			dayStart = dayEnd
		}
		_ = channelNames
	}
	return sortedUsage(stats)
}

func kookVoiceSessionDTOs(rows []kookVoiceSessionRow, start, end time.Time, channelNames map[string]string) []kookVoiceSessionDTO {
	list := make([]kookVoiceSessionDTO, 0, len(rows))
	for _, row := range rows {
		duration := kookVoiceOverlapSeconds(row, start, end)
		exitedAt := kookTimeString(row.ExitedAt)
		list = append(list, kookVoiceSessionDTO{
			ID:              row.ID,
			GuildID:         row.GuildID,
			ChannelID:       row.ChannelID,
			ChannelName:     channelNames[row.ChannelID],
			KookUserID:      row.KookUserID,
			Username:        stringValue(row.Username),
			Nickname:        stringValue(row.Nickname),
			JoinedAt:        row.JoinedAt.Format("2006-01-02 15:04:05"),
			ExitedAt:        exitedAt,
			DurationSeconds: duration,
			DurationText:    durationText(duration),
			Status:          row.Status,
			Source:          row.Source,
		})
	}
	return list
}

func sortedUsage(stats map[string]*kookVoiceUsageDTO) []kookVoiceUsageDTO {
	list := make([]kookVoiceUsageDTO, 0, len(stats))
	for _, item := range stats {
		list = append(list, *item)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].DurationSeconds == list[j].DurationSeconds {
			return list[i].SessionCount > list[j].SessionCount
		}
		return list[i].DurationSeconds > list[j].DurationSeconds
	})
	if len(list) > 100 {
		return list[:100]
	}
	return list
}

func kookVoiceOverlapSeconds(row kookVoiceSessionRow, start, end time.Time) int64 {
	overlapStart := maxTime(row.JoinedAt, start)
	overlapEnd := minTime(kookVoiceExitTime(row), end)
	if !overlapEnd.After(overlapStart) {
		return 0
	}
	return int64(overlapEnd.Sub(overlapStart).Seconds())
}

func kookVoiceExitTime(row kookVoiceSessionRow) time.Time {
	if row.ExitedAt != nil {
		return *row.ExitedAt
	}
	return time.Now()
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func durationText(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		return strconv.FormatInt(hours, 10) + "小时" + strconv.FormatInt(minutes, 10) + "分钟"
	}
	if minutes > 0 {
		return strconv.FormatInt(minutes, 10) + "分钟" + strconv.FormatInt(secs, 10) + "秒"
	}
	return strconv.FormatInt(secs, 10) + "秒"
}

func parseDTOTime(value string) time.Time {
	parsed, _ := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	return parsed
}
