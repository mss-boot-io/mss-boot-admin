package service

import (
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"github.com/stretchr/testify/require"
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

	recovery := &PresentationProfileService{
		Database: db,
		Registry: service.Registry,
		RecoveryMode: func() bool {
			return true
		},
	}
	recovered, err := recovery.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.NoError(t, err)
	require.True(t, recovered.RecoveryMode)
	require.True(t, recovered.Fallback)
	require.Empty(t, recovered.Layers.Application)
}
