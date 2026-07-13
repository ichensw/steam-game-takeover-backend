package httpapi

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestValidateKookChannelSortConfig(t *testing.T) {
	weekday := 3
	monthday := 31
	tests := []struct {
		name    string
		config  kookChannelSortConfigDTO
		wantErr bool
	}{
		{name: "disabled configuration may stay empty", config: kookChannelSortConfigDTO{}},
		{name: "daily", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleDaily, Hour: 4}},
		{name: "weekly", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleWeekly, Weekday: &weekday, Hour: 4}},
		{name: "monthly", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleMonthly, Monthday: &monthday, Hour: 4}},
		{name: "missing group", config: kookChannelSortConfigDTO{Enabled: true, ScheduleType: kookChannelSortScheduleDaily, Hour: 4}, wantErr: true},
		{name: "invalid schedule", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: "sometimes", Hour: 4}, wantErr: true},
		{name: "missing weekday", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleWeekly, Hour: 4}, wantErr: true},
		{name: "invalid weekday", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleWeekly, Weekday: intPointer(8), Hour: 4}, wantErr: true},
		{name: "missing monthday", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleMonthly, Hour: 4}, wantErr: true},
		{name: "invalid monthday", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleMonthly, Monthday: intPointer(32), Hour: 4}, wantErr: true},
		{name: "invalid hour", config: kookChannelSortConfigDTO{Enabled: true, GroupIDs: []string{"group-1"}, ScheduleType: kookChannelSortScheduleDaily, Hour: 24}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateKookChannelSortConfig(test.config)
			if (err != nil) != test.wantErr {
				t.Fatalf("validate error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestPrepareKookChannelSortConfigSerializesGroupsAndNextRun(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 13, 10, 0, 0, 0, location)
	input := kookChannelSortConfigDTO{
		Enabled:      true,
		GroupIDs:     []string{"group-1", "group-2"},
		ScheduleType: kookChannelSortScheduleDaily,
		Hour:         4,
	}

	row, err := prepareKookChannelSortConfig(input, now, location)
	if err != nil {
		t.Fatalf("prepare config: %v", err)
	}
	if row.GroupIDs != `["group-1","group-2"]` {
		t.Fatalf("group JSON = %s", row.GroupIDs)
	}
	if row.NextRunAt == nil || !row.NextRunAt.Equal(time.Date(2026, time.July, 14, 4, 0, 0, 0, location)) {
		t.Fatalf("next run = %v", row.NextRunAt)
	}

	dto, err := kookChannelSortConfigFromModel(row)
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if len(dto.GroupIDs) != 2 || dto.GroupIDs[1] != "group-2" {
		t.Fatalf("decoded groups = %#v", dto.GroupIDs)
	}
}

func TestKookChannelSortLeaseAcquireUsesAtomicExpiryCondition(t *testing.T) {
	db := newTakeoverExpirationDryRunDB(t)
	sqlText := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return kookChannelSortLeaseAcquireUpdate(tx, "owner-1", 5*time.Minute)
	})
	for _, fragment := range []string{
		"UPDATE `ttw_kook_channel_sort_config`",
		"`lock_token`='owner-1'",
		"`locked_until`=DATE_ADD(NOW(), INTERVAL 300000000 MICROSECOND)",
		"id = 1",
		"locked_until IS NULL OR locked_until < NOW()",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("lease SQL missing %q: %s", fragment, sqlText)
		}
	}
}

func TestKookChannelSortRoutesAreRegistered(t *testing.T) {
	routes := NewRouter(&Handler{}).Routes()
	want := map[string]string{
		"GET /api/admin/kook-channel-sort/config": "",
		"PUT /api/admin/kook-channel-sort/config": "",
		"GET /api/admin/kook-channel-sort/runs":   "",
	}
	for _, route := range routes {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing routes: %#v", want)
	}
}

func intPointer(value int) *int { return &value }
