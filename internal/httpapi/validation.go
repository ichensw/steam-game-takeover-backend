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
	KookChannelID    string  `json:"kookChannelId"`
	KookChannelName  string  `json:"kookChannelName"`
	SummaryName      *string `json:"summaryName"`
}

type parsedTakeoverInput struct {
	Title            string
	ParticipantLimit uint
	ScheduleType     uint8
	StartDate        *time.Time
	EndDate          *time.Time
	PlayTime         string
	Description      *string
	KookChannelID    *string
	KookChannelName  *string
	KookInviteURL    *string
}

func validateTakeoverInput(input takeoverInput, checkPast bool) (parsedTakeoverInput, error) {
	title := strings.TrimSpace(input.Title)
	description := strings.TrimSpace(input.Description)
	kookChannelID := strings.TrimSpace(input.KookChannelID)
	kookChannelName := strings.TrimSpace(input.KookChannelName)
	playTime := strings.TrimSpace(input.PlayTime)

	if title == "" || len([]rune(title)) > 30 {
		return parsedTakeoverInput{}, errors.New("请输入 30 个字以内的标题")
	}
	if input.ParticipantLimit < 2 || input.ParticipantLimit > 99 {
		return parsedTakeoverInput{}, errors.New("人数上限必须在 2-99 之间")
	}
	if input.ScheduleType < model.ScheduleSpecifiedDate || input.ScheduleType > model.ScheduleDateRange {
		return parsedTakeoverInput{}, errors.New("请选择正确的时间类型")
	}
	if !timePattern.MatchString(playTime) {
		return parsedTakeoverInput{}, errors.New("请选择固定时间")
	}
	parsedPlayTime, err := time.Parse("15:04", playTime)
	if err != nil {
		return parsedTakeoverInput{}, errors.New("固定时间格式不正确")
	}
	if len([]rune(description)) > 500 {
		return parsedTakeoverInput{}, errors.New("介绍不能超过 500 个字")
	}
	if len([]rune(kookChannelID)) > 64 || len([]rune(kookChannelName)) > 128 {
		return parsedTakeoverInput{}, errors.New("KOOK频道信息不正确")
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
			return parsedTakeoverInput{}, errors.New("请选择指定日期")
		}
		if endDate != nil && !sameDate(*startDate, *endDate) {
			return parsedTakeoverInput{}, errors.New("指定日期不需要填写结束日期")
		}
		if checkPast && truncateDate(*startDate).Before(today) {
			return parsedTakeoverInput{}, errors.New("不能选择今天之前的日期")
		}
		if checkPast && sameDate(*startDate, today) && !isPlayTimeAfterNow(parsedPlayTime) {
			return parsedTakeoverInput{}, errors.New("固定时间必须晚于当前时间")
		}
	case model.ScheduleDaily:
		startDate = nil
		endDate = nil
	case model.ScheduleDateRange:
		if startDate == nil || endDate == nil {
			return parsedTakeoverInput{}, errors.New("请选择日期范围")
		}
		if endDate.Before(*startDate) {
			return parsedTakeoverInput{}, errors.New("结束日期不能早于开始日期")
		}
		if checkPast && truncateDate(*endDate).Before(today) {
			return parsedTakeoverInput{}, errors.New("日期范围不能早于今天")
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
		KookChannelID:    optionalStringPtr(kookChannelID),
		KookChannelName:  optionalStringPtr(kookChannelName),
	}, nil
}

func optionalStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func parseOptionalDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(*value), time.Local)
	if err != nil {
		return nil, errors.New("日期格式不正确")
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

func isPlayTimeAfterNow(playTime time.Time) bool {
	now := time.Now()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), playTime.Hour(), playTime.Minute(), 0, 0, now.Location())
	return candidate.After(now)
}
