package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestRunWechatSummaryDailyLoopStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := make(chan time.Time, 1)
	done := make(chan struct{})
	go func() {
		runWechatSummaryDailyLoop(ctx, time.Millisecond, func(now time.Time) { calls <- now })
		close(done)
	}()
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("worker did not tick")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWechatSummaryDailySchedulesNormalize(t *testing.T) {
	raw := `[{"enabled":true,"time":"23:00","dateMode":"yesterday","period":"evening","roomId":" r1 "},{"enabled":true,"time":"25:00","period":"day"}]`
	got := parseWechatSummaryDailySchedules(raw)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Time != "23:00" || got[0].DateMode != "yesterday" || got[0].Period != "evening" || got[0].RoomID != "r1" {
		t.Fatalf("unexpected schedule: %#v", got[0])
	}
}

func TestWechatSummaryDailyDateMode(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 5, 0, 0, time.Local)
	if got := wechatSummaryDailyDate(now, wechatSummaryDailySchedule{DateMode: "yesterday"}); got != "2026-07-15" {
		t.Fatalf("yesterday date = %s", got)
	}
	if got := wechatSummaryDailyDate(now, wechatSummaryDailySchedule{DateMode: "today"}); got != "2026-07-16" {
		t.Fatalf("today date = %s", got)
	}
}

func TestWechatSummaryDailyRunKeyRoundTrip(t *testing.T) {
	key := wechatSummaryDailyRunKey("2026-07-15", wechatSummaryDailySchedule{Time: "18:00", DateMode: "today", Period: "afternoon", RoomID: "r1"})
	raw := marshalWechatSummaryDailyRunKeys(map[string]bool{key: true})
	var decoded map[string]bool
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded[key] {
		t.Fatalf("run key %q was not preserved", key)
	}
}
