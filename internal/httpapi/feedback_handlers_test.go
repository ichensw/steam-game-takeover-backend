package httpapi

import (
	"errors"
	"strings"
	"testing"

	"steam-game-takeover-backend/internal/model"
)

func TestNormalizeUserFeedbackInputRejectsInvalidType(t *testing.T) {
	_, err := normalizeUserFeedbackInput(userFeedbackInput{FeedbackType: "bad", Content: "hello"})
	if !errors.Is(err, errFeedbackTypeInvalid) {
		t.Fatalf("error = %v, want %v", err, errFeedbackTypeInvalid)
	}
}

func TestNormalizeUserFeedbackInputRejectsEmptyContent(t *testing.T) {
	_, err := normalizeUserFeedbackInput(userFeedbackInput{FeedbackType: "suggestion", Content: " "})
	if !errors.Is(err, errFeedbackContentInvalid) {
		t.Fatalf("error = %v, want %v", err, errFeedbackContentInvalid)
	}
}

func TestNormalizeUserFeedbackInputRejectsLongContent(t *testing.T) {
	_, err := normalizeUserFeedbackInput(userFeedbackInput{FeedbackType: "suggestion", Content: strings.Repeat("字", 501)})
	if !errors.Is(err, errFeedbackContentInvalid) {
		t.Fatalf("error = %v, want %v", err, errFeedbackContentInvalid)
	}
}

func TestNormalizeUserFeedbackInputRejectsTooManyImages(t *testing.T) {
	_, err := normalizeUserFeedbackInput(userFeedbackInput{
		FeedbackType: "suggestion",
		Content:      "hello",
		Images:       []string{"https://e.com/1.png", "https://e.com/2.png", "https://e.com/3.png", "https://e.com/4.png"},
	})
	if !errors.Is(err, errFeedbackImagesInvalid) {
		t.Fatalf("error = %v, want %v", err, errFeedbackImagesInvalid)
	}
}

func TestNormalizeFeedbackStatusRejectsInvalid(t *testing.T) {
	if _, err := normalizeFeedbackStatus(9); !errors.Is(err, errFeedbackStatusInvalid) {
		t.Fatalf("error = %v, want %v", err, errFeedbackStatusInvalid)
	}
}

func TestAdminFeedbackFiltersSupportStatusAndKeyword(t *testing.T) {
	filters, err := normalizeAdminFeedbackFilters("2", " suggestion ", " 阿川 ")
	if err != nil {
		t.Fatalf("normalizeAdminFeedbackFilters() error = %v", err)
	}
	if filters.Status == nil || *filters.Status != model.FeedbackStatusAccepted {
		t.Fatalf("Status = %v, want accepted", filters.Status)
	}
	if filters.FeedbackType != "suggestion" {
		t.Fatalf("FeedbackType = %q, want suggestion", filters.FeedbackType)
	}
	if filters.Keyword != "阿川" {
		t.Fatalf("Keyword = %q, want 阿川", filters.Keyword)
	}
}
