package service

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPresentationEffectiveLayersUseServerSubjectsAndNeverExposeDrafts(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	seedPresentationSubject(t, db, presentation.ScopeRole, "role-a")
	seedPresentationSubject(t, db, presentation.ScopeUser, "user-a")

	inputs := []struct {
		scope     presentation.ScopeKind
		subjectID string
		pageSize  int
		key       string
	}{
		{presentation.ScopeApplication, "", 20, "publish-app-1"},
		{presentation.ScopeRole, "role-a", 30, "publish-role-1"},
		{presentation.ScopeUser, "user-a", 40, "publish-user-1"},
	}
	resources := make([]*dto.PresentationProfileResource, 0, len(inputs))
	for _, input := range inputs {
		subject := input.subjectID
		scope := presentation.Scope{Kind: input.scope}
		if input.scope != presentation.ScopeApplication {
			scope.Subject = &subject
		}
		raw := presentationProfileJSON(t, capability, scope, func(profile *presentation.Profile) {
			pageSize := input.pageSize
			profile.Spec.List = &presentation.ListPatch{PageSize: &pageSize}
		})
		created, err := service.CreateDraft(ctx, dto.PresentationProfileIdentity{
			Scope: input.scope, SubjectID: input.subjectID, PageKey: capability.PageKey,
		}, raw, "author")
		require.NoError(t, err)
		published, err := service.Publish(ctx, created.ID, created.Version, input.key, "publisher")
		require.NoError(t, err)
		resources = append(resources, published.Profile)
	}

	newPageSize := 99
	userSubject := "user-a"
	userDraft := presentationProfileJSON(
		t,
		capability,
		presentation.Scope{Kind: presentation.ScopeUser, Subject: &userSubject},
		func(profile *presentation.Profile) {
			profile.Spec.List = &presentation.ListPatch{PageSize: &newPageSize}
		},
	)
	_, err := service.ReplaceDraft(ctx, resources[2].ID, resources[2].Version, userDraft, "author")
	require.NoError(t, err)

	effective, err := service.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.NoError(t, err)
	require.False(t, effective.Fallback)
	require.Equal(t, 20, effectivePageSize(t, effective.Layers.Application))
	require.Equal(t, 30, effectivePageSize(t, effective.Layers.Role))
	require.Equal(t, 40, effectivePageSize(t, effective.Layers.User))

	other, err := service.Effective(ctx, capability.PageKey, "user-b", "role-b")
	require.NoError(t, err)
	require.NotEmpty(t, other.Layers.Application)
	require.Empty(t, other.Layers.Role)
	require.Empty(t, other.Layers.User)

	recoveryPolicy := presentation.MustNewAdoptionPolicy(
		presentation.AdoptionActive, []string{capability.PageKey}, true, service.Registry,
	)
	recovery, err := NewPresentationProfileService(service.Registry, recoveryPolicy)
	require.NoError(t, err)
	recovery.Database = db
	recovered, err := recovery.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.NoError(t, err)
	require.True(t, recovered.RecoveryMode)
	require.True(t, recovered.Fallback)
	require.Empty(t, recovered.Layers.Application)
}

func TestPresentationEffectiveAdoptionModesFailClosed(t *testing.T) {
	activeService, db, ctx, capability := newPresentationService(t)
	raw := presentationProfileJSON(
		t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil,
	)
	created, err := activeService.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: presentation.ScopeApplication, PageKey: capability.PageKey,
	}, raw, "author")
	require.NoError(t, err)
	_, err = activeService.Publish(ctx, created.ID, created.Version, "publish-adoption-1", "publisher")
	require.NoError(t, err)

	tests := []struct {
		name           string
		mode           presentation.AdoptionMode
		activePages    []string
		wantState      presentation.AdoptionState
		wantDiagnostic string
		wantResolved   bool
	}{
		{
			name: "disabled", mode: presentation.AdoptionDisabled,
			wantState: presentation.AdoptionStateDisabled, wantDiagnostic: "adoption-disabled",
		},
		{
			name: "shadow", mode: presentation.AdoptionShadow,
			wantState: presentation.AdoptionStateShadow, wantDiagnostic: "adoption-shadow", wantResolved: true,
		},
		{
			name: "active page not allowlisted", mode: presentation.AdoptionActive,
			wantState: presentation.AdoptionStateNotAllowlisted, wantDiagnostic: "adoption-not-allowlisted",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := presentation.MustNewAdoptionPolicy(
				test.mode, test.activePages, false, activeService.Registry,
			)
			current, serviceErr := NewPresentationProfileService(activeService.Registry, policy)
			require.NoError(t, serviceErr)
			current.Database = db
			effective, effectiveErr := current.Effective(ctx, capability.PageKey, "", "")
			require.NoError(t, effectiveErr)
			require.True(t, effective.Fallback)
			if test.wantResolved {
				require.NotEmpty(t, effective.Layers.Application)
			} else {
				require.Empty(t, effective.Layers.Application)
			}
			require.Equal(t, test.wantState, effective.Adoption.State)
			require.Equal(t, test.wantResolved, effective.Adoption.ResolveLayers)
			require.False(t, effective.Adoption.ApplyLayers)
			require.Equal(t, test.wantDiagnostic, effective.Diagnostics[0].Code)
		})
	}
}

func TestPresentationEffectiveActiveAllowlistedWithoutProfilesUsesCompiledDefaults(t *testing.T) {
	service, _, ctx, capability := newPresentationService(t)

	effective, err := service.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.NoError(t, err)
	require.False(t, effective.Fallback)
	require.False(t, effective.RecoveryMode)
	require.Equal(t, capability.DefinitionHash, effective.DefinitionHash)
	require.Equal(t, presentation.AdoptionActive, effective.Adoption.Mode)
	require.Equal(t, presentation.AdoptionStateActive, effective.Adoption.State)
	require.True(t, effective.Adoption.Allowlisted)
	require.True(t, effective.Adoption.ResolveLayers)
	require.True(t, effective.Adoption.ApplyLayers)
	require.Empty(t, effective.Layers.Application)
	require.Empty(t, effective.Layers.Role)
	require.Empty(t, effective.Layers.User)
	require.Empty(t, effective.Diagnostics)
}

func TestPresentationEffectiveStaleApplicationPreservesValidRoleAndUserLayers(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	seedPresentationSubject(t, db, presentation.ScopeRole, "role-a")
	seedPresentationSubject(t, db, presentation.ScopeUser, "user-a")

	application := publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeApplication, "", 20, "publish-stale-app",
	)
	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeRole, "role-a", 30, "publish-valid-role",
	)
	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeUser, "user-a", 40, "publish-valid-user",
	)

	const staleDefinitionHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	staleDocument := presentationProfileJSON(
		t,
		capability,
		presentation.Scope{Kind: presentation.ScopeApplication},
		func(profile *presentation.Profile) {
			profile.Metadata.DefinitionHash = staleDefinitionHash
			pageSize := 20
			profile.Spec.List = &presentation.ListPatch{PageSize: &pageSize}
		},
	)
	parsedStaleDocument, issues := presentation.ParseDocument(staleDocument)
	require.NotNil(t, parsedStaleDocument)
	require.Empty(t, issues)
	require.NoError(t, db.Model(&models.PresentationRevision{}).
		Where("profile_id = ? AND revision = ?", application.Profile.ID, application.Profile.PublishedRevision).
		Updates(map[string]any{
			"document":        string(parsedStaleDocument.Canonical),
			"content_digest":  parsedStaleDocument.Digest,
			"definition_hash": staleDefinitionHash,
		}).Error)

	effective, err := service.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.NoError(t, err)
	require.True(t, effective.Fallback)
	require.Empty(t, effective.Layers.Application)
	require.Equal(t, 30, effectivePageSize(t, effective.Layers.Role))
	require.Equal(t, 40, effectivePageSize(t, effective.Layers.User))
	require.Len(t, effective.Diagnostics, 1)
	require.Equal(t, presentation.ScopeApplication, effective.Diagnostics[0].Layer)
	require.Equal(t, application.Profile.ID, effective.Diagnostics[0].ProfileID)
	require.Equal(t, "published-definition-drift", effective.Diagnostics[0].Code)
	require.Equal(t, capability.DefinitionHash, effective.Diagnostics[0].ExpectedDefinitionHash)
	require.Equal(t, staleDefinitionHash, effective.Diagnostics[0].ObservedDefinitionHash)
}

func TestPresentationEffectiveCorruptRolePreservesValidApplicationAndUserLayers(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	seedPresentationSubject(t, db, presentation.ScopeRole, "role-a")
	seedPresentationSubject(t, db, presentation.ScopeUser, "user-a")

	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeApplication, "", 20, "publish-valid-app",
	)
	role := publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeRole, "role-a", 30, "publish-corrupt-role",
	)
	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeUser, "user-a", 40, "publish-valid-user",
	)
	require.NoError(t, db.Model(&models.PresentationRevision{}).
		Where("profile_id = ? AND revision = ?", role.Profile.ID, role.Profile.PublishedRevision).
		Update("document", "{").Error)

	effective, err := service.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.NoError(t, err)
	require.True(t, effective.Fallback)
	require.Equal(t, 20, effectivePageSize(t, effective.Layers.Application))
	require.Empty(t, effective.Layers.Role)
	require.Equal(t, 40, effectivePageSize(t, effective.Layers.User))
	require.Len(t, effective.Diagnostics, 1)
	require.Equal(t, presentation.ScopeRole, effective.Diagnostics[0].Layer)
	require.Equal(t, role.Profile.ID, effective.Diagnostics[0].ProfileID)
	require.Equal(t, "corrupt-published-document", effective.Diagnostics[0].Code)
	require.NotEmpty(t, effective.Diagnostics[0].Issues)
}

func TestPresentationEffectiveStoreFailureAfterApplicationClearsEveryLayer(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	seedPresentationSubject(t, db, presentation.ScopeRole, "role-a")
	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeApplication, "", 20, "publish-app-before-failure",
	)
	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeRole, "role-a", 30, "publish-role-before-failure",
	)

	storeErr := errors.New("injected presentation store failure")
	profileQueries := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(
		"test:fail-presentation-store-after-application",
		func(query *gorm.DB) {
			if query.Statement.Schema == nil || query.Statement.Schema.Name != "PresentationProfile" {
				return
			}
			profileQueries++
			if profileQueries == 2 {
				query.AddError(storeErr)
			}
		},
	))

	effective, err := service.Effective(ctx, capability.PageKey, "", "role-a")
	require.ErrorIs(t, err, storeErr)
	require.GreaterOrEqual(t, profileQueries, 2)
	require.True(t, effective.Fallback)
	require.Empty(t, effective.Layers.Application)
	require.Empty(t, effective.Layers.Role)
	require.Empty(t, effective.Layers.User)
	require.Equal(t, []dto.EffectivePresentationDiagnostic{{Code: "presentation-store-unavailable"}}, effective.Diagnostics)
}

func TestPresentationEffectiveRecoveryKeepsRuntimeLayersEmptyAndManagementReadable(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	seedPresentationSubject(t, db, presentation.ScopeRole, "role-a")
	seedPresentationSubject(t, db, presentation.ScopeUser, "user-a")
	application := publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeApplication, "", 20, "publish-recovery-app",
	)
	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeRole, "role-a", 30, "publish-recovery-role",
	)
	publishPresentationEffectiveLayer(
		t, service, ctx, capability, presentation.ScopeUser, "user-a", 40, "publish-recovery-user",
	)

	recoveryPolicy := presentation.MustNewAdoptionPolicy(
		presentation.AdoptionActive, []string{capability.PageKey}, true, service.Registry,
	)
	recovery, err := NewPresentationProfileService(service.Registry, recoveryPolicy)
	require.NoError(t, err)
	recovery.Database = db

	effective, err := recovery.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.NoError(t, err)
	require.True(t, effective.RecoveryMode)
	require.True(t, effective.Fallback)
	require.Empty(t, effective.Layers.Application)
	require.Empty(t, effective.Layers.Role)
	require.Empty(t, effective.Layers.User)
	require.Equal(t, []dto.EffectivePresentationDiagnostic{{Code: "recovery-mode"}}, effective.Diagnostics)

	profile, err := recovery.Get(ctx, application.Profile.ID)
	require.NoError(t, err)
	require.Equal(t, application.Profile.ID, profile.ID)
	require.Equal(t, int64(1), profile.PublishedRevision)
	require.NotNil(t, profile.Published)
	history, err := recovery.ListRevisions(ctx, application.Profile.ID, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), history.Total)
	require.Len(t, history.Items, 1)
	require.Equal(t, int64(1), history.Items[0].Revision)
}

func publishPresentationEffectiveLayer(
	t *testing.T,
	service *PresentationProfileService,
	ctx *gin.Context,
	capability presentation.CapabilityDefinition,
	scope presentation.ScopeKind,
	subjectID string,
	pageSize int,
	idempotencyKey string,
) *dto.PresentationTransitionResponse {
	t.Helper()
	scopeValue := presentation.Scope{Kind: scope}
	if scope != presentation.ScopeApplication {
		scopeValue.Subject = &subjectID
	}
	raw := presentationProfileJSON(t, capability, scopeValue, func(profile *presentation.Profile) {
		profile.Spec.List = &presentation.ListPatch{PageSize: &pageSize}
	})
	created, err := service.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: scope, SubjectID: subjectID, PageKey: capability.PageKey,
	}, raw, "author")
	require.NoError(t, err)
	published, err := service.Publish(ctx, created.ID, created.Version, idempotencyKey, "publisher")
	require.NoError(t, err)
	return published
}
