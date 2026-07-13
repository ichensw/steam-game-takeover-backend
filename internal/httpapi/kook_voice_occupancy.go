package httpapi

import (
	"sort"
	"time"
)

type kookVoiceInterval struct {
	GuildID   string
	ChannelID string
	JoinedAt  time.Time
	ExitedAt  *time.Time
}

type kookVoiceChannelKey struct {
	GuildID   string
	ChannelID string
}

type kookVoiceTimeRange struct {
	start time.Time
	end   time.Time
}

func mergeKookVoiceIntervals(intervals []kookVoiceInterval, rangeStart, rangeEnd time.Time) map[kookVoiceChannelKey]int64 {
	result := make(map[kookVoiceChannelKey]int64)
	if !rangeEnd.After(rangeStart) {
		return result
	}

	rangesByChannel := make(map[kookVoiceChannelKey][]kookVoiceTimeRange)
	for _, interval := range intervals {
		end := rangeEnd
		if interval.ExitedAt != nil {
			end = minTime(*interval.ExitedAt, rangeEnd)
		}
		start := maxTime(interval.JoinedAt, rangeStart)
		if !end.After(start) {
			continue
		}
		key := kookVoiceChannelKey{GuildID: interval.GuildID, ChannelID: interval.ChannelID}
		rangesByChannel[key] = append(rangesByChannel[key], kookVoiceTimeRange{start: start, end: end})
	}

	for key, ranges := range rangesByChannel {
		sort.Slice(ranges, func(i, j int) bool {
			if ranges[i].start.Equal(ranges[j].start) {
				return ranges[i].end.Before(ranges[j].end)
			}
			return ranges[i].start.Before(ranges[j].start)
		})
		current := ranges[0]
		for _, next := range ranges[1:] {
			if !next.start.After(current.end) {
				current.end = maxTime(current.end, next.end)
				continue
			}
			result[key] += int64(current.end.Sub(current.start).Seconds())
			current = next
		}
		result[key] += int64(current.end.Sub(current.start).Seconds())
	}
	return result
}

func attachKookVoiceOccupancy(list []kookVoiceChannelUsageDTO, guildID string, occupied map[kookVoiceChannelKey]int64) {
	for i := range list {
		seconds := occupied[kookVoiceChannelKey{GuildID: guildID, ChannelID: list[i].ChannelID}]
		list[i].OccupiedDurationSeconds = seconds
		list[i].OccupiedDurationText = durationText(seconds)
	}
}
