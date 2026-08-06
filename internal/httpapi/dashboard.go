package httpapi

import (
	"sort"
	"time"

	"steam-game-takeover-backend/internal/model"
)

const dashboardCacheTTL = 30 * time.Second

type dashboardSnapshot struct {
	Summary         dashboardSummary     `json:"summary"`
	RecentTakeovers []dashboardTakeover  `json:"recentTakeovers"`
	KookMemberTotal int64                `json:"kookMemberTotal"`
	KookUsage       []dashboardKookUsage `json:"kookUsage"`
	VoiceStats      dashboardVoiceStats  `json:"voiceStats"`
}

type dashboardSummary struct {
	TakeoverTotal        int64 `json:"takeoverTotal"`
	UserTotal            int64 `json:"userTotal"`
	PendingReportTotal   int64 `json:"pendingReportTotal"`
	PendingFeedbackTotal int64 `json:"pendingFeedbackTotal"`
	AdminUserTotal       int64 `json:"adminUserTotal"`
}

type dashboardTakeover struct {
	ID               uint64 `json:"id"`
	Title            string `json:"title"`
	StatusLabel      string `json:"statusLabel"`
	JoinedCount      int64  `json:"joinedCount"`
	ParticipantLimit uint   `json:"participantLimit"`
	ScheduleText     string `json:"scheduleText"`
	CreatorName      string `json:"creatorName"`
	KookChannelName  string `json:"kookChannelName"`
	CreatedAt        string `json:"createdAt"`
}

type dashboardKookUsage struct {
	ChannelID       string `json:"channelId"`
	ChannelName     string `json:"channelName"`
	DurationSeconds int64  `json:"durationSeconds"`
	DurationText    string `json:"durationText"`
	SessionCount    int64  `json:"sessionCount"`
	ActiveUserCount int64  `json:"activeUserCount"`
}

type dashboardVoiceStats struct {
	Range              dashboardRange       `json:"range"`
	UserStats          []dashboardVoiceUser `json:"userStats"`
	TotalDuration      int64                `json:"totalDurationSeconds"`
	ActiveUserTotal    int64                `json:"activeUserTotal"`
	ActiveChannelTotal int64                `json:"activeChannelTotal"`
}

type dashboardVoiceUser struct {
	KookUserID      string `json:"kookUserId"`
	Username        string `json:"username"`
	Nickname        string `json:"nickname"`
	DurationSeconds int64  `json:"durationSeconds"`
	DurationText    string `json:"durationText"`
	SessionCount    int    `json:"sessionCount"`
}

type dashboardRange struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type dashboardTakeoverRow struct {
	model.Takeover `gorm:"embedded"`
	JoinedCount    int64   `gorm:"column:joined_count"`
	CreatorName    *string `gorm:"column:creator_name"`
}

func (h *Handler) dashboardSnapshot() (dashboardSnapshot, bool, error) {
	// ponytail: one shared lock coalesces dashboard refreshes; use singleflight if traffic grows.
	h.dashboardMu.Lock()
	defer h.dashboardMu.Unlock()

	if time.Now().Before(h.dashboardCacheUntil) {
		return h.dashboardCache, true, nil
	}

	snapshot, err := h.loadDashboardSnapshot(time.Now())
	if err != nil {
		return dashboardSnapshot{}, false, err
	}
	h.dashboardCache = snapshot
	h.dashboardCacheUntil = time.Now().Add(dashboardCacheTTL)
	return snapshot, false, nil
}

func (h *Handler) loadDashboardSnapshot(now time.Time) (dashboardSnapshot, error) {
	var summary dashboardSummary
	if err := h.db.Model(&model.Takeover{}).Where("is_deleted = ?", false).Count(&summary.TakeoverTotal).Error; err != nil {
		return dashboardSnapshot{}, err
	}
	if err := h.db.Model(&model.User{}).Where("is_deleted = ?", false).Count(&summary.UserTotal).Error; err != nil {
		return dashboardSnapshot{}, err
	}
	if err := h.adminReportBaseQuery("pending").Count(&summary.PendingReportTotal).Error; err != nil {
		return dashboardSnapshot{}, err
	}
	if err := h.db.Model(&model.UserFeedback{}).Where("status = ?", model.FeedbackStatusPending).Count(&summary.PendingFeedbackTotal).Error; err != nil {
		return dashboardSnapshot{}, err
	}
	if err := h.db.Model(&model.AdminUser{}).Count(&summary.AdminUserTotal).Error; err != nil {
		return dashboardSnapshot{}, err
	}

	var kookMemberTotal int64
	if err := h.db.Model(&model.KookMember{}).Count(&kookMemberTotal).Error; err != nil {
		return dashboardSnapshot{}, err
	}

	recentTakeovers, err := h.dashboardRecentTakeovers(now)
	if err != nil {
		return dashboardSnapshot{}, err
	}

	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	usage, err := h.kookVoiceChannelUsageSummary(start, now)
	if err != nil {
		return dashboardSnapshot{}, err
	}
	userStats, err := h.kookVoiceUserStats(start, now, "", "")
	if err != nil {
		return dashboardSnapshot{}, err
	}

	channelNames := h.kookVoiceChannelNames()
	voiceUsage, totalDuration, activeUsers, activeChannels := dashboardChannelUsage(usage, channelNames)
	return dashboardSnapshot{
		Summary:         summary,
		RecentTakeovers: recentTakeovers,
		KookMemberTotal: kookMemberTotal,
		KookUsage:       voiceUsage,
		VoiceStats: dashboardVoiceStats{
			Range: dashboardRange{
				StartTime: start.Format("2006-01-02 15:04:05"),
				EndTime:   now.Format("2006-01-02 15:04:05"),
			},
			UserStats:          dashboardTopVoiceUsers(userStats),
			TotalDuration:      totalDuration,
			ActiveUserTotal:    activeUsers,
			ActiveChannelTotal: activeChannels,
		},
	}, nil
}

func (h *Handler) dashboardRecentTakeovers(now time.Time) ([]dashboardTakeover, error) {
	rows := []dashboardTakeoverRow{}
	err := h.takeoverListQuery(0).
		Joins("LEFT JOIN ttw_user AS creator ON creator.id = ttw_takeover.creator_user_id").
		Select("ttw_takeover.*, COALESCE(j.joined_count, 0) AS joined_count, creator.nickname AS creator_name").
		Where("ttw_takeover.is_deleted = ?", false).
		Order("ttw_takeover.gmt_create DESC").
		Limit(8).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	list := make([]dashboardTakeover, 0, len(rows))
	for _, row := range rows {
		takeover := row.Takeover
		if isTakeoverExpiredAt(takeover, now) {
			takeover.TakeoverState = model.TakeoverStateClosed
		}
		list = append(list, dashboardTakeover{
			ID:               takeover.ID,
			Title:            takeover.Title,
			StatusLabel:      takeoverStatusLabel(takeover, row.JoinedCount),
			JoinedCount:      row.JoinedCount,
			ParticipantLimit: takeover.ParticipantLimit,
			ScheduleText:     scheduleText(takeover),
			CreatorName:      stringValue(row.CreatorName),
			KookChannelName:  stringValue(takeover.KookChannelName),
			CreatedAt:        takeover.GmtCreate.Format("2006-01-02 15:04:05"),
		})
	}
	return list, nil
}

func dashboardChannelUsage(usage []kookVoiceChannelUsageDTO, channelNames map[string]string) ([]dashboardKookUsage, int64, int64, int64) {
	result := make([]dashboardKookUsage, 0, len(usage))
	var totalDuration, activeUsers, activeChannels int64
	for _, item := range usage {
		totalDuration += item.DurationSeconds
		activeUsers += item.ActiveUserCount
		if item.ActiveUserCount > 0 {
			activeChannels++
		}
		result = append(result, dashboardKookUsage{
			ChannelID:       item.ChannelID,
			ChannelName:     channelNames[item.ChannelID],
			DurationSeconds: item.DurationSeconds,
			DurationText:    item.DurationText,
			SessionCount:    item.SessionCount,
			ActiveUserCount: item.ActiveUserCount,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ActiveUserCount == result[j].ActiveUserCount {
			return result[i].DurationSeconds > result[j].DurationSeconds
		}
		return result[i].ActiveUserCount > result[j].ActiveUserCount
	})
	if len(result) > 5 {
		result = result[:5]
	}
	return result, totalDuration, activeUsers, activeChannels
}

func dashboardTopVoiceUsers(users []kookVoiceUsageDTO) []dashboardVoiceUser {
	if len(users) > 5 {
		users = users[:5]
	}
	result := make([]dashboardVoiceUser, 0, len(users))
	for _, user := range users {
		result = append(result, dashboardVoiceUser{
			KookUserID:      user.KookUserID,
			Username:        user.Username,
			Nickname:        user.Nickname,
			DurationSeconds: user.DurationSeconds,
			DurationText:    user.DurationText,
			SessionCount:    user.SessionCount,
		})
	}
	return result
}
