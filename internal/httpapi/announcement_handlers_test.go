package httpapi

import (
	"errors"
	"strings"
	"testing"
	"time"

	"steam-game-takeover-backend/internal/model"
)

func TestNormalizeAnnouncementInputRejectsInvalidTitle(t *testing.T) {
	_, err := normalizeAnnouncementInput(announcementInput{Content: "content", Status: 1}, time.Now())
	if !errors.Is(err, errAnnouncementTitleInvalid) {
		t.Fatalf("error = %v, want %v", err, errAnnouncementTitleInvalid)
	}
}

func TestNormalizeAnnouncementInputRejectsLongContent(t *testing.T) {
	_, err := normalizeAnnouncementInput(announcementInput{Title: "title", Content: strings.Repeat("字", 1001), Status: 1}, time.Now())
	if !errors.Is(err, errAnnouncementContentInvalid) {
		t.Fatalf("error = %v, want %v", err, errAnnouncementContentInvalid)
	}
}

func TestNormalizeAnnouncementInputRejectsEndBeforeStart(t *testing.T) {
	_, err := normalizeAnnouncementInput(announcementInput{
		Title:     "title",
		Content:   "content",
		Status:    1,
		StartTime: "2026-07-07 10:00:00",
		EndTime:   "2026-07-07 09:00:00",
	}, time.Now())
	if !errors.Is(err, errAnnouncementTimeInvalid) {
		t.Fatalf("error = %v, want %v", err, errAnnouncementTimeInvalid)
	}
}

func TestNormalizeAnnouncementStatusDefaultsToEnabled(t *testing.T) {
	status, err := normalizeAnnouncementStatus(0)
	if err != nil {
		t.Fatalf("normalizeAnnouncementStatus() error = %v", err)
	}
	if status != model.AnnouncementStatusEnabled {
		t.Fatalf("status = %d, want enabled", status)
	}
}
