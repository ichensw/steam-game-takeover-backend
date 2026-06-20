package httpapi

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"
)

var timePattern = regexp.MustCompile(`^\d{2}:\d{2}$`)

type takeoverInput struct {
	Title            string  `json:"title"`
	ParticipantLimit uint    `json:"participantLimit"`
	ScheduleType     uint8   `json:"scheduleType"`
	StartDate        *string `json:"startDate"`
	EndDate          *string `json:"endDate"`
	PlayTime         string  `json:"playTime"`
	Description      string  `json:"description"`
}

type parsedTakeoverInput struct {
	Title            string
	ParticipantLimit uint
	ScheduleType     uint8
	StartDate        *time.Time
	EndDate          *time.Time
	PlayTime         string
	Description      *string
}

func validateTakeoverInput(input takeoverInput, checkPast bool) (parsedTakeoverInput, error) {
	title := strings.TrimSpace(input.Title)
	description := strings.TrimSpace(input.Description)
	playTime := strings.TrimSpace(input.PlayTime)

	if title == "" || len([]rune(title)) > 50 {
		return parsedTakeoverInput{}, errors.New("title is required and must be at most 50 characters")
	}
	if input.ParticipantLimit < 2 || input.ParticipantLimit > 99 {
		return parsedTakeoverInput{}, errors.New("participantLimit must be between 2 and 99")
	}
	if input.ScheduleType < model.ScheduleSpecifiedDate || input.ScheduleType > model.ScheduleDateRange {
		return parsedTakeoverInput{}, errors.New("invalid scheduleType")
	}
	if !timePattern.MatchString(playTime) {
		return parsedTakeoverInput{}, errors.New("playTime must be HH:mm")
	}
	if _, err := time.Parse("15:04", playTime); err != nil {
		return parsedTakeoverInput{}, errors.New("invalid playTime")
	}
	if len([]rune(description)) > 500 {
		return parsedTakeoverInput{}, errors.New("description must be at most 500 characters")
	}

	startDate, err := parseOptionalDate(input.StartDate)
	if err != nil {
		return parsedTakeoverInput{}, err
	}
	endDate, err := parseOptionalDate(input.EndDate)
	if err != nil {
		return parsedTakeoverInput{}, err
	}

	today := truncateDate(time.Now())
	switch input.ScheduleType {
	case model.ScheduleSpecifiedDate:
		if startDate == nil {
			return parsedTakeoverInput{}, errors.New("startDate is required for specified date")
		}
		if endDate != nil && !sameDate(*startDate, *endDate) {
			return parsedTakeoverInput{}, errors.New("endDate must be empty or equal to startDate for specified date")
		}
		if checkPast && truncateDate(*startDate).Before(today) {
			return parsedTakeoverInput{}, errors.New("startDate cannot be before today")
		}
	case model.ScheduleDaily:
		startDate = nil
		endDate = nil
	case model.ScheduleDateRange:
		if startDate == nil || endDate == nil {
			return parsedTakeoverInput{}, errors.New("startDate and endDate are required for date range")
		}
		if endDate.Before(*startDate) {
			return parsedTakeoverInput{}, errors.New("endDate cannot be before startDate")
		}
		if checkPast && truncateDate(*endDate).Before(today) {
			return parsedTakeoverInput{}, errors.New("date range cannot end before today")
		}
	}

	return parsedTakeoverInput{
		Title:            title,
		ParticipantLimit: input.ParticipantLimit,
		ScheduleType:     input.ScheduleType,
		StartDate:        startDate,
		EndDate:          endDate,
		PlayTime:         playTime + ":00",
		Description:      stringPtr(description),
	}, nil
}

func parseOptionalDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(*value), time.Local)
	if err != nil {
		return nil, errors.New("date must be YYYY-MM-DD")
	}
	return &parsed, nil
}

func truncateDate(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, value.Location())
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
