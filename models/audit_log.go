package models

import (
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

type AuditLogType string

const (
	AuditLogTypeLogin    AuditLogType = "login"
	AuditLogTypeLogout   AuditLogType = "logout"
	AuditLogTypeCreate   AuditLogType = "create"
	AuditLogTypeUpdate   AuditLogType = "update"
	AuditLogTypeDelete   AuditLogType = "delete"
	AuditLogTypeExport   AuditLogType = "export"
	AuditLogTypeImport   AuditLogType = "import"
	AuditLogTypeConfig   AuditLogType = "config"
	AuditLogTypeSecurity AuditLogType = "security"
)

type AuditLog struct {
	actions.ModelGorm
	UserID    string       `json:"userID" gorm:"column:user_id;type:varchar(36);index"`
	Username  string       `json:"username" gorm:"column:username;type:varchar(255)"`
	Type      AuditLogType `json:"type" gorm:"column:type;type:varchar(50);index"`
	Action    string       `json:"action" gorm:"column:action;type:varchar(255)"`
	Resource  string       `json:"resource" gorm:"column:resource;type:varchar(255)"`
	Method    string       `json:"method" gorm:"column:method;type:varchar(10)"`
	Path      string       `json:"path" gorm:"column:path;type:varchar(500)"`
	IP        string       `json:"ip" gorm:"column:ip;type:varchar(50)"`
	UserAgent string       `json:"userAgent" gorm:"column:user_agent;type:varchar(500)"`
	Status    enum.Status  `json:"status" gorm:"column:status;type:varchar(20)"`
	Message   string       `json:"message" gorm:"column:message;type:text"`
	Request   string       `json:"request,omitempty" gorm:"column:request;type:text"`
	Response  string       `json:"response,omitempty" gorm:"column:response;type:text"`
	Duration  int64        `json:"duration" gorm:"column:duration;type:bigint"`
	CreatedAt time.Time    `json:"createdAt" gorm:"column:created_at;type:timestamp;index"`
}

func (*AuditLog) TableName() string {
	return "mss_boot_audit_logs"
}

type LoginLog struct {
	actions.ModelGorm
	UserID    string      `json:"userID" gorm:"column:user_id;type:varchar(36);index"`
	Username  string      `json:"username" gorm:"column:username;type:varchar(255);index"`
	IP        string      `json:"ip" gorm:"column:ip;type:varchar(50)"`
	Location  string      `json:"location" gorm:"column:location;type:varchar(255)"`
	UserAgent string      `json:"userAgent" gorm:"column:user_agent;type:varchar(500)"`
	Status    enum.Status `json:"status" gorm:"column:status;type:varchar(20)"`
	Message   string      `json:"message" gorm:"column:message;type:varchar(500)"`
	LoginAt   time.Time   `json:"loginAt" gorm:"column:login_at;type:timestamp;index"`
	LogoutAt  *time.Time  `json:"logoutAt" gorm:"column:logout_at;type:timestamp"`
}

func (*LoginLog) TableName() string {
	return "mss_boot_login_logs"
}
