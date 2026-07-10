package httpapi

import (
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestCleanSummaryNameTrimsNoisyTitle(t *testing.T) {
	got := cleanSummaryName("今晚 20:00 一起玩 幻兽帕鲁 来人")
	if got != "幻兽帕鲁" {
		t.Fatalf("cleanSummaryName() = %q", got)
	}
}

func TestApplyManualTakeoverSummaryMarksManual(t *testing.T) {
	name := "幻兽帕鲁"
	takeover := model.Takeover{Title: "今晚八点 幻兽帕鲁 缺人"}
	updates := map[string]interface{}{}

	if err := applyManualTakeoverSummary(updates, &takeover, &name); err != nil {
		t.Fatalf("applyManualTakeoverSummary() error = %v", err)
	}

	if updates["summary_source"] != summarySourceManual {
		t.Fatalf("summary_source = %v", updates["summary_source"])
	}
	if stringValue(takeover.SummaryName) != name {
		t.Fatalf("SummaryName = %q", stringValue(takeover.SummaryName))
	}
}
