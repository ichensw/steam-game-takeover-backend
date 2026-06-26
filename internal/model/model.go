package model

import "time"

const (
	GenderMale   = 1
	GenderFemale = 2

	ScheduleSpecifiedDate = 1
	ScheduleDaily         = 2
	ScheduleDateRange     = 3

	TakeoverStateNormal = 1
	TakeoverStateClosed = 2

	MemberStateJoined = 1
	MemberStateExited = 2

	ReportStatePending   = 1
	ReportStateIgnored   = 2
	ReportStatePenalized = 3

	ContentAuditStatusPass   = "pass"
	ContentAuditStatusReview = "review"
	ContentAuditStatusRisky  = "risky"
	ContentAuditStatusError  = "error"

	DefaultCreditScore   = 100
	MinJoinCreditScore   = 70
	MinCreateCreditScore = 51
)

type User struct {
	ID                 uint64     `gorm:"primaryKey;column:id"`
	OpenID             string     `gorm:"column:openid;size:64;uniqueIndex:uk_openid"`
	UnionID            *string    `gorm:"column:unionid;size:64"`
	Nickname           *string    `gorm:"column:nickname;size:32"`
	SteamID            *string    `gorm:"column:steam_id;size:64;index:idx_steam_id"`
	Gender             *uint8     `gorm:"column:gender"`
	AvatarURL          *string    `gorm:"column:avatar_url;size:255"`
	IsProfileCompleted bool       `gorm:"column:is_profile_completed"`
	IsAdmin            bool       `gorm:"column:is_admin"`
	IsBanned           bool       `gorm:"column:is_banned"`
	BanReason          *string    `gorm:"column:ban_reason;size:255"`
	BannedAt           *time.Time `gorm:"column:banned_at"`
	BannedByAdminID    *uint64    `gorm:"column:banned_by_admin_id"`
	IsDeleted          bool       `gorm:"column:is_deleted"`
	CreditScore        uint       `gorm:"column:credit_score;default:100"`
	LastLoginTime      *time.Time `gorm:"column:last_login_time"`
	GmtCreate          time.Time  `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified        time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (User) TableName() string { return "ttw_user" }

type AdminUser struct {
	ID            uint64     `gorm:"primaryKey;column:id"`
	Username      string     `gorm:"column:username;size:64;uniqueIndex:uk_username"`
	PasswordHash  string     `gorm:"column:password_hash;size:255"`
	Nickname      *string    `gorm:"column:nickname;size:64"`
	Enabled       bool       `gorm:"column:enabled"`
	LastLoginTime *time.Time `gorm:"column:last_login_time"`
	GmtCreate     time.Time  `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified   time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (AdminUser) TableName() string { return "ttw_admin_user" }

type AdminToken struct {
	ID          uint64    `gorm:"primaryKey;column:id"`
	AdminUserID uint64    `gorm:"column:admin_user_id;index:idx_admin_user_id"`
	TokenID     string    `gorm:"column:token_id;size:64;uniqueIndex:uk_token_id"`
	ExpiresAt   time.Time `gorm:"column:expires_at;index:idx_expires_at"`
	IsRevoked   bool      `gorm:"column:is_revoked;index:idx_is_revoked"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (AdminToken) TableName() string { return "ttw_admin_token" }

type Takeover struct {
	ID               uint64     `gorm:"primaryKey;column:id"`
	CreatorUserID    uint64     `gorm:"column:creator_user_id;index:idx_creator_user_id"`
	Title            string     `gorm:"column:title;size:50"`
	ParticipantLimit uint       `gorm:"column:participant_limit"`
	ScheduleType     uint8      `gorm:"column:schedule_type;index:idx_schedule"`
	StartDate        *time.Time `gorm:"column:start_date;type:date;index:idx_schedule"`
	EndDate          *time.Time `gorm:"column:end_date;type:date;index:idx_schedule"`
	PlayTime         string     `gorm:"column:play_time;type:time;index:idx_schedule"`
	Description      *string    `gorm:"column:description;size:500"`
	KookChannelID    *string    `gorm:"column:kook_channel_id;size:64"`
	KookChannelName  *string    `gorm:"column:kook_channel_name;size:128"`
	KookInviteURL    *string    `gorm:"column:kook_invite_url;size:255"`
	TakeoverState    uint8      `gorm:"column:takeover_state"`
	IsDeleted        bool       `gorm:"column:is_deleted"`
	GmtCreate        time.Time  `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
	GmtModified      time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (Takeover) TableName() string { return "ttw_takeover" }

type TakeoverMember struct {
	ID          uint64    `gorm:"primaryKey;column:id"`
	TakeoverID  uint64    `gorm:"column:takeover_id;uniqueIndex:uk_takeover_user"`
	UserID      uint64    `gorm:"column:user_id;uniqueIndex:uk_takeover_user;index:idx_user_id"`
	MemberState uint8     `gorm:"column:member_state"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (TakeoverMember) TableName() string { return "ttw_takeover_member" }

type TakeoverReport struct {
	ID               uint64     `gorm:"primaryKey;column:id"`
	TakeoverID       uint64     `gorm:"column:takeover_id;index:idx_takeover_id"`
	ReporterUserID   uint64     `gorm:"column:reporter_user_id;index:idx_reporter_user_id"`
	ReportedUserID   uint64     `gorm:"column:reported_user_id;index:idx_reported_user_id"`
	ReportContent    string     `gorm:"column:report_content;size:500"`
	ImageURLs        *string    `gorm:"column:image_urls;type:json"`
	PenaltyScore     uint       `gorm:"column:penalty_score"`
	HandleNote       *string    `gorm:"column:handle_note;size:500"`
	HandledByAdminID *uint64    `gorm:"column:handled_by_admin_id"`
	HandledAt        *time.Time `gorm:"column:handled_at"`
	ReportState      uint8      `gorm:"column:report_state"`
	GmtCreate        time.Time  `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
	GmtModified      time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (TakeoverReport) TableName() string { return "ttw_takeover_report" }

type AdminOperateLog struct {
	ID             uint64    `gorm:"primaryKey;column:id"`
	OperateType    string    `gorm:"column:operate_type;size:32"`
	TargetType     string    `gorm:"column:target_type;size:32;index:idx_target"`
	TargetID       uint64    `gorm:"column:target_id;index:idx_target"`
	OperateContent *string   `gorm:"column:operate_content;size:1000"`
	GmtCreate      time.Time `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
}

func (AdminOperateLog) TableName() string { return "ttw_admin_operate_log" }

type ContentAudit struct {
	ID          uint64    `gorm:"primaryKey;column:id"`
	UserID      uint64    `gorm:"column:user_id;index:idx_user_id"`
	OpenID      string    `gorm:"column:openid;size:64;index:idx_openid"`
	ContentType string    `gorm:"column:content_type;size:32;index:idx_content"`
	TargetID    uint64    `gorm:"column:target_id;index:idx_content"`
	Scene       uint8     `gorm:"column:scene"`
	Status      string    `gorm:"column:status;size:16;index:idx_status"`
	WXResult    *string   `gorm:"column:wx_result;type:json"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
}

func (ContentAudit) TableName() string { return "ttw_content_audit" }

type SensitiveWord struct {
	ID          uint64    `gorm:"primaryKey;column:id"`
	Word        string    `gorm:"column:word;size:128;uniqueIndex:uk_word"`
	Enabled     bool      `gorm:"column:enabled;index:idx_enabled"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (SensitiveWord) TableName() string { return "ttw_sensitive_word" }

type PublishTakeoverWhitelist struct {
	ID          uint64    `gorm:"primaryKey;column:id"`
	OpenID      *string   `gorm:"column:openid;size:64;uniqueIndex:uk_openid"`
	SteamID     *string   `gorm:"column:steam_id;size:64;uniqueIndex:uk_steam_id"`
	Enabled     bool      `gorm:"column:enabled;index:idx_enabled"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (PublishTakeoverWhitelist) TableName() string { return "ttw_publish_takeover_whitelist" }

const (
	AppConfigPublishTakeoverEnabled = "publish_takeover_enabled"
	AppConfigUAPIKey                = "uapi_key"
	AppConfigSteamWebAPIKey         = "steam_web_api_key"
	AppConfigKookBotToken           = "kook_bot_token"
	AppConfigKookGuildID            = "kook_guild_id"
)

type AppConfig struct {
	ConfigKey   string    `gorm:"primaryKey;column:config_key;size:64"`
	ConfigValue string    `gorm:"column:config_value;size:255"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (AppConfig) TableName() string { return "ttw_app_config" }
