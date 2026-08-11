package httpapi

import "testing"

func TestCreatorPointUnits(t *testing.T) {
	tests := []struct {
		name             string
		joinedCount      int
		participantLimit uint
		want             int64
	}{
		{name: "full", joinedCount: 5, participantLimit: 5, want: 22},
		{name: "partial", joinedCount: 3, participantLimit: 5, want: 17},
		{name: "capped", joinedCount: 6, participantLimit: 5, want: 22},
		{name: "empty", joinedCount: 0, participantLimit: 5, want: 10},
		{name: "invalid limit", joinedCount: 3, participantLimit: 0, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := creatorPointUnits(tt.joinedCount, tt.participantLimit); got != tt.want {
				t.Fatalf("creatorPointUnits(%d, %d) = %d, want %d", tt.joinedCount, tt.participantLimit, got, tt.want)
			}
		})
	}
}

func TestPointLevel(t *testing.T) {
	tests := []struct {
		units        uint64
		name         string
		nextName     string
		progress     int
		nextAtPoints float64
	}{
		{units: 0, name: "新人", nextName: "兔友", progress: 0, nextAtPoints: 10},
		{units: 50, name: "新人", nextName: "兔友", progress: 50, nextAtPoints: 10},
		{units: 100, name: "兔友", nextName: "活跃", progress: 0, nextAtPoints: 30},
		{units: 450, name: "活跃", nextName: "核心", progress: 50, nextAtPoints: 60},
		{units: 2400, name: "王牌", nextName: "", progress: 100, nextAtPoints: 240},
	}

	for _, tt := range tests {
		level := pointLevel(tt.units)
		if level.Name != tt.name || level.NextName != tt.nextName || level.Progress != tt.progress || level.NextAtPoints != tt.nextAtPoints {
			t.Fatalf("pointLevel(%d) = %+v", tt.units, level)
		}
	}
}

func TestPointValueRequiresOneDecimal(t *testing.T) {
	tests := []struct {
		value float64
		units int64
		ok    bool
	}{
		{value: 1, units: 10, ok: true},
		{value: -2.5, units: -25, ok: true},
		{value: 0.1, units: 1, ok: true},
		{value: 1.25, ok: false},
		{value: 0, ok: false},
	}

	for _, tt := range tests {
		units, ok := pointValueToUnits(tt.value)
		if units != tt.units || ok != tt.ok {
			t.Fatalf("pointValueToUnits(%v) = (%d, %v), want (%d, %v)", tt.value, units, ok, tt.units, tt.ok)
		}
	}
}

func TestRankedPointItemsShareCompetitionRank(t *testing.T) {
	rows := []pointRankingRow{
		{UserID: 1, PointsUnits: 100},
		{UserID: 2, PointsUnits: 100},
		{UserID: 3, PointsUnits: 90},
	}
	items := rankedPointItems(rows)
	if items[0].Rank != 1 || items[1].Rank != 1 || items[2].Rank != 3 {
		t.Fatalf("unexpected ranks: %d, %d, %d", items[0].Rank, items[1].Rank, items[2].Rank)
	}
}

func TestRankedPointItemsUseAccountPointsForLevel(t *testing.T) {
	items := rankedPointItems([]pointRankingRow{{
		UserID:             1,
		PointsUnits:        100,
		AccountPointsUnits: 2400,
	}})
	if items[0].Points != 10 || items[0].Level != "王牌" {
		t.Fatalf("unexpected ranking item: %+v", items[0])
	}
}

func TestPointRoutesAreRegistered(t *testing.T) {
	want := map[string]bool{
		"GET /api/point-rankings":                     true,
		"GET /api/me/point-logs":                      true,
		"GET /api/admin/users/:userId/point-logs":     true,
		"POST /api/admin/users/:userId/points/adjust": true,
	}
	for _, route := range NewRouter(&Handler{}).Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing routes: %#v", want)
	}
}
