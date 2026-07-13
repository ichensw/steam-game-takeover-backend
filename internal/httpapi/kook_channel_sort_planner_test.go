package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestBuildKookChannelSortPlanUsesLiveOrderAndDistributesOverflow(t *testing.T) {
	groups := []kookChannelDTO{
		{ID: "group-3", Name: "3区", Type: 0, Level: 33, KookSort: 300},
		{ID: "group-1", Name: "1区", Type: 0, Level: 11, KookSort: 100},
		{ID: "group-5", Name: "5区", Type: 0, Level: 55, KookSort: 500},
		{ID: "group-2", Name: "2区", Type: 0, Level: 22, KookSort: 200},
		{ID: "group-4", Name: "4区", Type: 0, Level: 44, KookSort: 400},
	}
	channels := append([]kookChannelDTO{}, groups...)
	usage := make(map[string]kookChannelSortUsage, 78)

	// Add channels in reverse slice order so the planner must use live KOOK order.
	for i := 77; i >= 0; i-- {
		groupIndex, groupOffset := initialVoicePosition(i)
		id := fmt.Sprintf("voice-%02d", i)
		channels = append(channels, kookChannelDTO{
			ID:       id,
			Name:     id,
			Type:     2,
			ParentID: fmt.Sprintf("group-%d", groupIndex+1),
			Level:    9000 + i,
			KookSort: (groupOffset + 1) * 100,
		})
		usage[id] = kookChannelSortUsage{
			UsageSeconds:    int64((78 - i) * 300),
			OccupiedSeconds: int64(10_000 - i/2),
		}
	}
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("text-%d", i)
		channels = append(channels, kookChannelDTO{
			ID: id, Name: id, Type: 1, ParentID: fmt.Sprintf("group-%d", i), KookSort: 50,
		})
		usage[id] = kookChannelSortUsage{OccupiedSeconds: 99_999}
	}
	channels = append(channels,
		kookChannelDTO{ID: "unselected", Type: 2, ParentID: "other-group", KookSort: 100},
		kookChannelDTO{ID: "nested", Type: 2, ParentID: "voice-00", KookSort: 100},
	)

	plan, err := buildKookChannelSortPlan(channels, []string{
		"group-5", "group-3", "group-1", "group-4", "group-2",
	}, usage)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	wantCounts := []int{15, 15, 15, 15, 18}
	var position int
	for i, group := range plan.Groups {
		wantGroupID := fmt.Sprintf("group-%d", i+1)
		if group.ID != wantGroupID {
			t.Fatalf("group %d ID = %q, want %q", i, group.ID, wantGroupID)
		}
		if len(group.Channels) != wantCounts[i] {
			t.Fatalf("group %s has %d channels, want %d", group.ID, len(group.Channels), wantCounts[i])
		}
		for _, channel := range group.Channels {
			wantID := fmt.Sprintf("voice-%02d", position)
			if channel.ChannelID != wantID {
				t.Fatalf("planned position %d = %q, want %q", position, channel.ChannelID, wantID)
			}
			if channel.ToParentID != wantGroupID {
				t.Fatalf("channel %s target group = %q, want %q", channel.ChannelID, channel.ToParentID, wantGroupID)
			}
			position++
		}
	}
	if position != 78 {
		t.Fatalf("planned %d voice channels, want 78", position)
	}

	original := make(map[string]kookChannelDTO, len(channels))
	for _, channel := range channels {
		original[channel.ID] = channel
	}
	for _, move := range plan.Moves {
		channel := original[move.ChannelID]
		if channel.Type != 2 {
			t.Fatalf("non-voice channel %q appeared in moves", move.ChannelID)
		}
		if move.FromLevel != channel.KookSort {
			t.Fatalf("move %s uses from level %d, want KOOK sort %d", move.ChannelID, move.FromLevel, channel.KookSort)
		}
		if move.FromParentID == move.ToParentID && move.FromLevel == move.ToLevel {
			t.Fatalf("unchanged channel %q appeared in moves", move.ChannelID)
		}
	}
}

func TestBuildKookChannelSortPlanRejectsMissingSelectedGroup(t *testing.T) {
	_, err := buildKookChannelSortPlan(
		[]kookChannelDTO{{ID: "group-1", Type: 0, KookSort: 100}},
		[]string{"group-1", "missing"},
		nil,
	)
	if err == nil {
		t.Fatal("expected an error for a missing selected group")
	}
}

func initialVoicePosition(index int) (groupIndex, groupOffset int) {
	// The live structure starts with 16, 16, 16, 15, and 15 voice channels.
	for groupIndex, count := range []int{16, 16, 16, 15, 15} {
		if index < count {
			return groupIndex, index
		}
		index -= count
	}
	panic("voice index out of range")
}

func TestPreviousKookChannelSortRangeUsesCompleteNaturalPeriod(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 15, 16, 30, 0, 0, location)
	tests := []struct {
		name         string
		scheduleType string
		wantStart    time.Time
		wantEnd      time.Time
	}{
		{
			name:         "daily prior day",
			scheduleType: kookChannelSortScheduleDaily,
			wantStart:    time.Date(2026, time.July, 14, 0, 0, 0, 0, location),
			wantEnd:      time.Date(2026, time.July, 15, 0, 0, 0, 0, location),
		},
		{
			name:         "weekly prior Monday through Sunday",
			scheduleType: kookChannelSortScheduleWeekly,
			wantStart:    time.Date(2026, time.July, 6, 0, 0, 0, 0, location),
			wantEnd:      time.Date(2026, time.July, 13, 0, 0, 0, 0, location),
		},
		{
			name:         "monthly prior calendar month",
			scheduleType: kookChannelSortScheduleMonthly,
			wantStart:    time.Date(2026, time.June, 1, 0, 0, 0, 0, location),
			wantEnd:      time.Date(2026, time.July, 1, 0, 0, 0, 0, location),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := previousKookChannelSortRange(test.scheduleType, now, location)
			if err != nil {
				t.Fatalf("previous range: %v", err)
			}
			if !start.Equal(test.wantStart) || !end.Equal(test.wantEnd) {
				t.Fatalf("range = [%s, %s), want [%s, %s)", start, end, test.wantStart, test.wantEnd)
			}
		})
	}
}

func TestNextKookChannelSortRunUsesConfiguredWeekday(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	schedule := kookChannelSortSchedule{
		ScheduleType: kookChannelSortScheduleWeekly,
		Weekday:      3,
		Hour:         4,
	}
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "later this week",
			now:  time.Date(2026, time.July, 13, 10, 0, 0, 0, location),
			want: time.Date(2026, time.July, 15, 4, 0, 0, 0, location),
		},
		{
			name: "next week after this week's due hour",
			now:  time.Date(2026, time.July, 15, 5, 0, 0, 0, location),
			want: time.Date(2026, time.July, 22, 4, 0, 0, 0, location),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := nextKookChannelSortRun(test.now, schedule, location)
			if err != nil {
				t.Fatalf("next run: %v", err)
			}
			if !got.Equal(test.want) {
				t.Fatalf("next run = %s, want %s", got, test.want)
			}
		})
	}
}

func TestNextKookChannelSortRunFallsBackToMonthEnd(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	schedule := kookChannelSortSchedule{
		ScheduleType: kookChannelSortScheduleMonthly,
		Monthday:     31,
		Hour:         4,
	}
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "uses February last day",
			now:  time.Date(2027, time.February, 10, 8, 0, 0, 0, location),
			want: time.Date(2027, time.February, 28, 4, 0, 0, 0, location),
		},
		{
			name: "moves to next month after fallback has passed",
			now:  time.Date(2027, time.February, 28, 5, 0, 0, 0, location),
			want: time.Date(2027, time.March, 31, 4, 0, 0, 0, location),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := nextKookChannelSortRun(test.now, schedule, location)
			if err != nil {
				t.Fatalf("next run: %v", err)
			}
			if !got.Equal(test.want) {
				t.Fatalf("next run = %s, want %s", got, test.want)
			}
		})
	}
}

func TestNextKookChannelSortRunValidatesSchedule(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, location)
	tests := []kookChannelSortSchedule{
		{ScheduleType: "sometimes", Hour: 4},
		{ScheduleType: kookChannelSortScheduleDaily, Hour: 24},
		{ScheduleType: kookChannelSortScheduleWeekly, Weekday: 0, Hour: 4},
		{ScheduleType: kookChannelSortScheduleMonthly, Monthday: 32, Hour: 4},
	}
	for _, schedule := range tests {
		if _, err := nextKookChannelSortRun(now, schedule, location); err == nil {
			t.Fatalf("expected validation error for schedule %#v", schedule)
		}
	}
}
