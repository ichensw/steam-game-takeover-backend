package httpapi

import (
	"fmt"
	"sort"
	"time"
)

const kookChannelSortGroupSize = 15

const (
	kookChannelSortScheduleDaily   = "daily"
	kookChannelSortScheduleWeekly  = "weekly"
	kookChannelSortScheduleMonthly = "monthly"
)

type kookChannelSortUsage struct {
	UsageSeconds    int64 `json:"usageSeconds"`
	OccupiedSeconds int64 `json:"occupiedSeconds"`
}

type kookChannelMove struct {
	ChannelID       string `json:"channelId"`
	ChannelName     string `json:"channelName"`
	FromParentID    string `json:"fromParentId"`
	ToParentID      string `json:"toParentId"`
	FromLevel       int    `json:"fromLevel"`
	ToLevel         int    `json:"toLevel"`
	UsageSeconds    int64  `json:"usageSeconds"`
	OccupiedSeconds int64  `json:"occupiedSeconds"`
}

type kookChannelSortGroup struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	KookSort int               `json:"kookSort"`
	Channels []kookChannelMove `json:"channels"`
}

type kookChannelSortPlan struct {
	Groups []kookChannelSortGroup `json:"groups"`
	Moves  []kookChannelMove      `json:"moves"`
}

type kookChannelSortSchedule struct {
	ScheduleType string `json:"scheduleType"`
	Weekday      int    `json:"weekday"`
	Monthday     int    `json:"monthday"`
	Hour         int    `json:"hour"`
}

func buildKookChannelSortPlan(channels []kookChannelDTO, selectedGroupIDs []string, usage map[string]kookChannelSortUsage) (kookChannelSortPlan, error) {
	selected := make(map[string]struct{}, len(selectedGroupIDs))
	for _, id := range selectedGroupIDs {
		if id == "" {
			return kookChannelSortPlan{}, fmt.Errorf("selected KOOK group ID cannot be empty")
		}
		if _, exists := selected[id]; exists {
			return kookChannelSortPlan{}, fmt.Errorf("KOOK group %q is selected more than once", id)
		}
		selected[id] = struct{}{}
	}
	if len(selected) == 0 {
		return kookChannelSortPlan{}, fmt.Errorf("at least one KOOK group must be selected")
	}

	groups := make([]kookChannelDTO, 0, len(selected))
	for _, channel := range channels {
		if _, ok := selected[channel.ID]; ok && channel.Type == 0 {
			groups = append(groups, channel)
		}
	}
	if len(groups) != len(selected) {
		return kookChannelSortPlan{}, fmt.Errorf("one or more selected KOOK groups are missing or invalid")
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].KookSort < groups[j].KookSort
	})

	groupOrder := make(map[string]int, len(groups))
	for i, group := range groups {
		groupOrder[group.ID] = i
	}

	voices := make([]kookChannelDTO, 0, len(channels))
	for _, channel := range channels {
		if channel.Type == 2 {
			if _, ok := groupOrder[channel.ParentID]; ok {
				voices = append(voices, channel)
			}
		}
	}
	sort.SliceStable(voices, func(i, j int) bool {
		leftGroup, rightGroup := groupOrder[voices[i].ParentID], groupOrder[voices[j].ParentID]
		if leftGroup != rightGroup {
			return leftGroup < rightGroup
		}
		return voices[i].KookSort < voices[j].KookSort
	})
	sort.SliceStable(voices, func(i, j int) bool {
		return usage[voices[i].ID].OccupiedSeconds > usage[voices[j].ID].OccupiedSeconds
	})

	plan := kookChannelSortPlan{
		Groups: make([]kookChannelSortGroup, 0, len(groups)),
		Moves:  make([]kookChannelMove, 0, len(voices)),
	}
	voiceIndex := 0
	for groupIndex, group := range groups {
		count := len(voices) - voiceIndex
		if groupIndex < len(groups)-1 && count > kookChannelSortGroupSize {
			count = kookChannelSortGroupSize
		}
		targetLevels := kookChannelTargetLevels(channels, group.ID, count)
		plannedGroup := kookChannelSortGroup{
			ID:       group.ID,
			Name:     group.Name,
			KookSort: group.KookSort,
			Channels: make([]kookChannelMove, 0, count),
		}
		for i := 0; i < count; i++ {
			channel := voices[voiceIndex+i]
			channelUsage := usage[channel.ID]
			move := kookChannelMove{
				ChannelID:       channel.ID,
				ChannelName:     channel.Name,
				FromParentID:    channel.ParentID,
				ToParentID:      group.ID,
				FromLevel:       channel.KookSort,
				ToLevel:         targetLevels[i],
				UsageSeconds:    channelUsage.UsageSeconds,
				OccupiedSeconds: channelUsage.OccupiedSeconds,
			}
			plannedGroup.Channels = append(plannedGroup.Channels, move)
			if move.FromParentID != move.ToParentID || move.FromLevel != move.ToLevel {
				plan.Moves = append(plan.Moves, move)
			}
		}
		voiceIndex += count
		plan.Groups = append(plan.Groups, plannedGroup)
	}
	return plan, nil
}

func kookChannelTargetLevels(channels []kookChannelDTO, groupID string, count int) []int {
	levels := make([]int, 0, count)
	maxLevel := 0
	for _, channel := range channels {
		if channel.ParentID != groupID {
			continue
		}
		if channel.KookSort > maxLevel {
			maxLevel = channel.KookSort
		}
		if channel.Type == 2 {
			levels = append(levels, channel.KookSort)
		}
	}
	sort.Ints(levels)
	if len(levels) > count {
		levels = levels[:count]
	}
	for len(levels) < count {
		maxLevel += 100
		levels = append(levels, maxLevel)
	}
	return levels
}

func previousKookChannelSortRange(scheduleType string, now time.Time, location *time.Location) (time.Time, time.Time, error) {
	if location == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("KOOK channel sort location is required")
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	switch scheduleType {
	case kookChannelSortScheduleDaily:
		return today.AddDate(0, 0, -1), today, nil
	case kookChannelSortScheduleWeekly:
		currentMonday := today.AddDate(0, 0, -daysSinceMonday(today.Weekday()))
		return currentMonday.AddDate(0, 0, -7), currentMonday, nil
	case kookChannelSortScheduleMonthly:
		currentMonth := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
		return currentMonth.AddDate(0, -1, 0), currentMonth, nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid KOOK channel sort schedule type %q", scheduleType)
	}
}

func nextKookChannelSortRun(now time.Time, schedule kookChannelSortSchedule, location *time.Location) (time.Time, error) {
	if location == nil {
		return time.Time{}, fmt.Errorf("KOOK channel sort location is required")
	}
	if err := validateKookChannelSortSchedule(schedule); err != nil {
		return time.Time{}, err
	}

	localNow := now.In(location)
	var candidate time.Time
	switch schedule.ScheduleType {
	case kookChannelSortScheduleDaily:
		candidate = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), schedule.Hour, 0, 0, 0, location)
		if !candidate.After(localNow) {
			candidate = candidate.AddDate(0, 0, 1)
		}
	case kookChannelSortScheduleWeekly:
		monday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), schedule.Hour, 0, 0, 0, location).
			AddDate(0, 0, -daysSinceMonday(localNow.Weekday()))
		candidate = monday.AddDate(0, 0, schedule.Weekday-1)
		if !candidate.After(localNow) {
			candidate = candidate.AddDate(0, 0, 7)
		}
	case kookChannelSortScheduleMonthly:
		candidate = kookChannelSortMonthRun(localNow.Year(), localNow.Month(), schedule.Monthday, schedule.Hour, location)
		if !candidate.After(localNow) {
			nextMonth := time.Date(localNow.Year(), localNow.Month()+1, 1, 0, 0, 0, 0, location)
			candidate = kookChannelSortMonthRun(nextMonth.Year(), nextMonth.Month(), schedule.Monthday, schedule.Hour, location)
		}
	}
	return candidate, nil
}

func validateKookChannelSortSchedule(schedule kookChannelSortSchedule) error {
	if schedule.Hour < 0 || schedule.Hour > 23 {
		return fmt.Errorf("KOOK channel sort hour must be between 0 and 23")
	}
	switch schedule.ScheduleType {
	case kookChannelSortScheduleDaily:
		return nil
	case kookChannelSortScheduleWeekly:
		if schedule.Weekday < 1 || schedule.Weekday > 7 {
			return fmt.Errorf("KOOK channel sort weekday must be between 1 and 7")
		}
		return nil
	case kookChannelSortScheduleMonthly:
		if schedule.Monthday < 1 || schedule.Monthday > 31 {
			return fmt.Errorf("KOOK channel sort monthday must be between 1 and 31")
		}
		return nil
	default:
		return fmt.Errorf("invalid KOOK channel sort schedule type %q", schedule.ScheduleType)
	}
}

func daysSinceMonday(weekday time.Weekday) int {
	return (int(weekday) + 6) % 7
}

func kookChannelSortMonthRun(year int, month time.Month, monthday, hour int, location *time.Location) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, location).Day()
	if monthday > lastDay {
		monthday = lastDay
	}
	return time.Date(year, month, monthday, hour, 0, 0, 0, location)
}
