package httpapi

import (
	"errors"
	"strings"
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
		{
			name: "closed state ended",
			takeover: model.Takeover{
				TakeoverState: model.TakeoverStateClosed,
				ScheduleType:  model.ScheduleDaily,
				PlayTime:      "00:00:00",
			},
			want: "已结束",
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

func TestTakeoverDTOIncludesKookInviteURL(t *testing.T) {
	url := "https://kook.top/abc"
	dto := toTakeoverDTO(model.Takeover{KookInviteURL: &url}, 0, false)
	if dto.KookInviteURL != url {
		t.Fatalf("KookInviteURL = %q, want %q", dto.KookInviteURL, url)
	}
}

func TestMemberRemarkNormalization(t *testing.T) {
	remark, err := normalizeMemberRemark("  可能晚到 5 分钟  ")
	if err != nil {
		t.Fatalf("normalizeMemberRemark() error = %v", err)
	}
	if remark == nil || *remark != "可能晚到 5 分钟" {
		t.Fatalf("remark = %v, want trimmed value", remark)
	}

	remark, err = normalizeMemberRemark("   ")
	if err != nil {
		t.Fatalf("normalizeMemberRemark(blank) error = %v", err)
	}
	if remark != nil {
		t.Fatalf("blank remark = %v, want nil", *remark)
	}

	if _, err := normalizeMemberRemark(strings.Repeat("字", 101)); !errors.Is(err, errRemarkTooLong) {
		t.Fatalf("normalizeMemberRemark(long) error = %v, want %v", err, errRemarkTooLong)
	}
}

func TestMemberDTOIncludesRemark(t *testing.T) {
	remark := "可能晚到"
	dto := toMemberDTO(memberRow{Remark: &remark}, false)
	if dto.Remark != remark {
		t.Fatalf("Remark = %q, want %q", dto.Remark, remark)
	}
}

func TestSchedulesConflict(t *testing.T) {
	day1 := truncateDate(time.Now().AddDate(0, 0, 1))
	day2 := day1.AddDate(0, 0, 1)
	day3 := day1.AddDate(0, 0, 2)

	cases := []struct {
		name string
		a    model.Takeover
		b    model.Takeover
		want bool
	}{
		{
			name: "same specified date and time",
			a:    model.Takeover{ScheduleType: model.ScheduleSpecifiedDate, StartDate: &day1, PlayTime: "20:00:00"},
			b:    model.Takeover{ScheduleType: model.ScheduleSpecifiedDate, StartDate: &day1, PlayTime: "20:00:00"},
			want: true,
		},
		{
			name: "different time",
			a:    model.Takeover{ScheduleType: model.ScheduleSpecifiedDate, StartDate: &day1, PlayTime: "20:00:00"},
			b:    model.Takeover{ScheduleType: model.ScheduleSpecifiedDate, StartDate: &day1, PlayTime: "21:00:00"},
		},
		{
			name: "range overlaps specified date",
			a:    model.Takeover{ScheduleType: model.ScheduleDateRange, StartDate: &day1, EndDate: &day3, PlayTime: "20:00:00"},
			b:    model.Takeover{ScheduleType: model.ScheduleSpecifiedDate, StartDate: &day2, PlayTime: "20:00:00"},
			want: true,
		},
		{
			name: "daily conflicts with same time",
			a:    model.Takeover{ScheduleType: model.ScheduleDaily, PlayTime: "20:00:00"},
			b:    model.Takeover{ScheduleType: model.ScheduleSpecifiedDate, StartDate: &day2, PlayTime: "20:00:00"},
			want: true,
		},
		{
			name: "date ranges do not overlap",
			a:    model.Takeover{ScheduleType: model.ScheduleDateRange, StartDate: &day1, EndDate: &day1, PlayTime: "20:00:00"},
			b:    model.Takeover{ScheduleType: model.ScheduleDateRange, StartDate: &day2, EndDate: &day3, PlayTime: "20:00:00"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := schedulesConflict(tt.a, tt.b); got != tt.want {
				t.Fatalf("schedulesConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUserProfileCompletedUsesStoredFields(t *testing.T) {
	gender := uint8(model.GenderFemale)
	nickname := "兔兔"
	steamID := "7656119"

	user := model.User{
		Nickname:           &nickname,
		SteamID:            &steamID,
		Gender:             &gender,
		IsProfileCompleted: false,
	}
	if !isUserProfileCompleted(user) {
		t.Fatal("expected profile completed from stored profile fields")
	}

	user.SteamID = nil
	user.IsProfileCompleted = true
	if !isUserProfileCompleted(user) {
		t.Fatal("expected profile completed without steam id")
	}
}

func TestToUserDTOWithPublishWhitelist(t *testing.T) {
	steamID := "7656119"
	user := model.User{OpenID: "openid-1", SteamID: &steamID}

	dto := toAdminWXUserDTOWithPublishWhitelist(user, map[string]bool{steamID: true})
	if dto.OpenID != "openid-1" {
		t.Fatalf("OpenID = %q, want openid-1", dto.OpenID)
	}
	if !dto.PublishWhitelisted {
		t.Fatal("expected user marked publish whitelisted")
	}

	emptySteamID := ""
	user.SteamID = &emptySteamID
	if toUserDTOWithPublishWhitelist(user, map[string]bool{"": true}).PublishWhitelisted {
		t.Fatal("expected empty steam id not marked publish whitelisted")
	}
	if !toAdminWXUserDTOWithPublishWhitelist(user, map[string]bool{"openid-1": true}).PublishWhitelisted {
		t.Fatal("expected openid whitelist to work without steam id")
	}
}
