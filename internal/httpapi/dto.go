package httpapi

import (
	"fmt"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"gorm.io/gorm"
)

type userDTO struct {
	ID                 uint64 `json:"id"`
	Nickname           string `json:"nickname"`
	SteamID            string `json:"steamId"`
	Gender             *uint8 `json:"gender"`
	AvatarURL          string `json:"avatarUrl"`
	ProfileCompleted   bool   `json:"profileCompleted"`
	IsAdmin            bool   `json:"isAdmin"`
	IsBanned           bool   `json:"isBanned"`
	BanReason          string `json:"banReason,omitempty"`
	BannedAt           string `json:"bannedAt,omitempty"`
	PublishWhitelisted bool   `json:"publishWhitelisted"`
	CreditScore        uint   `json:"creditScore"`
	CreditStatus       string `json:"creditStatus"`
}

type adminWXUserDTO struct {
	userDTO
	OpenID string `json:"openid"`
}

type adminUserDTO struct {
	ID            uint64 `json:"id"`
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	Enabled       bool   `json:"enabled"`
	LastLoginTime string `json:"lastLoginTime"`
	CreatedAt     string `json:"createdAt"`
}

type memberDTO struct {
	UserID       uint64 `json:"userId"`
	OpenID       string `json:"openid,omitempty"`
	Nickname     string `json:"nickname"`
	SteamID      string `json:"steamId"`
	Gender       *uint8 `json:"gender"`
	AvatarURL    string `json:"avatarUrl"`
	CreditScore  uint   `json:"creditScore"`
	CreditStatus string `json:"creditStatus"`
	JoinedAt     string `json:"joinedAt,omitempty"`
	IsSelf       bool   `json:"isSelf"`
	HasReported  bool   `json:"hasReported"`
}

type takeoverDTO struct {
	ID                  uint64      `json:"id"`
	CreatorUserID       uint64      `json:"creatorUserId"`
	CreatorName         string      `json:"creatorName"`
	CreatorCreditScore  uint        `json:"creatorCreditScore"`
	CreatorCreditStatus string      `json:"creatorCreditStatus"`
	Title               string      `json:"title"`
	ParticipantLimit    uint        `json:"participantLimit"`
	JoinedCount         int64       `json:"joinedCount"`
	ScheduleType        uint8       `json:"scheduleType"`
	StartDate           *string     `json:"startDate"`
	EndDate             *string     `json:"endDate"`
	PlayTime            string      `json:"playTime"`
	ScheduleText        string      `json:"scheduleText"`
	StatusLabel         string      `json:"statusLabel"`
	Description         string      `json:"description"`
	KookChannelID       string      `json:"kookChannelId"`
	KookChannelName     string      `json:"kookChannelName"`
	KookInviteURL       string      `json:"kookInviteUrl"`
	HasJoined           bool        `json:"hasJoined"`
	IsCreator           bool        `json:"isCreator"`
	CanManage           bool        `json:"canManage"`
	PreviewMembers      []memberDTO `json:"previewMembers,omitempty"`
	Members             []memberDTO `json:"members,omitempty"`
}

type memberRow struct {
	UserID      uint64
	OpenID      string
	Nickname    *string
	SteamID     *string
	Gender      *uint8
	AvatarURL   *string
	CreditScore uint
	JoinedAt    time.Time
}

func toUserDTO(user model.User) userDTO {
	return toUserDTOWithPublishWhitelist(user, nil)
}

func toAdminWXUserDTO(user model.User) adminWXUserDTO {
	return toAdminWXUserDTOWithPublishWhitelist(user, nil)
}

func toAdminWXUserDTOWithPublishWhitelist(user model.User, whitelist map[string]bool) adminWXUserDTO {
	return adminWXUserDTO{
		userDTO: toUserDTOWithPublishWhitelist(user, whitelist),
		OpenID:  user.OpenID,
	}
}

func toUserDTOWithPublishWhitelist(user model.User, whitelist map[string]bool) userDTO {
	steamID := stringValue(user.SteamID)
	openID := strings.TrimSpace(user.OpenID)
	return userDTO{
		ID:                 user.ID,
		Nickname:           stringValue(user.Nickname),
		SteamID:            steamID,
		Gender:             user.Gender,
		AvatarURL:          normalizeAvatarURL(stringValue(user.AvatarURL), user.Gender),
		ProfileCompleted:   isUserProfileCompleted(user),
		IsAdmin:            user.IsAdmin,
		IsBanned:           user.IsBanned,
		BanReason:          stringValue(user.BanReason),
		BannedAt:           timeString(user.BannedAt),
		PublishWhitelisted: (openID != "" && whitelist[openID]) || (steamID != "" && whitelist[steamID]),
		CreditScore:        user.CreditScore,
		CreditStatus:       creditStatus(user.CreditScore),
	}
}

func toAdminUserDTO(admin model.AdminUser) adminUserDTO {
	return adminUserDTO{
		ID:            admin.ID,
		Username:      admin.Username,
		Nickname:      stringValue(admin.Nickname),
		Enabled:       admin.Enabled,
		LastLoginTime: timeString(admin.LastLoginTime),
		CreatedAt:     admin.GmtCreate.Format("2006-01-02 15:04:05"),
	}
}

func toTakeoverDTO(t model.Takeover, joinedCount int64, hasJoined bool) takeoverDTO {
	return takeoverDTO{
		ID:               t.ID,
		CreatorUserID:    t.CreatorUserID,
		Title:            t.Title,
		ParticipantLimit: t.ParticipantLimit,
		JoinedCount:      joinedCount,
		ScheduleType:     t.ScheduleType,
		StartDate:        dateString(t.StartDate),
		EndDate:          dateString(t.EndDate),
		PlayTime:         shortTime(t.PlayTime),
		ScheduleText:     scheduleText(t),
		StatusLabel:      takeoverStatusLabel(t, joinedCount),
		Description:      stringValue(t.Description),
		KookChannelID:    stringValue(t.KookChannelID),
		KookChannelName:  stringValue(t.KookChannelName),
		KookInviteURL:    stringValue(t.KookInviteURL),
		HasJoined:        hasJoined,
	}
}

func toTakeoverDTOWithCreator(db *gorm.DB, t model.Takeover, joinedCount int64, hasJoined bool) takeoverDTO {
	dto := toTakeoverDTO(t, joinedCount, hasJoined)
	var creator model.User
	if err := db.Where("id = ? AND is_deleted = ?", t.CreatorUserID, false).First(&creator).Error; err == nil {
		dto.CreatorName = stringValue(creator.Nickname)
		dto.CreatorCreditScore = creator.CreditScore
		dto.CreatorCreditStatus = creditStatus(creator.CreditScore)
	}
	return dto
}

func toMemberDTO(row memberRow, includeOpenID bool) memberDTO {
	dto := memberDTO{
		UserID:       row.UserID,
		Nickname:     stringValue(row.Nickname),
		SteamID:      stringValue(row.SteamID),
		Gender:       row.Gender,
		AvatarURL:    normalizeAvatarURL(stringValue(row.AvatarURL), row.Gender),
		CreditScore:  row.CreditScore,
		CreditStatus: creditStatus(row.CreditScore),
		JoinedAt:     row.JoinedAt.Format("2006-01-02 15:04:05"),
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
		return fmt.Sprintf("每天 %s", playTime)
	case model.ScheduleDateRange:
		if t.StartDate == nil || t.EndDate == nil {
			return playTime
		}
		return fmt.Sprintf("%s 至 %s %s", friendlyDate(*t.StartDate), friendlyDate(*t.EndDate), playTime)
	default:
		return playTime
	}
}

func takeoverStatusLabel(t model.Takeover, joinedCount int64) string {
	if isTakeoverExpired(t) {
		return "已结束"
	}
	if t.ParticipantLimit > 0 && joinedCount >= int64(t.ParticipantLimit) {
		return "已满员"
	}
	return "招募中"
}

func isTakeoverExpired(t model.Takeover) bool {
	var endDate *time.Time
	switch t.ScheduleType {
	case model.ScheduleSpecifiedDate:
		endDate = t.StartDate
	case model.ScheduleDateRange:
		endDate = t.EndDate
	default:
		return false
	}
	if endDate == nil {
		return false
	}
	endAt, err := combineDateAndPlayTime(*endDate, t.PlayTime)
	if err != nil {
		return false
	}
	return time.Now().After(endAt)
}

func combineDateAndPlayTime(date time.Time, playTime string) (time.Time, error) {
	parsedTime, err := time.Parse("15:04", shortTime(playTime))
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		parsedTime.Hour(),
		parsedTime.Minute(),
		0,
		0,
		time.Local,
	), nil
}

func friendlyDate(value time.Time) string {
	today := truncateDate(time.Now())
	day := truncateDate(value)
	switch {
	case sameDate(day, today):
		return "今天"
	case sameDate(day, today.AddDate(0, 0, 1)):
		return "明天"
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

func hasUserProfileFields(user model.User) bool {
	if strings.TrimSpace(stringValue(user.Nickname)) == "" {
		return false
	}
	if user.Gender == nil {
		return false
	}
	return *user.Gender == model.GenderMale || *user.Gender == model.GenderFemale
}

func isUserProfileCompleted(user model.User) bool {
	return !user.IsDeleted && hasUserProfileFields(user)
}

func creditStatus(score uint) string {
	switch {
	case score <= 50:
		return "disabled"
	case score < model.MinJoinCreditScore:
		return "limited"
	default:
		return "normal"
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

const (
	defaultMaleAvatarURL   = "https://wechat-bot-images.oss-cn-hangzhou.aliyuncs.com/miniapp/default-avatar/avatar-male.jpg"
	defaultFemaleAvatarURL = "https://wechat-bot-images.oss-cn-hangzhou.aliyuncs.com/miniapp/default-avatar/avatar-female.jpg"
)

func normalizeAvatarURL(value string, gender *uint8) string {
	switch value {
	case "./assets/avatar-male.jpg", "/assets/avatar-male.jpg", "assets/avatar-male.jpg":
		return defaultMaleAvatarURL
	case "./assets/avatar-female.jpg", "/assets/avatar-female.jpg", "assets/avatar-female.jpg":
		return defaultFemaleAvatarURL
	}

	if value != "" {
		return value
	}
	if gender != nil && *gender == model.GenderFemale {
		return defaultFemaleAvatarURL
	}
	return defaultMaleAvatarURL
}

func normalizeAvatarURLForGender(value string, gender uint8) string {
	return normalizeAvatarURL(value, &gender)
}
