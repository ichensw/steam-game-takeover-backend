package httpapi

import (
	"testing"
	"time"
)

func TestMergeKookVoiceIntervals(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	minute := func(offset int) time.Time { return start.Add(time.Duration(offset) * time.Minute) }
	exit := func(offset int) *time.Time {
		value := minute(offset)
		return &value
	}

	tests := []struct {
		name      string
		intervals []kookVoiceInterval
		want      map[kookVoiceChannelKey]int64
	}{
		{
			name: "fully overlapping sessions count once",
			intervals: []kookVoiceInterval{
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: start, ExitedAt: exit(1)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: start, ExitedAt: exit(1)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: start, ExitedAt: exit(1)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: start, ExitedAt: exit(1)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: start, ExitedAt: exit(1)},
			},
			want: map[kookVoiceChannelKey]int64{{GuildID: "guild-1", ChannelID: "voice-1"}: 60},
		},
		{
			name: "partially overlapping sessions are merged",
			intervals: []kookVoiceInterval{
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(1), ExitedAt: exit(3)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(0), ExitedAt: exit(2)},
			},
			want: map[kookVoiceChannelKey]int64{{GuildID: "guild-1", ChannelID: "voice-1"}: 180},
		},
		{
			name: "touching sessions form one continuous interval",
			intervals: []kookVoiceInterval{
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(0), ExitedAt: exit(1)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(1), ExitedAt: exit(2)},
			},
			want: map[kookVoiceChannelKey]int64{{GuildID: "guild-1", ChannelID: "voice-1"}: 120},
		},
		{
			name: "disjoint sessions are added",
			intervals: []kookVoiceInterval{
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(0), ExitedAt: exit(1)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(2), ExitedAt: exit(3)},
			},
			want: map[kookVoiceChannelKey]int64{{GuildID: "guild-1", ChannelID: "voice-1"}: 120},
		},
		{
			name: "sessions are clipped at both range boundaries",
			intervals: []kookVoiceInterval{
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(-1), ExitedAt: exit(1)},
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(59), ExitedAt: exit(61)},
			},
			want: map[kookVoiceChannelKey]int64{{GuildID: "guild-1", ChannelID: "voice-1"}: 120},
		},
		{
			name: "active session ends at supplied range end",
			intervals: []kookVoiceInterval{
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(55), ExitedAt: nil},
			},
			want: map[kookVoiceChannelKey]int64{{GuildID: "guild-1", ChannelID: "voice-1"}: 300},
		},
		{
			name: "guild and channel together identify occupancy",
			intervals: []kookVoiceInterval{
				{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: minute(0), ExitedAt: exit(1)},
				{GuildID: "guild-2", ChannelID: "voice-1", JoinedAt: minute(0), ExitedAt: exit(2)},
				{GuildID: "guild-1", ChannelID: "voice-2", JoinedAt: minute(0), ExitedAt: exit(3)},
			},
			want: map[kookVoiceChannelKey]int64{
				{GuildID: "guild-1", ChannelID: "voice-1"}: 60,
				{GuildID: "guild-2", ChannelID: "voice-1"}: 120,
				{GuildID: "guild-1", ChannelID: "voice-2"}: 180,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeKookVoiceIntervals(tt.intervals, start, end)
			if len(got) != len(tt.want) {
				t.Fatalf("result has %d channels, want %d: %#v", len(got), len(tt.want), got)
			}
			for key, want := range tt.want {
				if got[key] != want {
					t.Fatalf("occupied duration for %#v = %d, want %d", key, got[key], want)
				}
			}
		})
	}
}

func TestKookVoiceUsageKeepsSummedDurationAlongsideOccupancy(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	exited := start.Add(time.Minute)
	intervals := make([]kookVoiceInterval, 5)
	for i := range intervals {
		intervals[i] = kookVoiceInterval{
			GuildID:   "guild-1",
			ChannelID: "voice-1",
			JoinedAt:  start,
			ExitedAt:  &exited,
		}
	}
	list := []kookVoiceChannelUsageDTO{{ChannelID: "voice-1", DurationSeconds: 300}}
	attachKookVoiceOccupancy(list, "guild-1", mergeKookVoiceIntervals(intervals, start, start.Add(time.Hour)), int64(time.Hour.Seconds()))

	if list[0].DurationSeconds != 300 {
		t.Fatalf("usage duration = %d, want 300", list[0].DurationSeconds)
	}
	if list[0].OccupiedDurationSeconds != 60 {
		t.Fatalf("occupied duration = %d, want 60", list[0].OccupiedDurationSeconds)
	}
	if list[0].OccupiedDurationText != "1分钟0秒" {
		t.Fatalf("occupied duration text = %q, want %q", list[0].OccupiedDurationText, "1分钟0秒")
	}
	if list[0].IdleDurationSeconds != 3540 {
		t.Fatalf("idle duration = %d, want 3540", list[0].IdleDurationSeconds)
	}
}

func TestKookVoiceStatsChannelUsageShowsOccupancyAndIdle(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	exited := start.Add(time.Minute)
	intervals := make([]kookVoiceInterval, 5)
	for i := range intervals {
		intervals[i] = kookVoiceInterval{
			GuildID:   "guild-1",
			ChannelID: "voice-1",
			JoinedAt:  start,
			ExitedAt:  &exited,
		}
	}
	list := []kookVoiceUsageDTO{{GuildID: "guild-1", ChannelID: "voice-1", DurationSeconds: 300}}
	attachKookVoiceUsageOccupancy(list, mergeKookVoiceIntervals(intervals, start, start.Add(time.Hour)), int64(time.Hour.Seconds()))

	if list[0].DurationSeconds != 300 {
		t.Fatalf("summed duration = %d, want 300", list[0].DurationSeconds)
	}
	if list[0].OccupiedDurationSeconds != 60 {
		t.Fatalf("occupied duration = %d, want 60", list[0].OccupiedDurationSeconds)
	}
	if list[0].IdleDurationSeconds != 3540 {
		t.Fatalf("idle duration = %d, want 3540", list[0].IdleDurationSeconds)
	}
}
