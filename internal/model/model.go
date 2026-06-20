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
	IsBlocked          bool       `gorm:"column:is_blocked"`
	LastLoginTime      *time.Time `gorm:"column:last_login_time"`
	GmtCreate          time.Time  `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified        time.Time  `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (User) TableName() string { return "ttw_user" }

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

type BlockUser struct {
	ID           uint64    `gorm:"primaryKey;column:id"`
	UserID       uint64    `gorm:"column:user_id;uniqueIndex:uk_user_id"`
	OpenID       string    `gorm:"column:openid;size:64;index:idx_openid"`
	NicknameSnap *string   `gorm:"column:nickname_snapshot;size:32"`
	SteamIDSnap  *string   `gorm:"column:steam_id_snapshot;size:64"`
	BlockReason  *string   `gorm:"column:block_reason;size:255"`
	IsDeleted    bool      `gorm:"column:is_deleted"`
	GmtCreate    time.Time `gorm:"column:gmt_create;autoCreateTime"`
	GmtModified  time.Time `gorm:"column:gmt_modified;autoUpdateTime"`
}

func (BlockUser) TableName() string { return "ttw_block_user" }

type AdminOperateLog struct {
	ID             uint64    `gorm:"primaryKey;column:id"`
	OperateType    string    `gorm:"column:operate_type;size:32"`
	TargetType     string    `gorm:"column:target_type;size:32;index:idx_target"`
	TargetID       uint64    `gorm:"column:target_id;index:idx_target"`
	OperateContent *string   `gorm:"column:operate_content;size:1000"`
	GmtCreate      time.Time `gorm:"column:gmt_create;autoCreateTime;index:idx_gmt_create"`
}

func (AdminOperateLog) TableName() string { return "ttw_admin_operate_log" }
