package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPresentationDraftValidationPublicationAndConflict(t *testing.T) {
	service, _, ctx, capability := newPresentationService(t)
	identity := dto.PresentationProfileIdentity{Scope: presentation.ScopeApplication, PageKey: capability.PageKey}

	invalidSemantic := presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, func(profile *presentation.Profile) {
		missing := "missing"
		profile.Spec.List = &presentation.ListPatch{Columns: &[]presentation.FieldPatch{{Field: missing}}}
	})
	created, err := service.CreateDraft(ctx, identity, invalidSemantic, "author-a")
	require.NoError(t, err)
	require.Equal(t, int64(1), created.Version)
	require.Equal(t, "draft", created.State)
	require.NotNil(t, created.DraftValid)
	require.False(t, *created.DraftValid)
	require.Contains(t, presentationIssueCodes(created.Draft.Issues), "unknown-field")

	_, err = service.Publish(ctx, created.ID, created.Version, "publish-invalid-a", "publisher-a")
	require.ErrorIs(t, err, ErrPresentationInvalidDocument)
	current, getErr := service.Get(ctx, created.ID)
	require.NoError(t, getErr)
	require.Equal(t, int64(1), current.Version)
	require.Zero(t, current.PublishedRevision)

	valid := presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, func(profile *presentation.Profile) {
		pageSize := 50
		collapsed := false
		profile.Spec.List = &presentation.ListPatch{PageSize: &pageSize}
		profile.Spec.Search = &presentation.SearchPatch{CollapsedByDefault: &collapsed}
	})
	replaced, err := service.ReplaceDraft(ctx, created.ID, created.Version, valid, "author-b")
	require.NoError(t, err)
	require.Equal(t, int64(2), replaced.Version)
	require.True(t, *replaced.DraftValid)
	require.NotNil(t, replaced.Draft.Issues)
	replacedDraftJSON, err := json.Marshal(replaced.Draft)
	require.NoError(t, err)
	require.Contains(t, string(replacedDraftJSON), `"issues":[]`)

	_, err = service.ReplaceDraft(ctx, created.ID, created.Version, valid, "stale-author")
	require.ErrorIs(t, err, ErrPresentationRevisionConflict)
	var conflict *PresentationRevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, int64(2), conflict.Actual)
	conflictJSON, err := json.Marshal(conflict.Current)
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"id":%q,"version":2}`, created.ID), string(conflictJSON))

	published, err := service.Publish(ctx, created.ID, replaced.Version, "publish-orders-v1", "publisher-a")
	require.NoError(t, err)
	require.False(t, published.Replayed)
	require.Equal(t, "published", published.Profile.State)
	require.Equal(t, int64(3), published.Profile.Version)
	require.Equal(t, int64(1), published.Profile.PublishedRevision)
	require.Nil(t, published.Profile.Draft)
	require.Equal(t, models.PresentationTransitionPublish, published.Revision.Transition)
}

func TestPresentationPublishRejectsLimitedTableVisibilityConditions(t *testing.T) {
	capability := servicePresentationCapability(t)
	capability.Fields[0].Surfaces = []presentation.Surface{presentation.SurfaceList, presentation.SurfaceSearch}
	capability.Actions = []presentation.CapabilityAction{}
	capability.DefaultPresentation.Form.Fields = []presentation.CompleteField{}
	capability.DefaultPresentation.Detail.Fields = []presentation.CompleteField{}
	capability.DefaultPresentation.Actions = []presentation.CompleteAction{}
	var err error
	capability.DefinitionHash, err = presentation.ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.Empty(t, presentation.ValidateCapability(&capability))

	service, _, ctx := newPresentationServiceForCapability(t, capability)
	raw := presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, func(profile *presentation.Profile) {
		field := "status"
		operator := "eq"
		value := json.RawMessage(`"open"`)
		profile.Spec.List = &presentation.ListPatch{Columns: &[]presentation.FieldPatch{{
			Field: field,
			VisibleWhen: &presentation.Condition{
				Field: &field, Operator: &operator, Value: &value,
			},
		}}}
	})
	created, err := service.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: presentation.ScopeApplication, PageKey: capability.PageKey,
	}, raw, "author")
	require.NoError(t, err)
	require.NotNil(t, created.DraftValid)
	require.False(t, *created.DraftValid)
	require.Contains(t, presentationIssueCodes(created.Draft.Issues), "unsupported-limited-surface")

	_, err = service.Publish(ctx, created.ID, created.Version, "publish-limited-condition", "publisher")
	require.ErrorIs(t, err, ErrPresentationInvalidDocument)
}

func TestPresentationPublishRejectsLimitedTableUnsupportedSurface(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		mutate   func(*presentation.Profile)
		wantPath string
	}{
		{
			name: "data source",
			mutate: func(profile *presentation.Profile) {
				dataSource := "orders.list"
				profile.Spec.DataSource = &dataSource
			},
			wantPath: "spec.dataSource",
		},
		{
			name: "form",
			mutate: func(profile *presentation.Profile) {
				columns := 1
				profile.Spec.Form = &presentation.FormPatch{Columns: &columns}
			},
			wantPath: "spec.form",
		},
		{
			name: "list component",
			mutate: func(profile *presentation.Profile) {
				component := "text"
				fields := []presentation.FieldPatch{{Field: "status", Component: &component}}
				profile.Spec.List = &presentation.ListPatch{Columns: &fields}
			},
			wantPath: "spec.list.columns[0].component",
		},
		{
			name: "default sort",
			mutate: func(profile *presentation.Profile) {
				sorts := []presentation.Sort{{Field: "status", Direction: "asc"}}
				profile.Spec.List = &presentation.ListPatch{DefaultSort: &sorts}
			},
			wantPath: "spec.list.defaultSort",
		},
		{
			name: "search help",
			mutate: func(profile *presentation.Profile) {
				help := presentationText("状态帮助", "Status help")
				fields := []presentation.FieldPatch{{Field: "status", Help: &help}}
				profile.Spec.Search = &presentation.SearchPatch{Fields: &fields}
			},
			wantPath: "spec.search.fields[0].help",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			capability := servicePresentationCapability(t)
			capability.Fields[0].Surfaces = []presentation.Surface{presentation.SurfaceList, presentation.SurfaceSearch}
			capability.Actions = []presentation.CapabilityAction{}
			capability.DefaultPresentation.Form.Fields = []presentation.CompleteField{}
			capability.DefaultPresentation.Detail.Fields = []presentation.CompleteField{}
			capability.DefaultPresentation.Actions = []presentation.CompleteAction{}
			capability.DefaultPresentation.List.DefaultSort = []presentation.Sort{}
			var err error
			capability.DefinitionHash, err = presentation.ComputeDefinitionHash(&capability)
			require.NoError(t, err)
			require.Empty(t, presentation.ValidateCapability(&capability))

			service, _, ctx := newPresentationServiceForCapability(t, capability)
			raw := presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, testCase.mutate)
			created, err := service.CreateDraft(ctx, dto.PresentationProfileIdentity{
				Scope: presentation.ScopeApplication, PageKey: capability.PageKey,
			}, raw, "author")
			require.NoError(t, err)
			require.NotNil(t, created.DraftValid)
			require.False(t, *created.DraftValid)
			require.Equal(t, []string{"unsupported-limited-surface"}, presentationIssueCodes(created.Draft.Issues))
			require.Equal(t, testCase.wantPath, created.Draft.Issues[0].Path)

			_, err = service.Publish(ctx, created.ID, created.Version, "publish-limited-surface", "publisher")
			require.ErrorIs(t, err, ErrPresentationInvalidDocument)
			current, getErr := service.Get(ctx, created.ID)
			require.NoError(t, getErr)
			require.Zero(t, current.PublishedRevision)
		})
	}
}

func TestPresentationConcurrentPublicationCommitsExactlyOneRevision(t *testing.T) {
	service, db, _, capability := newPresentationService(t)
	identity := dto.PresentationProfileIdentity{Scope: presentation.ScopeApplication, PageKey: capability.PageKey}
	ctx := newPresentationServiceTestContext()
	created, err := service.CreateDraft(
		ctx,
		identity,
		presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil),
		"author",
	)
	require.NoError(t, err)

	start := make(chan struct{})
	results := make(chan error, 2)
	for attempt := 0; attempt < 2; attempt++ {
		attempt := attempt
		go func() {
			requestContext := newPresentationServiceTestContext()
			<-start
			_, publishErr := service.Publish(
				requestContext,
				created.ID,
				created.Version,
				fmt.Sprintf("publish-concurrent-%d", attempt),
				fmt.Sprintf("publisher-%d", attempt),
			)
			results <- publishErr
		}()
	}
	close(start)

	succeeded := 0
	conflicted := 0
	for attempt := 0; attempt < 2; attempt++ {
		switch publishErr := <-results; {
		case publishErr == nil:
			succeeded++
		case errors.Is(publishErr, ErrPresentationRevisionConflict):
			conflicted++
		default:
			require.NoError(t, publishErr)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, conflicted)

	current, err := service.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), current.Version)
	require.Equal(t, int64(1), current.PublishedRevision)
	var revisionCount int64
	require.NoError(t, db.Model(&models.PresentationRevision{}).
		Where("profile_id = ?", created.ID).
		Count(&revisionCount).Error)
	require.Equal(t, int64(1), revisionCount)
}

func TestPresentationPublishIdempotencyAndConflictingReuse(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	identity := dto.PresentationProfileIdentity{Scope: presentation.ScopeApplication, PageKey: capability.PageKey}
	created, err := service.CreateDraft(ctx, identity, presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil), "author")
	require.NoError(t, err)

	first, err := service.Publish(ctx, created.ID, created.Version, "publish-idempotent-1", "publisher")
	require.NoError(t, err)
	require.False(t, first.Replayed)

	draftAfterPublish, err := service.ReplaceDraft(
		ctx,
		created.ID,
		first.Profile.Version,
		presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil),
		"author-later",
	)
	require.NoError(t, err)
	require.NotNil(t, draftAfterPublish.Draft)

	retry, err := service.Publish(ctx, created.ID, created.Version, "publish-idempotent-1", "publisher")
	require.NoError(t, err)
	require.True(t, retry.Replayed)
	require.Equal(t, first.Revision.Revision, retry.Revision.Revision)
	require.Equal(t, first.Revision.ContentDigest, retry.Revision.ContentDigest)
	firstProfileJSON, err := json.Marshal(first.Profile)
	require.NoError(t, err)
	retryProfileJSON, err := json.Marshal(retry.Profile)
	require.NoError(t, err)
	require.JSONEq(t, string(firstProfileJSON), string(retryProfileJSON))
	require.Nil(t, retry.Profile.Draft)
	require.NotEqual(t, draftAfterPublish.Version, retry.Profile.Version)

	_, err = service.Publish(ctx, created.ID, created.Version, "publish-idempotent-1", "another-publisher")
	require.ErrorIs(t, err, ErrPresentationIdempotencyConflict)

	var count int64
	require.NoError(t, db.Model(&models.PresentationRevision{}).Where("profile_id = ?", created.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)

	_, err = service.Publish(ctx, created.ID, retry.Profile.Version, "publish-idempotent-1", "publisher")
	require.ErrorIs(t, err, ErrPresentationIdempotencyConflict)
	_, err = service.Publish(ctx, created.ID, retry.Profile.Version, " short ", "publisher")
	require.ErrorIs(t, err, ErrPresentationIdempotencyKey)
}

func TestPresentationRollbackRepublishesHistoryAndRejectsPendingDraft(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	identity := dto.PresentationProfileIdentity{Scope: presentation.ScopeApplication, PageKey: capability.PageKey}
	firstDocument := presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, func(profile *presentation.Profile) {
		pageSize := 20
		profile.Spec.List = &presentation.ListPatch{PageSize: &pageSize}
	})
	created, err := service.CreateDraft(ctx, identity, firstDocument, "author")
	require.NoError(t, err)
	first, err := service.Publish(ctx, created.ID, created.Version, "publish-first-1", "publisher")
	require.NoError(t, err)

	secondDocument := presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, func(profile *presentation.Profile) {
		pageSize := 50
		profile.Spec.List = &presentation.ListPatch{PageSize: &pageSize}
	})
	draft, err := service.ReplaceDraft(ctx, created.ID, first.Profile.Version, secondDocument, "author")
	require.NoError(t, err)
	_, err = service.Rollback(ctx, created.ID, draft.Version, 1, "rollback-blocked-1", "recovery")
	require.ErrorIs(t, err, ErrPresentationInvalidTransition)

	second, err := service.Publish(ctx, created.ID, draft.Version, "publish-second-1", "publisher")
	require.NoError(t, err)
	rollback, err := service.Rollback(ctx, created.ID, second.Profile.Version, 1, "rollback-first-1", "recovery")
	require.NoError(t, err)
	require.Equal(t, int64(3), rollback.Revision.Revision)
	require.Equal(t, models.PresentationTransitionRollback, rollback.Revision.Transition)
	require.Equal(t, int64(1), *rollback.Revision.SourceRevision)
	require.Equal(t, first.Revision.ContentDigest, rollback.Revision.ContentDigest)
	require.Equal(t, int64(5), rollback.Profile.Version)

	retry, err := service.Rollback(ctx, created.ID, second.Profile.Version, 1, "rollback-first-1", "recovery")
	require.NoError(t, err)
	require.True(t, retry.Replayed)
	require.Equal(t, rollback.Revision.Revision, retry.Revision.Revision)

	var count int64
	require.NoError(t, db.Model(&models.PresentationRevision{}).Where("profile_id = ?", created.ID).Count(&count).Error)
	require.Equal(t, int64(3), count)
	history, err := service.ListRevisions(ctx, created.ID, 1, 1000)
	require.NoError(t, err)
	require.Equal(t, PresentationHistoryMaxPageSize, history.PageSize)
	require.Equal(t, []int64{3, 2, 1}, []int64{history.Items[0].Revision, history.Items[1].Revision, history.Items[2].Revision})
}

func TestPresentationRollbackAndEffectiveReadRejectDefinitionDrift(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	identity := dto.PresentationProfileIdentity{Scope: presentation.ScopeApplication, PageKey: capability.PageKey}
	created, err := service.CreateDraft(ctx, identity, presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil), "author")
	require.NoError(t, err)
	published, err := service.Publish(ctx, created.ID, created.Version, "publish-drift-1", "publisher")
	require.NoError(t, err)

	changed := capability
	changed.Fields[0].Required = false
	changed.DefinitionHash, err = presentation.ComputeDefinitionHash(&changed)
	require.NoError(t, err)
	driftedRegistry := presentation.MustNewRegistry(changed)
	driftedPolicy := presentation.MustNewAdoptionPolicy(
		presentation.AdoptionActive, []string{changed.PageKey}, false, driftedRegistry,
	)
	drifted, err := NewPresentationProfileService(driftedRegistry, driftedPolicy)
	require.NoError(t, err)
	drifted.Database = db

	_, err = drifted.Rollback(ctx, created.ID, published.Profile.Version, 1, "rollback-drift-1", "recovery")
	require.ErrorIs(t, err, ErrPresentationInvalidDocument)
	var documentErr *PresentationDocumentError
	require.ErrorAs(t, err, &documentErr)
	require.Contains(t, presentationIssueCodes(documentErr.Issues), "definition-drift")

	effective, err := drifted.Effective(ctx, capability.PageKey, "", "")
	require.NoError(t, err)
	require.True(t, effective.Fallback)
	require.Empty(t, effective.Layers.Application)
	require.Equal(t, "published-definition-drift", effective.Diagnostics[0].Code)
}

func TestPresentationScopeOwnershipAndListBounds(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	roleSubject := "missing-role"
	raw := presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeRole, Subject: &roleSubject}, nil)
	_, err := service.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: presentation.ScopeRole, SubjectID: roleSubject, PageKey: capability.PageKey,
	}, raw, "author")
	require.ErrorIs(t, err, ErrPresentationSubjectNotFound)

	seedPresentationSubject(t, db, presentation.ScopeRole, "role-a")
	_, err = service.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: presentation.ScopeRole, SubjectID: "role-a", PageKey: capability.PageKey,
	}, raw, "author")
	require.ErrorIs(t, err, ErrPresentationIdentityMismatch)

	created, err := service.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: presentation.ScopeApplication, PageKey: capability.PageKey,
	}, presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil), "author")
	require.NoError(t, err)
	_, err = service.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: presentation.ScopeApplication, PageKey: capability.PageKey,
	}, presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil), "author")
	require.ErrorIs(t, err, ErrPresentationIdentityConflict)
	published, err := service.Publish(ctx, created.ID, created.Version, "publish-list-projection", "publisher")
	require.NoError(t, err)

	list, err := service.List(ctx, dto.PresentationProfileListRequest{Page: -1, PageSize: 1000})
	require.NoError(t, err)
	require.Equal(t, 1, list.Page)
	require.Equal(t, PresentationListMaxPageSize, list.PageSize)
	require.Equal(t, int64(1), list.Total)
	require.Equal(t, created.ID, list.Items[0].ID)
	require.Equal(t, "published", list.Items[0].State)
	require.Equal(t, published.Profile.PublishedRevision, list.Items[0].PublishedRevision)
}

func TestPresentationRoleScopeAcceptsOpaqueServerIdentifier(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	const roleID = "0123456789abcdef0123456789abcdef"
	role := &models.Role{Name: "opaque-role", Status: enum.Enabled}
	role.ID = roleID
	require.NoError(t, db.Create(role).Error)

	roleSubject := role.ID
	raw := presentationProfileJSON(
		t,
		capability,
		presentation.Scope{Kind: presentation.ScopeRole, Subject: &roleSubject},
		nil,
	)
	created, err := service.CreateDraft(ctx, dto.PresentationProfileIdentity{
		Scope: presentation.ScopeRole, SubjectID: role.ID, PageKey: capability.PageKey,
	}, raw, "author")
	require.NoError(t, err)
	require.Equal(t, role.ID, created.SubjectID)
}

func newPresentationService(t *testing.T) (*PresentationProfileService, *gorm.DB, *gin.Context, presentation.CapabilityDefinition) {
	t.Helper()
	capability := servicePresentationCapability(t)
	service, db, ctx := newPresentationServiceForCapability(t, capability)
	return service, db, ctx, capability
}

func newPresentationServiceForCapability(t *testing.T, capability presentation.CapabilityDefinition) (*PresentationProfileService, *gorm.DB, *gin.Context) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "-"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(
		&models.PresentationProfile{}, &models.PresentationRevision{}, &models.Role{}, &models.User{},
	))
	registry := presentation.MustNewRegistry(capability)
	policy := presentation.MustNewAdoptionPolicy(
		presentation.AdoptionActive, []string{capability.PageKey}, false, registry,
	)
	service, err := NewPresentationProfileService(registry, policy)
	require.NoError(t, err)
	service.Database = db
	ctx := newPresentationServiceTestContext()
	return service, db, ctx
}

func newPresentationServiceTestContext() *gin.Context {
	request := httptest.NewRequest("GET", "/", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	return ctx
}

func servicePresentationCapability(t *testing.T) presentation.CapabilityDefinition {
	t.Helper()
	capability := presentation.CapabilityDefinition{
		PageKey: "orders.list", DefinitionVersion: presentation.DefinitionVersion,
		Components: []presentation.CapabilityComponent{{ID: "text"}, {ID: "select"}},
		Fields: []presentation.CapabilityField{{
			ID: "status", Label: presentationText("状态", "Status"), ValueType: "enum", Required: true,
			Sortable: true, Filterable: true,
			Surfaces:   []presentation.Surface{presentation.SurfaceList, presentation.SurfaceSearch, presentation.SurfaceForm, presentation.SurfaceDetail},
			Components: []string{"text", "select"},
		}},
		DataSources: []presentation.CapabilityDataSource{{ID: "orders.list", RequiredPermissions: []string{"/orders", "/orders/permissions/list"}}},
		Actions:     []presentation.CapabilityAction{{ID: "orders.read", RequiredPermissions: []string{"/orders/permissions/read"}, Placements: []presentation.ActionPlacement{presentation.PlacementRow}}},
		DefaultPresentation: presentation.CompletePresentation{
			Title: presentationText("订单", "Orders"), DataSource: "orders.list",
			List: presentation.CompleteListPresentation{
				Columns: []presentation.CompleteField{{Field: "status", Component: "text", Order: 10}},
				Density: "middle", PageSize: 20, DefaultSort: []presentation.Sort{{Field: "status", Direction: "asc"}},
			},
			Search:  presentation.CompleteSearchPresentation{Fields: []presentation.CompleteField{{Field: "status", Component: "select", Order: 10}}},
			Form:    presentation.CompleteFormPresentation{Fields: []presentation.CompleteField{{Field: "status", Component: "select", Order: 10}}, Columns: 1},
			Detail:  presentation.CompleteDetailPresentation{Fields: []presentation.CompleteField{{Field: "status", Component: "text", Order: 10}}, Columns: 1},
			Actions: []presentation.CompleteAction{{Action: "orders.read", Placement: presentation.PlacementRow, Order: 10}},
		},
	}
	capability.DefinitionHash, _ = presentation.ComputeDefinitionHash(&capability)
	require.Empty(t, presentation.ValidateCapability(&capability))
	return capability
}

func presentationProfileJSON(
	t *testing.T,
	capability presentation.CapabilityDefinition,
	scope presentation.Scope,
	mutate func(*presentation.Profile),
) json.RawMessage {
	t.Helper()
	profile := &presentation.Profile{
		APIVersion: presentation.APIVersion,
		Kind:       presentation.Kind,
		Metadata: presentation.Metadata{
			Name: "orders-default", PageKey: capability.PageKey,
			DefinitionHash: capability.DefinitionHash, Scope: scope,
		},
		Spec: presentation.ProfileSpec{Title: ptrPresentationText("订单视图", "Order view")},
	}
	if mutate != nil {
		mutate(profile)
	}
	raw, err := json.Marshal(profile)
	require.NoError(t, err)
	return raw
}

func presentationText(zhCN, enUS string) presentation.LocalizedText {
	return presentation.LocalizedText{ZhCN: &zhCN, EnUS: &enUS}
}

func ptrPresentationText(zhCN, enUS string) *presentation.LocalizedText {
	value := presentationText(zhCN, enUS)
	return &value
}

func presentationIssueCodes(issues []presentation.Issue) []string {
	result := make([]string, 0, len(issues))
	for _, current := range issues {
		result = append(result, current.Code)
	}
	return result
}

func seedPresentationSubject(t *testing.T, db *gorm.DB, scope presentation.ScopeKind, id string) {
	t.Helper()
	switch scope {
	case presentation.ScopeRole:
		role := &models.Role{Name: id, Status: enum.Enabled}
		role.ID = id
		require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(role).Error)
	case presentation.ScopeUser:
		user := &models.User{UserLogin: models.UserLogin{Username: id, Status: enum.Enabled}}
		user.ID = id
		require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Omit("Role", "Post", "Department", "OAuth2").Create(user).Error)
	default:
		require.FailNow(t, "unsupported presentation subject scope")
	}
}

func effectivePageSize(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	document, issues := presentation.ParseDocument(raw)
	require.Empty(t, issues)
	require.NotNil(t, document.Profile.Spec.List)
	require.NotNil(t, document.Profile.Spec.List.PageSize)
	return *document.Profile.Spec.List.PageSize
}

func TestPresentationEffectiveStoreFailureReturnsSafeFallback(t *testing.T) {
	service, db, ctx, capability := newPresentationService(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	effective, err := service.Effective(ctx, capability.PageKey, "user-a", "role-a")
	require.Error(t, err)
	require.True(t, effective.Fallback)
	require.Empty(t, effective.Layers.Application)
	require.Equal(t, "presentation-store-unavailable", effective.Diagnostics[0].Code)
}

func TestPresentationRevisionLookupIsProfileScoped(t *testing.T) {
	service, _, ctx, capability := newPresentationService(t)
	identity := dto.PresentationProfileIdentity{Scope: presentation.ScopeApplication, PageKey: capability.PageKey}
	created, err := service.CreateDraft(ctx, identity, presentationProfileJSON(t, capability, presentation.Scope{Kind: presentation.ScopeApplication}, nil), "author")
	require.NoError(t, err)
	published, err := service.Publish(ctx, created.ID, created.Version, "publish-lookup-1", "publisher")
	require.NoError(t, err)

	revision, err := service.GetRevision(ctx, created.ID, 1)
	require.NoError(t, err)
	require.Equal(t, published.Revision.ContentDigest, revision.ContentDigest)
	_, err = service.GetRevision(ctx, "another-profile", 1)
	require.True(t, errors.Is(err, ErrPresentationRevisionNotFound))
}
