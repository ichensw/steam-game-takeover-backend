package httpapi

import (
	"fmt"
	"time"

	"steam-game-takeover-backend/internal/model"
)

type userDTO struct {
	ID               uint64 `json:"id"`
	Nickname         string `json:"nickname"`
	SteamID          string `json:"steamId"`
	Gender           *uint8 `json:"gender"`
	AvatarURL        string `json:"avatarUrl"`
	ProfileCompleted bool   `json:"profileCompleted"`
	Blocked          bool   `json:"blocked"`
}

type memberDTO struct {
	UserID    uint64 `json:"userId"`
	OpenID    string `json:"openid,omitempty"`
	Nickname  string `json:"nickname"`
	SteamID   string `json:"steamId"`
	Gender    *uint8 `json:"gender"`
	AvatarURL string `json:"avatarUrl"`
	JoinedAt  string `json:"joinedAt,omitempty"`
}

type takeoverDTO struct {
	ID               uint64      `json:"id"`
	Title            string      `json:"title"`
	ParticipantLimit uint        `json:"participantLimit"`
	JoinedCount      int64       `json:"joinedCount"`
	ScheduleType     uint8       `json:"scheduleType"`
	StartDate        *string     `json:"startDate"`
	EndDate          *string     `json:"endDate"`
	PlayTime         string      `json:"playTime"`
	ScheduleText     string      `json:"scheduleText"`
	Description      string      `json:"description"`
	HasJoined        bool        `json:"hasJoined"`
	PreviewMembers   []memberDTO `json:"previewMembers,omitempty"`
	Members          []memberDTO `json:"members,omitempty"`
}

type memberRow struct {
	UserID    uint64
	OpenID    string
	Nickname  *string
	SteamID   *string
	Gender    *uint8
	AvatarURL *string
	JoinedAt  time.Time
}

func toUserDTO(user model.User) userDTO {
	return userDTO{
		ID:               user.ID,
		Nickname:         stringValue(user.Nickname),
		SteamID:          stringValue(user.SteamID),
		Gender:           user.Gender,
		AvatarURL:        stringValue(user.AvatarURL),
		ProfileCompleted: user.IsProfileCompleted,
		Blocked:          user.IsBlocked,
	}
}

func toTakeoverDTO(t model.Takeover, joinedCount int64, hasJoined bool) takeoverDTO {
	return takeoverDTO{
		ID:               t.ID,
		Title:            t.Title,
		ParticipantLimit: t.ParticipantLimit,
		JoinedCount:      joinedCount,
		ScheduleType:     t.ScheduleType,
		StartDate:        dateString(t.StartDate),
		EndDate:          dateString(t.EndDate),
		PlayTime:         shortTime(t.PlayTime),
		ScheduleText:     scheduleText(t),
		Description:      stringValue(t.Description),
		HasJoined:        hasJoined,
	}
}

func toMemberDTO(row memberRow, includeOpenID bool) memberDTO {
	dto := memberDTO{
		UserID:    row.UserID,
		Nickname:  stringValue(row.Nickname),
		SteamID:   stringValue(row.SteamID),
		Gender:    row.Gender,
		AvatarURL: stringValue(row.AvatarURL),
		JoinedAt:  row.JoinedAt.Format("2006-01-02 15:04:05"),
	}
	if includeOpenID {
		dto.OpenID = row.OpenID
	}
	return dto
}

func dateString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}

func shortTime(value string) string {
	if len(value) >= 5 {
		return value[:5]
	}
	return value
}

func scheduleText(t model.Takeover) string {
	playTime := shortTime(t.PlayTime)
	switch t.ScheduleType {
	case model.ScheduleSpecifiedDate:
		if t.StartDate == nil {
			return playTime
		}
		return fmt.Sprintf("%s %s", friendlyDate(*t.StartDate), playTime)
	case model.ScheduleDaily:
		return fmt.Sprintf("Daily %s", playTime)
	case model.ScheduleDateRange:
		if t.StartDate == nil || t.EndDate == nil {
			return playTime
		}
		return fmt.Sprintf("%s to %s %s", friendlyDate(*t.StartDate), friendlyDate(*t.EndDate), playTime)
	default:
		return playTime
	}
}

func friendlyDate(value time.Time) string {
	today := truncateDate(time.Now())
	day := truncateDate(value)
	switch {
	case sameDate(day, today):
		return "Today"
	case sameDate(day, today.AddDate(0, 0, 1)):
		return "Tomorrow"
	default:
		return day.Format("01/02")
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
