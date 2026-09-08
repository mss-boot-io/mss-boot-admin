package litellmops

import (
	"time"

	"gorm.io/gorm"
)

// UserSnapshot is the local read-only projection of one LiteLLM user. It is
// refreshed only by SyncSnapshots and never edited through this Admin.
type UserSnapshot struct {
	ID             string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt      time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	UserID         string         `gorm:"column:user_id;type:varchar(64);not null" json:"user_id"`
	Email          string         `gorm:"column:email;type:varchar(254)" json:"email"`
	UserRole       string         `gorm:"column:user_role;type:varchar(64)" json:"user_role"`
	Models         string         `gorm:"column:models;type:text" json:"models"`
	MaxBudget      *float64       `gorm:"column:max_budget" json:"max_budget"`
	BudgetDuration *string        `gorm:"column:budget_duration;type:varchar(16)" json:"budget_duration"`
	BudgetResetAt  *time.Time     `gorm:"column:budget_reset_at" json:"budget_reset_at"`
	Spend          float64        `gorm:"column:spend" json:"spend"`
	SyncedAt       time.Time      `gorm:"column:synced_at" json:"synced_at"`
}

// TableName implements gorm.Tabler.
func (UserSnapshot) TableName() string { return "litellmops_user_snapshot" }

// KeySnapshot is the local read-only projection of one LiteLLM virtual key.
// It stores only the hash prefix — never the complete key material.
type KeySnapshot struct {
	ID                  string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt           time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	KeyHashPrefix       string         `gorm:"column:key_hash_prefix;type:varchar(32);not null" json:"key_hash_prefix"`
	Alias               *string        `gorm:"column:alias;type:varchar(128)" json:"alias"`
	UserID              string         `gorm:"column:user_id;type:varchar(64);not null" json:"user_id"`
	UserEmail           string         `gorm:"column:user_email;type:varchar(254)" json:"user_email"`
	MaxBudget           *float64       `gorm:"column:max_budget" json:"max_budget"`
	Spend               float64        `gorm:"column:spend" json:"spend"`
	TPMLimit            *int64         `gorm:"column:tpm_limit" json:"tpm_limit"`
	RPMLimit            *int64         `gorm:"column:rpm_limit" json:"rpm_limit"`
	MaxParallelRequests *int           `gorm:"column:max_parallel_requests" json:"max_parallel_requests"`
	Expires             *time.Time     `gorm:"column:expires" json:"expires"`
	IsSessionKey        bool           `gorm:"column:is_session_key;not null;default:false" json:"is_session_key"`
	SyncedAt            time.Time      `gorm:"column:synced_at" json:"synced_at"`
}

// TableName implements gorm.Tabler.
func (KeySnapshot) TableName() string { return "litellmops_key_snapshot" }
