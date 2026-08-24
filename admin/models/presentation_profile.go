package models

import (
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"gorm.io/gorm"
)

const (
	PresentationTransitionPublish  = "publish"
	PresentationTransitionRollback = "rollback"
)

// PresentationProfile is the mutable aggregate envelope. Portable profile
// JSON remains data-only and lives in DraftDocument until an explicit publish
// appends an immutable PresentationRevision.
type PresentationProfile struct {
	ID                  string    `json:"id" gorm:"column:id;type:varchar(32);primaryKey"`
	Scope               string    `json:"scope" gorm:"column:scope;type:varchar(16);not null;uniqueIndex:ux_presentation_profile_identity,priority:1"`
	SubjectID           string    `json:"subjectID,omitempty" gorm:"column:subject_id;type:varchar(160);not null;default:'';uniqueIndex:ux_presentation_profile_identity,priority:2"`
	PageKey             string    `json:"pageKey" gorm:"column:page_key;type:varchar(120);not null;uniqueIndex:ux_presentation_profile_identity,priority:3"`
	Version             int64     `json:"version" gorm:"column:version;type:bigint;not null;default:1;check:chk_presentation_profile_version,version > 0"`
	DraftDocument       string    `json:"-" gorm:"column:draft_document;type:text;not null"`
	DraftDigest         string    `json:"draftDigest,omitempty" gorm:"column:draft_digest;type:varchar(71);not null;default:''"`
	DraftDefinitionHash string    `json:"draftDefinitionHash,omitempty" gorm:"column:draft_definition_hash;type:varchar(71);not null;default:''"`
	PublishedRevision   int64     `json:"publishedRevision,omitempty" gorm:"column:published_revision;type:bigint;not null;default:0;check:chk_presentation_profile_published_revision,published_revision >= 0"`
	CreatedBy           string    `json:"createdBy" gorm:"column:created_by;type:varchar(64);not null"`
	UpdatedBy           string    `json:"updatedBy" gorm:"column:updated_by;type:varchar(64);not null"`
	CreatedAt           time.Time `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt           time.Time `json:"updatedAt" gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (*PresentationProfile) TableName() string {
	return "mss_boot_presentation_profiles"
}

func (profile *PresentationProfile) BeforeCreate(*gorm.DB) error {
	if profile.ID == "" {
		profile.ID = pkg.SimpleID()
	}
	if profile.Version == 0 {
		profile.Version = 1
	}
	return nil
}

func (profile *PresentationProfile) State() string {
	if profile != nil && profile.DraftDocument != "" {
		return "draft"
	}
	if profile != nil && profile.PublishedRevision > 0 {
		return "published"
	}
	return "invalid"
}

// PresentationRevision is append-only application history. The service has no
// update or delete operation for this table; rollback republishes the stored
// document as another row with a new profile-local Revision number.
type PresentationRevision struct {
	ID                 string    `json:"id" gorm:"column:id;type:varchar(32);primaryKey"`
	ProfileID          string    `json:"profileID" gorm:"column:profile_id;type:varchar(32);not null;index;uniqueIndex:ux_presentation_revision_number,priority:1;uniqueIndex:ux_presentation_revision_idempotency,priority:1"`
	Revision           int64     `json:"revision" gorm:"column:revision;type:bigint;not null;uniqueIndex:ux_presentation_revision_number,priority:2;check:chk_presentation_revision_number,revision > 0"`
	AggregateVersion   int64     `json:"aggregateVersion" gorm:"column:aggregate_version;type:bigint;not null;check:chk_presentation_revision_aggregate_version,aggregate_version > 0"`
	Document           string    `json:"-" gorm:"column:document;type:text;not null"`
	ContentDigest      string    `json:"contentDigest" gorm:"column:content_digest;type:varchar(71);not null"`
	DefinitionHash     string    `json:"definitionHash" gorm:"column:definition_hash;type:varchar(71);not null"`
	Transition         string    `json:"transition" gorm:"column:transition;type:varchar(16);not null"`
	SourceRevision     *int64    `json:"sourceRevision,omitempty" gorm:"column:source_revision;type:bigint"`
	ActorID            string    `json:"actorID" gorm:"column:actor_id;type:varchar(64);not null;index"`
	IdempotencyKeyHash string    `json:"-" gorm:"column:idempotency_key_hash;type:varchar(71);not null;uniqueIndex:ux_presentation_revision_idempotency,priority:2"`
	RequestHash        string    `json:"-" gorm:"column:request_hash;type:varchar(71);not null"`
	CreatedAt          time.Time `json:"createdAt" gorm:"column:created_at;not null;autoCreateTime"`
}

func (*PresentationRevision) TableName() string {
	return "mss_boot_presentation_revisions"
}

func (revision *PresentationRevision) BeforeCreate(*gorm.DB) error {
	if revision.ID == "" {
		revision.ID = pkg.SimpleID()
	}
	return nil
}
