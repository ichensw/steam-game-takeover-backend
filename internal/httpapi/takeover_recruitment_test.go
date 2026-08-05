package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTakeoverRecruitmentStatus(t *testing.T) {
	now := time.Date(2026, 7, 30, 20, 0, 0, 0, time.Local)
	past := now.AddDate(0, 0, -1)
	description := "还差一位"

	tests := []struct {
		name   string
		found  bool
		row    model.Takeover
		joined int64
		status string
		label  string
		ready  bool
	}{
		{
			name:   "missing",
			found:  false,
			row:    model.Takeover{ID: 1},
			status: recruitmentStatusNotFound,
			label:  "接龙不存在",
		},
		{
			name:   "deleted",
			found:  true,
			row:    model.Takeover{ID: 2, Title: "已删除", IsDeleted: true},
			status: recruitmentStatusDeleted,
			label:  "接龙已删除",
		},
		{
			name:  "expired",
			found: true,
			row: model.Takeover{
				ID:            3,
				Title:         "昨天的车",
				ScheduleType:  model.ScheduleSpecifiedDate,
				StartDate:     &past,
				PlayTime:      "20:00",
				TakeoverState: model.TakeoverStateNormal,
			},
			status: recruitmentStatusExpired,
			label:  "接龙已过期",
		},
		{
			name:   "closed",
			found:  true,
			row:    model.Takeover{ID: 4, Title: "关车", ScheduleType: model.ScheduleDaily, TakeoverState: model.TakeoverStateClosed},
			status: recruitmentStatusClosed,
			label:  "已停止招募",
		},
		{
			name:   "full",
			found:  true,
			row:    model.Takeover{ID: 5, Title: "满员车", ScheduleType: model.ScheduleDaily, TakeoverState: model.TakeoverStateNormal, ParticipantLimit: 4},
			joined: 4,
			status: recruitmentStatusFull,
			label:  "已满员",
		},
		{
			name:  "recruiting",
			found: true,
			row: model.Takeover{
				ID:               6,
				Title:            "星露谷",
				Description:      &description,
				ScheduleType:     model.ScheduleDaily,
				TakeoverState:    model.TakeoverStateNormal,
				ParticipantLimit: 4,
			},
			joined: 3,
			status: recruitmentStatusRecruiting,
			label:  "招募中",
			ready:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := takeoverRecruitmentStatus(tt.row, tt.joined, tt.found, now)
			if got.Status != tt.status || got.StatusLabel != tt.label || got.CanRecruit != tt.ready {
				t.Fatalf("status = %#v", got)
			}
			if tt.found && !tt.row.IsDeleted && got.Title != tt.row.Title {
				t.Fatalf("title = %q, want %q", got.Title, tt.row.Title)
			}
		})
	}
}

func TestTakeoverRecruitmentCandidatePayload(t *testing.T) {
	description := "萌新农场，还差两位"
	row := takeoverListRow{
		Takeover: model.Takeover{
			ID:               123,
			Title:            "今晚星露谷联机",
			SummaryName:      stringPtr("星露谷物语"),
			Description:      &description,
			ScheduleType:     model.ScheduleDaily,
			PlayTime:         "20:00:00",
			TakeoverState:    model.TakeoverStateNormal,
			ParticipantLimit: 4,
		},
		JoinedCount: 2,
	}

	got := takeoverRecruitmentCandidate(row)
	if got.ID != 123 || got.Description != description || got.MissingCount != 2 || got.ParticipantLimit != 4 {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestTakeoverRecruitmentCandidatesAcceptTimeFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conn, err := sql.Open("mysql", "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		t.Fatalf("open sql handle: %v", err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      conn,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/takeovers/recruitment-candidates?timeFilter=tomorrow", nil)

	query, err := applyTimeFilter(db.Table("ttw_takeover"), c)
	if err != nil {
		t.Fatalf("apply time filter: %v", err)
	}
	sqlText := query.Find(&[]takeoverListRow{}).Statement.SQL.String()
	if !strings.Contains(sqlText, "schedule_type = ?") || !strings.Contains(sqlText, "start_date <= ?") || !strings.Contains(sqlText, "end_date >= ?") {
		t.Fatalf("tomorrow filter SQL missing date hit clauses: %s", sqlText)
	}
}

func TestTakeoverRecruitmentRoutesAreRegistered(t *testing.T) {
	want := map[string]bool{
		"GET /api/takeovers/recruitment-candidates":         true,
		"GET /api/takeovers/:takeoverId/recruitment-status": true,
	}
	for _, route := range NewRouter(&Handler{}).Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing routes: %#v", want)
	}
}
