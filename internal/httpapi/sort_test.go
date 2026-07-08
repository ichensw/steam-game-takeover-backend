package httpapi

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"steam-game-takeover-backend/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSortClause(t *testing.T) {
	allowed := map[string]string{"createdAt": "gmt_create", "nickname": "nickname"}
	if got := sortClause("nickname", "asc", allowed, "gmt_create"); got != "nickname ASC" {
		t.Fatalf("sortClause() = %q, want nickname ASC", got)
	}
	if got := sortClause("bad", "desc", allowed, "gmt_create"); got != "gmt_create DESC" {
		t.Fatalf("sortClause() = %q, want fallback DESC", got)
	}
}

func TestWXUserPublishWhitelistSortClause(t *testing.T) {
	got := wxUserPublishWhitelistSortClause("asc")
	if !strings.Contains(got, "ttw_publish_takeover_whitelist") {
		t.Fatalf("sort clause missing whitelist table: %q", got)
	}
	if !strings.HasSuffix(got, "ASC") {
		t.Fatalf("sort clause = %q, want ASC direction", got)
	}
}

func TestTakeoverSoonAcrossMidnight(t *testing.T) {
	now := time.Date(2026, 7, 7, 23, 30, 0, 0, time.Local)
	takeover := model.Takeover{ScheduleType: model.ScheduleDaily, PlayTime: "01:00:00"}
	if !isTakeoverSoon(takeover, now) {
		t.Fatal("daily 01:00 should be soon at 23:30")
	}
}

func TestApplyTakeoverRecommendOrderOnlyPartitionsFullThenLatest(t *testing.T) {
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
	var rows []takeoverListRow
	stmt := applyTakeoverRecommendOrder(db.Table("ttw_takeover"), time.Date(2026, 7, 7, 20, 30, 0, 0, time.Local)).
		Find(&rows).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "ORDER BY CASE WHEN participant_limit > 0 AND COALESCE(j.joined_count, 0) >= participant_limit THEN 1 ELSE 0 END ASC, ttw_takeover.id DESC") {
		t.Fatalf("missing custom order expression: %s", sql)
	}
	if strings.Contains(sql, "BETWEEN") || strings.Contains(sql, "play_time") || strings.Contains(sql, "hj.user_id IS NOT NULL THEN 0") {
		t.Fatalf("unexpected recommendation order expression: %s", sql)
	}
}
