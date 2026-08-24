package dto

import (
	"encoding/json"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
)

type PresentationProfileIdentity struct {
	Scope     presentation.ScopeKind `json:"scope" binding:"required"`
	SubjectID string                 `json:"subjectID,omitempty"`
	PageKey   string                 `json:"pageKey" binding:"required"`
}

type PresentationProfileCreateRequest struct {
	PresentationProfileIdentity
	Document json.RawMessage `json:"document" binding:"required"`
}

type PresentationDraftReplaceRequest struct {
	Document json.RawMessage `json:"document" binding:"required"`
}

type PresentationValidationRequest struct {
	Document json.RawMessage `json:"document" binding:"required"`
}

type PresentationRollbackRequest struct {
	Revision int64 `json:"revision" binding:"required"`
}

type PresentationProfileListRequest struct {
	Page     int                    `form:"page,default=1"`
	PageSize int                    `form:"pageSize,default=20"`
	Scope    presentation.ScopeKind `form:"scope"`
	PageKey  string                 `form:"pageKey"`
}

type PresentationProfileListResponse struct {
	Items    []PresentationProfileSummary `json:"items"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"pageSize"`
	Total    int64                        `json:"total"`
}

type PresentationCapabilityListResponse struct {
	Items        []presentation.CapabilityDefinition `json:"items"`
	RecoveryMode bool                                `json:"recoveryMode"`
}

type PresentationProfileSummary struct {
	ID                string                 `json:"id"`
	Scope             presentation.ScopeKind `json:"scope"`
	SubjectID         string                 `json:"subjectID,omitempty"`
	PageKey           string                 `json:"pageKey"`
	State             string                 `json:"state"`
	Version           int64                  `json:"version"`
	DraftValid        *bool                  `json:"draftValid,omitempty"`
	PublishedRevision int64                  `json:"publishedRevision,omitempty"`
	CreatedBy         string                 `json:"createdBy"`
	UpdatedBy         string                 `json:"updatedBy"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
}

type PresentationProfileResource struct {
	PresentationProfileSummary
	Draft     *PresentationDraftResource   `json:"draft,omitempty"`
	Published *PresentationRevisionSummary `json:"published,omitempty"`
}

type PresentationDraftResource struct {
	Document       json.RawMessage      `json:"document"`
	Digest         string               `json:"digest"`
	DefinitionHash string               `json:"definitionHash"`
	Valid          bool                 `json:"valid"`
	Issues         []presentation.Issue `json:"issues"`
}

type PresentationRevisionSummary struct {
	Revision         int64     `json:"revision"`
	AggregateVersion int64     `json:"aggregateVersion"`
	ContentDigest    string    `json:"contentDigest"`
	DefinitionHash   string    `json:"definitionHash"`
	Transition       string    `json:"transition"`
	SourceRevision   *int64    `json:"sourceRevision,omitempty"`
	ActorID          string    `json:"actorID"`
	CreatedAt        time.Time `json:"createdAt"`
}

type PresentationRevisionResource struct {
	PresentationRevisionSummary
	ProfileID string          `json:"profileID"`
	Document  json.RawMessage `json:"document"`
}

type PresentationRevisionListResponse struct {
	Items    []PresentationRevisionSummary `json:"items"`
	Page     int                           `json:"page"`
	PageSize int                           `json:"pageSize"`
	Total    int64                         `json:"total"`
}

type PresentationValidationResponse struct {
	StructurallyValid bool                 `json:"structurallyValid"`
	SemanticallyValid bool                 `json:"semanticallyValid"`
	CanonicalDocument json.RawMessage      `json:"canonicalDocument,omitempty"`
	Digest            string               `json:"digest,omitempty"`
	CurrentDefinition string               `json:"currentDefinition,omitempty"`
	Issues            []presentation.Issue `json:"issues"`
}

type PresentationTransitionResponse struct {
	Profile  *PresentationProfileResource  `json:"profile"`
	Revision *PresentationRevisionResource `json:"revision"`
	Replayed bool                          `json:"replayed"`
}

// PresentationConflictResource is deliberately limited to the opaque
// concurrency identity required to reconcile a stale mutation. Mutation
// permissions do not imply permission to read draft or history content.
type PresentationConflictResource struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

type EffectivePresentationResponse struct {
	PageKey      string                            `json:"pageKey"`
	RecoveryMode bool                              `json:"recoveryMode"`
	Fallback     bool                              `json:"fallback"`
	Layers       EffectivePresentationLayers       `json:"layers"`
	Diagnostics  []EffectivePresentationDiagnostic `json:"diagnostics"`
}

type EffectivePresentationLayers struct {
	Application json.RawMessage `json:"application,omitempty"`
	Role        json.RawMessage `json:"role,omitempty"`
	User        json.RawMessage `json:"user,omitempty"`
}

type EffectivePresentationDiagnostic struct {
	Layer     presentation.ScopeKind `json:"layer,omitempty"`
	ProfileID string                 `json:"profileID,omitempty"`
	Code      string                 `json:"code"`
	Issues    []presentation.Issue   `json:"issues,omitempty"`
}

type PresentationConflictResponseData struct {
	Current *PresentationConflictResource `json:"current"`
}

type PresentationConflictResponse struct {
	Success      bool                             `json:"success"`
	Status       string                           `json:"status"`
	Code         int                              `json:"code"`
	ErrorCode    string                           `json:"errorCode"`
	ErrorMessage string                           `json:"errorMessage"`
	TraceID      string                           `json:"traceID"`
	Data         PresentationConflictResponseData `json:"data"`
}
