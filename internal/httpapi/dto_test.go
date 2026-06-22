package httpapi

import (
	"testing"
	"time"

	"steam-game-takeover-backend/internal/model"
)

func TestTakeoverStatusLabelMarksExpiredSchedulesEnded(t *testing.T) {
	now := time.Now()
	yesterday := truncateDate(now.AddDate(0, 0, -1))
	tomorrow := truncateDate(now.AddDate(0, 0, 1))

	cases := []struct {
		name        string
		takeover    model.Takeover
		joinedCount int64
		want        string
	}{
		{
			name: "specified date expired",
			takeover: model.Takeover{
				ScheduleType:     model.ScheduleSpecifiedDate,
				StartDate:        &yesterday,
				PlayTime:         "23:59:00",
				ParticipantLimit: 4,
			},
			joinedCount: 4,
			want:        "已结束",
		},
		{
			name: "date range expired by end date time",
			takeover: model.Takeover{
				ScheduleType:     model.ScheduleDateRange,
				StartDate:        &yesterday,
				EndDate:          &yesterday,
				PlayTime:         "23:59:00",
				ParticipantLimit: 4,
			},
			joinedCount: 1,
			want:        "已结束",
		},
		{
			name: "future date not expired",
			takeover: model.Takeover{
				ScheduleType:     model.ScheduleSpecifiedDate,
				StartDate:        &tomorrow,
				PlayTime:         "00:00:00",
				ParticipantLimit: 4,
			},
			joinedCount: 4,
			want:        "已满员",
		},
		{
			name: "daily never expires",
			takeover: model.Takeover{
				ScheduleType:     model.ScheduleDaily,
				PlayTime:         "00:00:00",
				ParticipantLimit: 4,
			},
			joinedCount: 0,
			want:        "招募中",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := takeoverStatusLabel(tt.takeover, tt.joinedCount); got != tt.want {
				t.Fatalf("takeoverStatusLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSortTakeoverListOrdersRecruitingFullThenOthers(t *testing.T) {
	list := []takeoverDTO{
		{ID: 1, StatusLabel: "招募中"},
		{ID: 2, StatusLabel: "已结束"},
		{ID: 3, StatusLabel: "已满员"},
		{ID: 4, StatusLabel: "招募中"},
		{ID: 5, StatusLabel: "招募中"},
		{ID: 6, StatusLabel: "已满员"},
	}

	sortTakeoverList(list)

	wantIDs := []uint64{5, 4, 1, 6, 3, 2}
	for index, wantID := range wantIDs {
		if list[index].ID != wantID {
			t.Fatalf("list[%d].ID = %d, want %d", index, list[index].ID, wantID)
		}
	}
}
