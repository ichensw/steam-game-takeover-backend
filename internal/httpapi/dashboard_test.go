package httpapi

import "testing"

func TestDashboardChannelUsageKeepsOnlyTopFive(t *testing.T) {
	usage := []kookVoiceChannelUsageDTO{
		{ChannelID: "slow", DurationSeconds: 100, ActiveUserCount: 1},
		{ChannelID: "busy", DurationSeconds: 1, ActiveUserCount: 3},
		{ChannelID: "idle", DurationSeconds: 200, ActiveUserCount: 0},
		{ChannelID: "two", DurationSeconds: 50, ActiveUserCount: 2},
		{ChannelID: "three", DurationSeconds: 40, ActiveUserCount: 2},
		{ChannelID: "four", DurationSeconds: 30, ActiveUserCount: 2},
	}

	got, duration, activeUsers, activeChannels := dashboardChannelUsage(usage, map[string]string{"busy": "Busy"})
	if len(got) != 5 || got[0].ChannelID != "busy" || got[0].ChannelName != "Busy" {
		t.Fatalf("top usage = %#v", got)
	}
	if duration != 421 || activeUsers != 10 || activeChannels != 5 {
		t.Fatalf("totals = %d, %d, %d", duration, activeUsers, activeChannels)
	}
}
