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

	MemberActionJoin  = 1
	MemberActionLeave = 2

	ReminderSendPending = 1
	ReminderSendSent    = 2
	ReminderSendFailed  = 3

	ReportStatePending   = 1
	ReportStateIgnored   = 2
	ReportStatePenalized = 3

	FeedbackStatusPending  = 1
	FeedbackStatusAccepted = 2
	FeedbackStatusIgnored  = 3

	KookMemberStatusJoined = 1
	KookMemberStatusExited = 2

	KookVoiceSessionActive   = "active"
	KookVoiceSessionClosed   = "closed"
	KookVoiceSessionAbnormal = "abnormal"

	AnnouncementStatusEnabled  = 1
	AnnouncementStatusDisabled = 2

	ContentAuditStatusPass   = "pass"
	ContentAuditStatusReview = "review"
	ContentAuditStatusRisky  = "risky"
	ContentAuditStatusError  = "error"

	DefaultCreditScore   = 100
	MinJoinCreditScore   = 70
	MinCreateCreditScore = 51
)

const (
	ReportTypeNoShow     = "no_show"
	ReportTypeLeaveEarly = "leave_early"
	ReportTypeDisruptive = "disruptive"
	ReportTypeOffensive  = "offensive"
	ReportTypeOther      = "other"
)

const (
	AdminRoleSuperAdmin = "super_admin"
	AdminRoleKookAdmin  = "kook_admin"
	AdminRoleAdmin      = "admin"
)

const (
	KookChannelSortTriggerScheduled = "scheduled"
	KookChannelSortTriggerManual    = "manual"

	KookChannelSortStatusPlanning       = "planning"
	KookChannelSortStatusRunning        = "running"
	KookChannelSortStatusSucceeded      = "succeeded"
	KookChannelSortStatusFailed         = "failed"
	KookChannelSortStatusRollbackFailed = "rollback_failed"
)

type User struct {
	ID                  uint64     `gorm:"primaryKey;column:id"`
	OpenID              string     `gorm:"column:openid;size:64;uniqueIndex:uk_openid"`
	UnionID             *string    `gorm:"column:unionid;size:64"`
	Nickname            *string    `gorm:"column:nickname;size:32"`
	SteamID             *string    `gorm:"column:steam_id;size:64;index:idx_steam_id"`
	Gender              *uint8     `gorm:"column:gender"`
	AvatarURL           *string    `gorm:"column:avatar_url;size:255"`
	IsProfileCompleted  bool       `gorm:"column:is_profile_completed"`
	IsAdmin             bool       `gorm:"column:is_admin"`
	CanViewAllTakeovers bool       `gorm:"column:can_view_all_takeovers"`
	IsBanned            bool       `gorm:"column:is_banned"`
	BanReason           *string    `gorm:"column:ban_reason;size:255"`
	BannedAt            *time.Time `gorm:"column:banned_at"`
	BannedByAdminID     *uint64    `gorm:"column:banned_by_admin_id"`
	IsDeleted           bool       `gorm:"column:is_deleted"`
	CreditScore         uint       `gorm:"column:credit_score;default:100"`
	LastLoginTime       *time.Time `gorm:"column:last_login_time"`
	GmtCreate           time.Time  `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified         time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (User) TableName() string { return "ttw_user" }

type UserCreditLog struct {
	ID              uint64    `gorm:"primaryKey;column:id"`
	UserID          uint64    `gorm:"column:user_id;index:idx_user_id"`
	ScoreDelta      int       `gorm:"column:score_delta"`
	ScoreBefore     uint      `gorm:"column:score_before"`
	ScoreAfter      uint      `gorm:"column:score_after"`
	ReasonType      string    `gorm:"column:reason_type;size:32"`
	Reason          *string   `gorm:"column:reason;size:255"`
	OperatorAdminID *uint64   `gorm:"column:operator_admin_id"`
	RelatedReportID *uint64   `gorm:"column:related_report_id"`
	GmtCreate       time.Time `gorm:"column:gmt_create;autoCreateTime"`
}

func (UserCreditLog) TableName() string { return "ttw_user_credit_log" }

type KookMember struct {
	ID              uint64     `gorm:"primaryKey;column:id"`
	GuildID         string     `gorm:"column:guild_id;size:64;uniqueIndex:uk_guild_user"`
	KookUserID      string     `gorm:"column:kook_user_id;size:64;uniqueIndex:uk_guild_user;index:idx_kook_user_id"`
	Username        *string    `gorm:"column:username;size:64"`
	Nickname        *string    `gorm:"column:nickname;size:64"`
	IdentifyNum     *string    `gorm:"column:identify_num;size:16"`
	AvatarURL       *string    `gorm:"column:avatar_url;size:255"`
	IsBot           bool       `gorm:"column:is_bot"`
	RoleIDs         *string    `gorm:"column:role_ids;type:json"`
	MemberStatus    uint8      `gorm:"column:member_status;index:idx_member_status"`
	JoinedAt        *time.Time `gorm:"column:joined_at"`
	ExitedAt        *time.Time `gorm:"column:exited_at"`
	IsBlacklisted   bool       `gorm:"column:is_blacklisted;index:idx_is_blacklisted"`
	BlacklistReason *string    `gorm:"column:blacklist_reason;size:255"`
	BlacklistedAt   *time.Time `gorm:"column:blacklisted_at"`
	Remark          *string    `gorm:"column:remark;size:255"`
	GmtCreate       time.Time  `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
	GmtModified     time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (KookMember) TableName() string { return "ttw_kook_member" }

type KookVoiceSession struct {
	ID              uint64     `gorm:"primaryKey;column:id"`
	GuildID         string     `gorm:"column:guild_id;size:64;index:idx_guild_channel_joined"`
	ChannelID       string     `gorm:"column:channel_id;size:64;index:idx_guild_channel_joined"`
	KookUserID      string     `gorm:"column:kook_user_id;size:64;index:idx_user_joined"`
	JoinedAt        time.Time  `gorm:"column:joined_at;index:idx_guild_channel_joined;index:idx_user_joined"`
	ExitedAt        *time.Time `gorm:"column:exited_at;index:idx_exited_at"`
	DurationSeconds uint       `gorm:"column:duration_seconds"`
	Status          string     `gorm:"column:status;size:16;index:idx_status"`
	Source          string     `gorm:"column:source;size:16"`
	GmtCreate       time.Time  `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified     time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (KookVoiceSession) TableName() string { return "ttw_kook_voice_session" }

type KookChannelSortConfig struct {
	ID           uint8      `gorm:"primaryKey;column:id"`
	Enabled      bool       `gorm:"column:enabled"`
	GroupIDs     string     `gorm:"column:group_ids;type:json"`
	ScheduleType string     `gorm:"column:schedule_type;size:16"`
	Weekday      *int       `gorm:"column:weekday"`
	Monthday     *int       `gorm:"column:monthday"`
	Hour         int        `gorm:"column:hour"`
	NextRunAt    *time.Time `gorm:"column:next_run_at"`
	LockToken    *string    `gorm:"column:lock_token;size:64"`
	LockedUntil  *time.Time `gorm:"column:locked_until"`
	GmtCreate    time.Time  `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified  time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (KookChannelSortConfig) TableName() string { return "ttw_kook_channel_sort_config" }

type KookChannelSortRun struct {
	ID            uint64     `gorm:"primaryKey;column:id" json:"id"`
	Trigger       string     `gorm:"column:trigger;size:16" json:"trigger"`
	ExecutionKey  *string    `gorm:"column:execution_key;size:128;uniqueIndex:uk_execution_key" json:"-"`
	RangeStart    time.Time  `gorm:"column:range_start" json:"rangeStart"`
	RangeEnd      time.Time  `gorm:"column:range_end" json:"rangeEnd"`
	GroupSnapshot string     `gorm:"column:group_snapshot;type:longtext" json:"-"`
	PlanSnapshot  string     `gorm:"column:plan_snapshot;type:longtext" json:"-"`
	Status        string     `gorm:"column:status;size:32;index:idx_status" json:"status"`
	PlannedCount  int        `gorm:"column:planned_count" json:"plannedCount"`
	MovedCount    int        `gorm:"column:moved_count" json:"movedCount"`
	ErrorMessage  *string    `gorm:"column:error_message;type:text" json:"errorMessage"`
	StartedAt     time.Time  `gorm:"column:started_at;index:idx_started_at" json:"startedAt"`
	FinishedAt    *time.Time `gorm:"column:finished_at" json:"finishedAt"`
	GmtCreate     time.Time  `gorm:"column:gmt_create;autoCreateTime" json:"-"`
	GmtModified   time.Time  `gorm:"column:gmt_modified;autoUpdateTime" json:"-"`
}

func (KookChannelSortRun) TableName() string { return "ttw_kook_channel_sort_run" }

type AdminUser struct {
	ID            uint64     `gorm:"primaryKey;column:id"`
	Username      string     `gorm:"column:username;size:64;uniqueIndex:uk_username"`
	PasswordHash  string     `gorm:"column:password_hash;size:255"`
	Nickname      *string    `gorm:"column:nickname;size:64"`
	AvatarURL     *string    `gorm:"column:avatar_url;size:255"`
	Role          string     `gorm:"column:role;size:32"`
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

type AdminRoleMenu struct {
	Role        string    `gorm:"primaryKey;column:role;size:32"`
	MenuKeys    string    `gorm:"column:menu_keys;type:json"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (AdminRoleMenu) TableName() string { return "ttw_admin_role_menu" }

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
	SummaryName      *string    `gorm:"column:summary_name;size:64"`
	SummarySource    *string    `gorm:"column:summary_source;size:16"`
	SummaryTitleHash *string    `gorm:"column:summary_title_hash;size:64"`
	SummaryError     *string    `gorm:"column:summary_error;size:255"`
	SummaryUpdatedAt *time.Time `gorm:"column:summary_updated_at"`
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
	Remark      *string   `gorm:"column:remark;size:100"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (TakeoverMember) TableName() string { return "ttw_takeover_member" }

type TakeoverMemberActivity struct {
	ID         uint64    `gorm:"primaryKey;column:id"`
	TakeoverID uint64    `gorm:"column:takeover_id;index:idx_takeover_id"`
	UserID     uint64    `gorm:"column:user_id;index:idx_user_id"`
	Action     uint8     `gorm:"column:action"`
	Remark     *string   `gorm:"column:remark;size:100"`
	GmtCreate  time.Time `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
}

func (TakeoverMemberActivity) TableName() string { return "ttw_takeover_member_activity" }

type TakeoverReminderSubscription struct {
	ID          uint64     `gorm:"primaryKey;column:id"`
	TakeoverID  uint64     `gorm:"column:takeover_id;uniqueIndex:uk_takeover_user_play_at;index:idx_takeover_id"`
	UserID      uint64     `gorm:"column:user_id;uniqueIndex:uk_takeover_user_play_at;index:idx_user_id"`
	OpenID      string     `gorm:"column:openid;size:64"`
	RemindAt    time.Time  `gorm:"column:remind_at;index:idx_remind_state"`
	PlayAt      time.Time  `gorm:"column:play_at;uniqueIndex:uk_takeover_user_play_at"`
	SendState   uint8      `gorm:"column:send_state;index:idx_remind_state"`
	SendError   *string    `gorm:"column:send_error;size:255"`
	SentAt      *time.Time `gorm:"column:sent_at"`
	GmtCreate   time.Time  `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (TakeoverReminderSubscription) TableName() string {
	return "ttw_takeover_reminder_subscription"
}

type TakeoverReport struct {
	ID               uint64     `gorm:"primaryKey;column:id"`
	TakeoverID       uint64     `gorm:"column:takeover_id;index:idx_takeover_id"`
	ReporterUserID   uint64     `gorm:"column:reporter_user_id;index:idx_reporter_user_id"`
	ReportedUserID   uint64     `gorm:"column:reported_user_id;index:idx_reported_user_id"`
	ReportType       string     `gorm:"column:report_type;size:32"`
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

type UserFeedback struct {
	ID           uint64    `gorm:"primaryKey;column:id"`
	UserID       uint64    `gorm:"column:user_id;index:idx_user_id"`
	FeedbackType string    `gorm:"column:feedback_type;size:32"`
	Content      string    `gorm:"column:content;size:500"`
	Contact      string    `gorm:"column:contact;size:100"`
	Images       *string   `gorm:"column:images;type:json"`
	Status       uint8     `gorm:"column:status;index:idx_status_created_at"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime;index:idx_created_at;index:idx_status_created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserFeedback) TableName() string { return "ttw_user_feedback" }

type Announcement struct {
	ID          uint64     `gorm:"primaryKey;column:id"`
	Title       string     `gorm:"column:title;size:80"`
	Content     string     `gorm:"column:content;size:1000"`
	ImageURL    *string    `gorm:"column:image_url;size:255"`
	Status      uint8      `gorm:"column:status;index:idx_status_time"`
	StartTime   time.Time  `gorm:"column:start_time;index:idx_status_time"`
	EndTime     *time.Time `gorm:"column:end_time;index:idx_status_time"`
	GmtCreate   time.Time  `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
	GmtModified time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (Announcement) TableName() string { return "ttw_announcement" }

type UserAnnouncementRead struct {
	ID             uint64    `gorm:"primaryKey;column:id"`
	UserID         uint64    `gorm:"column:user_id;uniqueIndex:uk_user_announcement"`
	AnnouncementID uint64    `gorm:"column:announcement_id;uniqueIndex:uk_user_announcement;index:idx_announcement_id"`
	ReadAt         time.Time `gorm:"column:read_at;autoCreateTime"`
}

func (UserAnnouncementRead) TableName() string { return "ttw_user_announcement_read" }

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
	AppConfigPublishTakeoverEnabled      = "publish_takeover_enabled"
	AppConfigUAPIKey                     = "uapi_key"
	AppConfigSteamWebAPIKey              = "steam_web_api_key"
	AppConfigKookBotToken                = "kook_bot_token"
	AppConfigKookGuildID                 = "kook_guild_id"
	AppConfigKookVerifyToken             = "kook_verify_token"
	AppConfigKookEncryptKey              = "kook_encrypt_key"
	AppConfigAPIBaseURL                  = "api_base_url"
	AppConfigAIExtractEnabled            = "ai_extract_enabled"
	AppConfigAIExtractAPIKey             = "ai_extract_api_key"
	AppConfigAIExtractBaseURL            = "ai_extract_base_url"
	AppConfigAIExtractModel              = "ai_extract_model"
	AppConfigDailyTakeoverExpirationDays = "daily_takeover_expiration_days"
	AppConfigWechatSummaryMaxMessages    = "wechat_summary_max_messages"
	AppConfigWechatSummaryPrompt         = "wechat_summary_prompt"
	AppConfigWechatSummaryStyle          = "wechat_summary_style"
	AppConfigWechatSummaryModel          = "wechat_summary_model"
	AppConfigWechatSummaryCompareModels  = "wechat_summary_compare_models"
	AppConfigWechatSummaryAutoSend       = "wechat_summary_auto_send"
	AppConfigWechatSummaryAutoDaily      = "wechat_summary_auto_daily"
	AppConfigWechatSummaryDailyTime      = "wechat_summary_daily_time"
	AppConfigWechatSummaryDailyRoomID    = "wechat_summary_daily_room_id"
	AppConfigWechatSummaryLastRunDate    = "wechat_summary_last_run_date"
)

type AppConfig struct {
	ConfigKey   string    `gorm:"primaryKey;column:config_key;size:64"`
	ConfigValue string    `gorm:"column:config_value;type:longtext"`
	GmtCreate   time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (AppConfig) TableName() string { return "ttw_app_config" }
