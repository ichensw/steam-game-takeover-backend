package httpapi

import (
	"context"
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
