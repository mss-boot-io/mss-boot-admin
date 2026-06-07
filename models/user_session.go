package models

import "time"

type SessionRevokeReason string

const (
	SessionRevokeLogout         SessionRevokeReason = "logout"
	SessionRevokeForceBySession SessionRevokeReason = "force-by-session"
	SessionRevokeForceByUser    SessionRevokeReason = "force-by-user"
)

type UserSession struct {
	ModelGormTenant
	UserID       string              `gorm:"type:varchar(64);index;comment:用户ID" json:"userID"`
	Username     string              `gorm:"type:varchar(255);comment:用户名" json:"username"`
	RoleID       string              `gorm:"type:varchar(64);comment:角色ID" json:"roleID"`
	LoginAt      time.Time           `gorm:"index;comment:登录时间" json:"loginAt"`
	LastSeenAt   time.Time           `gorm:"comment:最后活跃时间" json:"lastSeenAt"`
	ExpiredAt    time.Time           `gorm:"index;comment:过期时间" json:"expiredAt"`
	IP           string              `gorm:"type:varchar(50);comment:登录IP" json:"ip"`
	UserAgent    string              `gorm:"type:varchar(500);comment:User-Agent" json:"userAgent"`
	Revoked      bool                `gorm:"index;default:false;comment:是否已吊销" json:"revoked"`
	RevokedAt    *time.Time          `gorm:"comment:吊销时间" json:"revokedAt,omitempty"`
	RevokedBy    string              `gorm:"type:varchar(64);comment:吊销操作者" json:"revokedBy,omitempty"`
	RevokeReason SessionRevokeReason `gorm:"type:varchar(32);comment:吊销原因" json:"revokeReason,omitempty"`
}

func (*UserSession) TableName() string {
	return "mss_boot_user_sessions"
}
