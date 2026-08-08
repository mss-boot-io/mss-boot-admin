package models

import "time"

// ConfigRevision serializes mutations to a scoped configuration resource.
// The application scope uses an empty OwnerID; user-scoped resources use the
// authenticated user ID. Revisions are authoritative database state and must
// not depend on an optional cache.
type ConfigRevision struct {
	Scope     string    `json:"scope" gorm:"column:scope;type:varchar(16);primaryKey;not null"`
	OwnerID   string    `json:"ownerID" gorm:"column:owner_id;type:varchar(64);primaryKey;not null;default:''"`
	Resource  string    `json:"resource" gorm:"column:resource;type:varchar(32);primaryKey;not null"`
	Revision  int64     `json:"revision" gorm:"column:revision;type:bigint;not null;default:0"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (*ConfigRevision) TableName() string {
	return "mss_boot_config_revisions"
}
