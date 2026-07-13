package httpapi

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestExpiredDailyTakeoversUpdateUsesCreationCutoffAndActiveState(t *testing.T) {
	db := newTakeoverExpirationDryRunDB(t)
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.Local)
	sqlText := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return expiredDailyTakeoversUpdate(tx, now, 10)
	})

	for _, clause := range []string{
		"UPDATE `ttw_takeover` SET `takeover_state`=2",
		"schedule_type = 2",
		"is_deleted = false",
		"takeover_state = 1",
		"gmt_create <= '2026-07-03 15:00:00'",
	} {
		if !strings.Contains(sqlText, clause) {
			t.Fatalf("expiration SQL missing %q: %s", clause, sqlText)
		}
	}
	wantCutoff := now.AddDate(0, 0, -10)
	if !strings.Contains(sqlText, wantCutoff.Format("2006-01-02 15:04:05")) {
		t.Fatalf("expiration SQL does not contain cutoff %s: %s", wantCutoff, sqlText)
	}
}

func TestRunDailyTakeoverExpirationLoopRunsImmediatelyAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		runDailyTakeoverExpirationLoop(ctx, time.Hour, func() { calls <- struct{}{} })
		close(done)
	}()

	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("worker did not run immediately")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func newTakeoverExpirationDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := sql.Open("mysql", "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		t.Fatalf("open sql handle: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      conn,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}
	return db
}
