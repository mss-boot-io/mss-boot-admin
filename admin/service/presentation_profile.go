package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

const (
	PresentationListMaxPageSize    = 100
	PresentationHistoryMaxPageSize = 100
	PresentationIdempotencyMin     = 8
	PresentationIdempotencyMax     = 200
)

var (
	ErrPresentationNotFound            = errors.New("presentation profile not found")
	ErrPresentationIdentityConflict    = errors.New("presentation profile identity already exists")
	ErrPresentationIdentityMismatch    = errors.New("presentation document identity does not match the aggregate")
	ErrPresentationInvalidScope        = errors.New("presentation scope is invalid")
	ErrPresentationSubjectNotFound     = errors.New("presentation scope subject not found")
	ErrPresentationRevisionConflict    = errors.New("presentation profile revision conflict")
	ErrPresentationInvalidTransition   = errors.New("presentation transition is invalid for the current state")
	ErrPresentationInvalidDocument     = errors.New("presentation document is invalid")
	ErrPresentationIdempotencyKey      = errors.New("presentation idempotency key is invalid")
	ErrPresentationIdempotencyConflict = errors.New("presentation idempotency key was reused for another request")
	ErrPresentationRevisionNotFound    = errors.New("presentation revision not found")
)

type PresentationDocumentError struct {
	Issues []presentation.Issue
}

func (err *PresentationDocumentError) Error() string {
	return fmt.Sprintf("%s: %d issue(s)", ErrPresentationInvalidDocument, len(err.Issues))
}

func (err *PresentationDocumentError) Unwrap() error { return ErrPresentationInvalidDocument }

type PresentationRevisionConflictError struct {
	Expected int64
	Actual   int64
	Current  *dto.PresentationProfileResource
}

func (err *PresentationRevisionConflictError) Error() string {
	return fmt.Sprintf("%s: expected %d, current %d", ErrPresentationRevisionConflict, err.Expected, err.Actual)
}

func (err *PresentationRevisionConflictError) Unwrap() error {
	return ErrPresentationRevisionConflict
}

type PresentationProfileService struct {
	Database     *gorm.DB
	Registry     *presentation.Registry
	RecoveryMode func() bool
}

func (service *PresentationProfileService) registry() *presentation.Registry {
	if service != nil && service.Registry != nil {
		return service.Registry
	}
	return presentation.DefaultRegistry
}

func (service *PresentationProfileService) database(ctx *gin.Context) *gorm.DB {
	if service != nil && service.Database != nil {
		db := service.Database
		if ctx != nil && ctx.Request != nil {
			db = db.WithContext(ctx.Request.Context())
		}
		return db.Clauses(dbresolver.Write).Session(&gorm.Session{})
	}
	return center.GetDB(ctx, &models.PresentationProfile{}).
		Clauses(dbresolver.Write).
		Session(&gorm.Session{})
}

func (service *PresentationProfileService) recoveryMode() bool {
	if service != nil && service.RecoveryMode != nil {
		return service.RecoveryMode()
	}
	return config.Cfg.Presentation.RecoveryMode
}

func (service *PresentationProfileService) Capabilities() []presentation.CapabilityDefinition {
	return service.registry().List()
}

func (service *PresentationProfileService) RecoveryEnabled() bool {
	return service.recoveryMode()
}

func (service *PresentationProfileService) Validate(raw json.RawMessage) *dto.PresentationValidationResponse {
	document, structuralIssues := presentation.ParseDocument(raw)
	response := &dto.PresentationValidationResponse{
		Issues: structuralIssues,
	}
	if document == nil {
		return response
	}
	response.StructurallyValid = true
	response.CanonicalDocument = append(json.RawMessage(nil), document.Canonical...)
	response.Digest = document.Digest
	if capability, ok := service.registry().Lookup(document.Profile.Metadata.PageKey); ok {
		response.CurrentDefinition = capability.DefinitionHash
	}
	response.Issues = service.registry().Validate(document.Profile)
	response.SemanticallyValid = len(response.Issues) == 0
	return response
}

func (service *PresentationProfileService) CreateDraft(
	ctx *gin.Context,
	identity dto.PresentationProfileIdentity,
	raw json.RawMessage,
	actorID string,
) (*dto.PresentationProfileResource, error) {
	document, issues := presentation.ParseDocument(raw)
	if document == nil {
		return nil, &PresentationDocumentError{Issues: issues}
	}
	if err := validatePresentationIdentity(identity, document.Profile); err != nil {
		return nil, err
	}
	if strings.TrimSpace(actorID) == "" {
		return nil, errors.New("presentation actor is required")
	}
	db := service.database(ctx)
	if err := validatePresentationSubject(db, identity); err != nil {
		return nil, err
	}
	profile := &models.PresentationProfile{
		Scope:               string(identity.Scope),
		SubjectID:           identity.SubjectID,
		PageKey:             identity.PageKey,
		Version:             1,
		DraftDocument:       string(document.Canonical),
		DraftDigest:         document.Digest,
		DraftDefinitionHash: document.Profile.Metadata.DefinitionHash,
		CreatedBy:           actorID,
		UpdatedBy:           actorID,
	}
	if err := db.Create(profile).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrPresentationIdentityConflict
		}
		return nil, err
	}
	return service.projectProfile(db, profile)
}

func (service *PresentationProfileService) ReplaceDraft(
	ctx *gin.Context,
	profileID string,
	expectedVersion int64,
	raw json.RawMessage,
	actorID string,
) (*dto.PresentationProfileResource, error) {
	document, issues := presentation.ParseDocument(raw)
	if document == nil {
		return nil, &PresentationDocumentError{Issues: issues}
	}
	db := service.database(ctx)
	var result *dto.PresentationProfileResource
	err := db.Transaction(func(tx *gorm.DB) error {
		profile, err := service.lockProfile(tx, profileID)
		if err != nil {
			return err
		}
		if expectedVersion != profile.Version {
			return service.conflict(tx, profile, expectedVersion)
		}
		identity := identityFromModel(profile)
		if err = validatePresentationIdentity(identity, document.Profile); err != nil {
			return err
		}
		updates := map[string]any{
			"draft_document":        string(document.Canonical),
			"draft_digest":          document.Digest,
			"draft_definition_hash": document.Profile.Metadata.DefinitionHash,
			"version":               profile.Version + 1,
			"updated_by":            actorID,
			"updated_at":            time.Now(),
		}
		updated, updateErr := conditionalProfileUpdate(tx, profile.ID, profile.Version, updates)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			current, currentErr := service.loadProfile(tx, profile.ID)
			if currentErr != nil {
				return currentErr
			}
			return service.conflict(tx, current, expectedVersion)
		}
		profile.DraftDocument = string(document.Canonical)
		profile.DraftDigest = document.Digest
		profile.DraftDefinitionHash = document.Profile.Metadata.DefinitionHash
		profile.Version++
		profile.UpdatedBy = actorID
		result, err = service.projectProfile(tx, profile)
		return err
	})
	return result, err
}

func (service *PresentationProfileService) Publish(
	ctx *gin.Context,
	profileID string,
	expectedVersion int64,
	idempotencyKey string,
	actorID string,
) (*dto.PresentationTransitionResponse, error) {
	keyHash, err := presentationIdempotencyHash(idempotencyKey)
	if err != nil {
		return nil, err
	}
	requestHash := presentationRequestHash("publish", profileID, expectedVersion, 0)
	return service.transition(ctx, transitionInput{
		profileID: profileID, expectedVersion: expectedVersion, actorID: actorID,
		keyHash: keyHash, requestHash: requestHash, transition: models.PresentationTransitionPublish,
	})
}

func (service *PresentationProfileService) Rollback(
	ctx *gin.Context,
	profileID string,
	expectedVersion int64,
	sourceRevision int64,
	idempotencyKey string,
	actorID string,
) (*dto.PresentationTransitionResponse, error) {
	if sourceRevision <= 0 {
		return nil, ErrPresentationRevisionNotFound
	}
	keyHash, err := presentationIdempotencyHash(idempotencyKey)
	if err != nil {
		return nil, err
	}
	requestHash := presentationRequestHash("rollback", profileID, expectedVersion, sourceRevision)
	return service.transition(ctx, transitionInput{
		profileID: profileID, expectedVersion: expectedVersion, actorID: actorID,
		keyHash: keyHash, requestHash: requestHash, transition: models.PresentationTransitionRollback,
		sourceRevision: sourceRevision,
	})
}

type transitionInput struct {
	profileID       string
	expectedVersion int64
	actorID         string
	keyHash         string
	requestHash     string
	transition      string
	sourceRevision  int64
}

func (service *PresentationProfileService) transition(
	ctx *gin.Context,
	input transitionInput,
) (*dto.PresentationTransitionResponse, error) {
	if strings.TrimSpace(input.actorID) == "" {
		return nil, errors.New("presentation actor is required")
	}
	db := service.database(ctx)
	var transitionRevision *models.PresentationRevision
	replayed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		existing, existingErr := findPresentationIdempotencyRevision(tx, input.profileID, input.keyHash)
		if existingErr == nil {
			if existing.RequestHash != input.requestHash {
				return ErrPresentationIdempotencyConflict
			}
			transitionRevision = existing
			replayed = true
			return nil
		}
		if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}

		profile, lockErr := service.lockProfile(tx, input.profileID)
		if lockErr != nil {
			return lockErr
		}
		if input.expectedVersion != profile.Version {
			return service.conflict(tx, profile, input.expectedVersion)
		}

		var document *presentation.Document
		var source *models.PresentationRevision
		switch input.transition {
		case models.PresentationTransitionPublish:
			if profile.DraftDocument == "" {
				return ErrPresentationInvalidTransition
			}
			var issues []presentation.Issue
			document, issues = presentation.ParseDocument([]byte(profile.DraftDocument))
			if document == nil {
				return &PresentationDocumentError{Issues: issues}
			}
		case models.PresentationTransitionRollback:
			if profile.DraftDocument != "" || profile.PublishedRevision <= 0 {
				return ErrPresentationInvalidTransition
			}
			source = &models.PresentationRevision{}
			if findErr := tx.Model(&models.PresentationRevision{}).
				Where("profile_id = ? AND revision = ?", profile.ID, input.sourceRevision).
				Take(source).Error; findErr != nil {
				if errors.Is(findErr, gorm.ErrRecordNotFound) {
					return ErrPresentationRevisionNotFound
				}
				return findErr
			}
			var issues []presentation.Issue
			document, issues = presentation.ParseDocument([]byte(source.Document))
			if document == nil {
				return &PresentationDocumentError{Issues: issues}
			}
		default:
			return ErrPresentationInvalidTransition
		}
		if identityErr := validatePresentationIdentity(identityFromModel(profile), document.Profile); identityErr != nil {
			return identityErr
		}
		if issues := service.registry().Validate(document.Profile); len(issues) > 0 {
			return &PresentationDocumentError{Issues: issues}
		}

		nextRevision := profile.PublishedRevision + 1
		nextVersion := profile.Version + 1
		transitionRevision = &models.PresentationRevision{
			ProfileID:          profile.ID,
			Revision:           nextRevision,
			AggregateVersion:   nextVersion,
			Document:           string(document.Canonical),
			ContentDigest:      document.Digest,
			DefinitionHash:     document.Profile.Metadata.DefinitionHash,
			Transition:         input.transition,
			ActorID:            input.actorID,
			IdempotencyKeyHash: input.keyHash,
			RequestHash:        input.requestHash,
		}
		if source != nil {
			sourceRevision := source.Revision
			transitionRevision.SourceRevision = &sourceRevision
		}
		if createErr := tx.Create(transitionRevision).Error; createErr != nil {
			if isUniqueConstraintError(createErr) {
				return ErrPresentationIdempotencyConflict
			}
			return createErr
		}
		updated, updateErr := conditionalProfileUpdate(tx, profile.ID, profile.Version, map[string]any{
			"draft_document":        "",
			"draft_digest":          "",
			"draft_definition_hash": "",
			"published_revision":    nextRevision,
			"version":               nextVersion,
			"updated_by":            input.actorID,
			"updated_at":            time.Now(),
		})
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			current, currentErr := service.loadProfile(tx, profile.ID)
			if currentErr != nil {
				return currentErr
			}
			return service.conflict(tx, current, input.expectedVersion)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	profile, err := service.Get(ctx, input.profileID)
	if err != nil {
		return nil, err
	}
	return &dto.PresentationTransitionResponse{
		Profile: profile, Revision: projectPresentationRevision(transitionRevision), Replayed: replayed,
	}, nil
}

func (service *PresentationProfileService) Get(ctx *gin.Context, profileID string) (*dto.PresentationProfileResource, error) {
	db := service.database(ctx)
	profile, err := service.loadProfile(db, profileID)
	if err != nil {
		return nil, err
	}
	return service.projectProfile(db, profile)
}

func (service *PresentationProfileService) List(
	ctx *gin.Context,
	request dto.PresentationProfileListRequest,
) (*dto.PresentationProfileListResponse, error) {
	page, pageSize := normalizePresentationPage(request.Page, request.PageSize, PresentationListMaxPageSize)
	db := service.database(ctx).Model(&models.PresentationProfile{})
	if request.Scope != "" {
		db = db.Where("scope = ?", request.Scope)
	}
	if request.PageKey != "" {
		db = db.Where("page_key = ?", request.PageKey)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	profiles := make([]models.PresentationProfile, 0, pageSize)
	if err := db.Order("page_key ASC, scope ASC, subject_id ASC, id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&profiles).Error; err != nil {
		return nil, err
	}
	items := make([]dto.PresentationProfileSummary, 0, len(profiles))
	for index := range profiles {
		resource, err := service.projectProfile(db, &profiles[index])
		if err != nil {
			return nil, err
		}
		items = append(items, resource.PresentationProfileSummary)
	}
	return &dto.PresentationProfileListResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (service *PresentationProfileService) ListRevisions(
	ctx *gin.Context,
	profileID string,
	page int,
	pageSize int,
) (*dto.PresentationRevisionListResponse, error) {
	page, pageSize = normalizePresentationPage(page, pageSize, PresentationHistoryMaxPageSize)
	db := service.database(ctx)
	if _, err := service.loadProfile(db, profileID); err != nil {
		return nil, err
	}
	query := db.Model(&models.PresentationRevision{}).Where("profile_id = ?", profileID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	rows := make([]models.PresentationRevision, 0, pageSize)
	if err := query.Order("revision DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]dto.PresentationRevisionSummary, 0, len(rows))
	for index := range rows {
		items = append(items, projectPresentationRevision(&rows[index]).PresentationRevisionSummary)
	}
	return &dto.PresentationRevisionListResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (service *PresentationProfileService) GetRevision(
	ctx *gin.Context,
	profileID string,
	revision int64,
) (*dto.PresentationRevisionResource, error) {
	row := &models.PresentationRevision{}
	err := service.database(ctx).Model(&models.PresentationRevision{}).
		Where("profile_id = ? AND revision = ?", profileID, revision).
		Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPresentationRevisionNotFound
	}
	if err != nil {
		return nil, err
	}
	return projectPresentationRevision(row), nil
}

func (service *PresentationProfileService) Effective(
	ctx *gin.Context,
	pageKey string,
	userID string,
	roleID string,
) (*dto.EffectivePresentationResponse, error) {
	response := &dto.EffectivePresentationResponse{
		PageKey:     pageKey,
		Layers:      dto.EffectivePresentationLayers{},
		Diagnostics: []dto.EffectivePresentationDiagnostic{},
	}
	if service.recoveryMode() {
		response.RecoveryMode = true
		response.Fallback = true
		response.Diagnostics = append(response.Diagnostics, dto.EffectivePresentationDiagnostic{Code: "recovery-mode"})
		return response, nil
	}
	capability, registered := service.registry().Lookup(pageKey)
	if !registered {
		response.Fallback = true
		response.Diagnostics = append(response.Diagnostics, dto.EffectivePresentationDiagnostic{Code: "unknown-page"})
		return response, nil
	}
	db := service.database(ctx)
	layers := []struct {
		kind      presentation.ScopeKind
		subjectID string
		assign    func(json.RawMessage)
	}{
		{presentation.ScopeApplication, "", func(raw json.RawMessage) { response.Layers.Application = raw }},
		{presentation.ScopeRole, roleID, func(raw json.RawMessage) { response.Layers.Role = raw }},
		{presentation.ScopeUser, userID, func(raw json.RawMessage) { response.Layers.User = raw }},
	}
	for _, layer := range layers {
		if layer.kind != presentation.ScopeApplication && strings.TrimSpace(layer.subjectID) == "" {
			continue
		}
		raw, diagnostic, err := service.loadEffectiveLayer(db, capability, layer.kind, layer.subjectID)
		if err != nil {
			response.Fallback = true
			response.Layers = dto.EffectivePresentationLayers{}
			response.Diagnostics = []dto.EffectivePresentationDiagnostic{{Code: "presentation-store-unavailable"}}
			return response, err
		}
		if diagnostic != nil {
			response.Fallback = true
			response.Diagnostics = append(response.Diagnostics, *diagnostic)
			continue
		}
		if len(raw) > 0 {
			layer.assign(raw)
		}
	}
	return response, nil
}

func (service *PresentationProfileService) loadEffectiveLayer(
	db *gorm.DB,
	capability *presentation.CapabilityDefinition,
	scope presentation.ScopeKind,
	subjectID string,
) (json.RawMessage, *dto.EffectivePresentationDiagnostic, error) {
	profile := &models.PresentationProfile{}
	err := db.Where("scope = ? AND subject_id = ? AND page_key = ?", scope, subjectID, capability.PageKey).Take(profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if profile.PublishedRevision <= 0 {
		return nil, nil, nil
	}
	revision := &models.PresentationRevision{}
	if err = db.Model(&models.PresentationRevision{}).
		Where("profile_id = ? AND revision = ?", profile.ID, profile.PublishedRevision).
		Take(revision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &dto.EffectivePresentationDiagnostic{Layer: scope, ProfileID: profile.ID, Code: "published-revision-missing"}, nil
		}
		return nil, nil, err
	}
	document, issues := presentation.ParseDocument([]byte(revision.Document))
	if document == nil {
		return nil, &dto.EffectivePresentationDiagnostic{Layer: scope, ProfileID: profile.ID, Code: "corrupt-published-document", Issues: issues}, nil
	}
	if err = validatePresentationIdentity(identityFromModel(profile), document.Profile); err != nil {
		return nil, &dto.EffectivePresentationDiagnostic{Layer: scope, ProfileID: profile.ID, Code: "published-identity-mismatch"}, nil
	}
	issues = presentation.ValidateProfile(capability, document.Profile)
	if len(issues) > 0 {
		return nil, &dto.EffectivePresentationDiagnostic{Layer: scope, ProfileID: profile.ID, Code: "published-definition-drift", Issues: issues}, nil
	}
	return append(json.RawMessage(nil), document.Canonical...), nil, nil
}

func (service *PresentationProfileService) loadProfile(db *gorm.DB, profileID string) (*models.PresentationProfile, error) {
	profile := &models.PresentationProfile{}
	if err := db.Where("id = ?", profileID).Take(profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPresentationNotFound
		}
		return nil, err
	}
	return profile, nil
}

func (service *PresentationProfileService) lockProfile(db *gorm.DB, profileID string) (*models.PresentationProfile, error) {
	return service.loadProfile(db.Clauses(clause.Locking{Strength: "UPDATE"}), profileID)
}

func (service *PresentationProfileService) conflict(
	db *gorm.DB,
	profile *models.PresentationProfile,
	expected int64,
) error {
	current, err := service.projectProfile(db, profile)
	if err != nil {
		return err
	}
	return &PresentationRevisionConflictError{Expected: expected, Actual: profile.Version, Current: current}
}

func (service *PresentationProfileService) projectProfile(
	db *gorm.DB,
	profile *models.PresentationProfile,
) (*dto.PresentationProfileResource, error) {
	resource := &dto.PresentationProfileResource{
		PresentationProfileSummary: dto.PresentationProfileSummary{
			ID: profile.ID, Scope: presentation.ScopeKind(profile.Scope), SubjectID: profile.SubjectID,
			PageKey: profile.PageKey, State: profile.State(), Version: profile.Version,
			PublishedRevision: profile.PublishedRevision, CreatedBy: profile.CreatedBy, UpdatedBy: profile.UpdatedBy,
			CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt,
		},
	}
	if profile.DraftDocument != "" {
		document, structuralIssues := presentation.ParseDocument([]byte(profile.DraftDocument))
		issues := make([]presentation.Issue, 0, len(structuralIssues))
		issues = append(issues, structuralIssues...)
		var raw json.RawMessage
		if document != nil {
			raw = append(json.RawMessage(nil), document.Canonical...)
			issues = append(issues, service.registry().Validate(document.Profile)...)
		} else {
			raw = json.RawMessage(profile.DraftDocument)
		}
		valid := len(issues) == 0
		resource.DraftValid = &valid
		resource.Draft = &dto.PresentationDraftResource{
			Document: raw, Digest: profile.DraftDigest, DefinitionHash: profile.DraftDefinitionHash,
			Valid: valid, Issues: issues,
		}
	}
	if profile.PublishedRevision > 0 {
		revision := &models.PresentationRevision{}
		if err := db.Model(&models.PresentationRevision{}).
			Where("profile_id = ? AND revision = ?", profile.ID, profile.PublishedRevision).
			Take(revision).Error; err != nil {
			return nil, err
		}
		projected := projectPresentationRevision(revision)
		resource.Published = &projected.PresentationRevisionSummary
	}
	return resource, nil
}

func projectPresentationRevision(revision *models.PresentationRevision) *dto.PresentationRevisionResource {
	if revision == nil {
		return nil
	}
	return &dto.PresentationRevisionResource{
		PresentationRevisionSummary: dto.PresentationRevisionSummary{
			Revision: revision.Revision, AggregateVersion: revision.AggregateVersion,
			ContentDigest: revision.ContentDigest, DefinitionHash: revision.DefinitionHash,
			Transition: revision.Transition, SourceRevision: revision.SourceRevision,
			ActorID: revision.ActorID, CreatedAt: revision.CreatedAt,
		},
		ProfileID: revision.ProfileID,
		Document:  append(json.RawMessage(nil), []byte(revision.Document)...),
	}
}

func validatePresentationIdentity(identity dto.PresentationProfileIdentity, profile *presentation.Profile) error {
	if profile == nil || identity.PageKey != profile.Metadata.PageKey || identity.Scope != profile.Metadata.Scope.Kind {
		return ErrPresentationIdentityMismatch
	}
	switch identity.Scope {
	case presentation.ScopeApplication:
		if identity.SubjectID != "" || profile.Metadata.Scope.Subject != nil {
			return ErrPresentationIdentityMismatch
		}
	case presentation.ScopeRole, presentation.ScopeUser:
		if strings.TrimSpace(identity.SubjectID) == "" || profile.Metadata.Scope.Subject == nil || *profile.Metadata.Scope.Subject != identity.SubjectID {
			return ErrPresentationIdentityMismatch
		}
	default:
		return ErrPresentationInvalidScope
	}
	return nil
}

func validatePresentationSubject(db *gorm.DB, identity dto.PresentationProfileIdentity) error {
	switch identity.Scope {
	case presentation.ScopeApplication:
		if identity.SubjectID != "" {
			return ErrPresentationInvalidScope
		}
		return nil
	case presentation.ScopeRole:
		var count int64
		if err := db.Model(&models.Role{}).Where("id = ? AND status = ?", identity.SubjectID, enum.Enabled).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrPresentationSubjectNotFound
		}
		return nil
	case presentation.ScopeUser:
		var count int64
		if err := db.Model(&models.User{}).Where("id = ? AND status = ?", identity.SubjectID, enum.Enabled).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrPresentationSubjectNotFound
		}
		return nil
	default:
		return ErrPresentationInvalidScope
	}
}

func identityFromModel(profile *models.PresentationProfile) dto.PresentationProfileIdentity {
	return dto.PresentationProfileIdentity{
		Scope: presentation.ScopeKind(profile.Scope), SubjectID: profile.SubjectID, PageKey: profile.PageKey,
	}
}

func conditionalProfileUpdate(db *gorm.DB, profileID string, version int64, updates map[string]any) (bool, error) {
	result := db.Model(&models.PresentationProfile{}).Where("id = ? AND version = ?", profileID, version).Updates(updates)
	return result.RowsAffected == 1, result.Error
}

func findPresentationIdempotencyRevision(db *gorm.DB, profileID, keyHash string) (*models.PresentationRevision, error) {
	revision := &models.PresentationRevision{}
	err := db.Model(&models.PresentationRevision{}).
		Where("profile_id = ? AND idempotency_key_hash = ?", profileID, keyHash).
		Take(revision).Error
	return revision, err
}

func presentationIdempotencyHash(key string) (string, error) {
	if len(key) < PresentationIdempotencyMin || len(key) > PresentationIdempotencyMax || strings.TrimSpace(key) != key {
		return "", ErrPresentationIdempotencyKey
	}
	for _, current := range []byte(key) {
		if current < 0x21 || current > 0x7e {
			return "", ErrPresentationIdempotencyKey
		}
	}
	sum := sha256.Sum256([]byte(key))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func presentationRequestHash(transition, profileID string, expectedVersion, sourceRevision int64) string {
	value := fmt.Sprintf("%s\x00%s\x00%d\x00%d", transition, profileID, expectedVersion, sourceRevision)
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func normalizePresentationPage(page, pageSize, maximum int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > maximum {
		pageSize = maximum
	}
	return page, pageSize
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key")
}
