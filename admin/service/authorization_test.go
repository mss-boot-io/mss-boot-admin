package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	bootenum "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

func setupAuthorizationServiceTest(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(
		&models.Role{},
		&models.Menu{},
		&models.API{},
		&models.CasbinRule{},
		&models.ConfigRevision{},
	))
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mss_boot_users (
		id varchar(64) PRIMARY KEY,
		role_id varchar(64),
		deleted_at datetime
	)`).Error)
	role := models.Role{Name: "role-a", Status: bootenum.Enabled}
	role.ID = "role-a"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&role).Error)
	return db
}

func authorizationTestMenu(id, path string, accessType adminpkg.AccessType, parentID string) models.Menu {
	menu := models.Menu{
		Path:     path,
		Method:   "GET",
		Type:     accessType,
		ParentID: parentID,
		Status:   bootenum.Enabled,
	}
	menu.ID = id
	return menu
}

func insertAuthorizationTestMenus(t *testing.T, db *gorm.DB, menus ...models.Menu) {
	t.Helper()
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&menus).Error)
}

func authorizationRevision(t *testing.T, db *gorm.DB, key configRevisionKey) int64 {
	t.Helper()
	revision, err := readConfigRevision(db, key)
	require.NoError(t, err)
	return revision
}

func TestAuthorizationReadRoleReturnsSortedStableResource(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	require.NoError(t, db.Create(&[]models.CasbinRule{
		{PType: "p", V0: "role-a", V1: adminpkg.ComponentAccessType.String(), V2: "/z", V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.MenuAccessType.String(), V2: "/a", V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.MenuAccessType.String(), V2: "/z", V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.APIAccessType.String(), V2: "/admin/api/hidden", V3: "GET"},
	}).Error)

	resource, err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
		ReadRole(context.Background(), db, "role-a")
	require.NoError(t, err)
	require.Equal(t, "role-a", resource.RoleID)
	require.Equal(t, []string{"/a", "/z"}, resource.Paths)
	require.Equal(t, "0", resource.Revision)
	require.Equal(t, int64(0), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-a")))
}

func TestAuthorizationReadRoleRejectsMissingAndSoftDeletedTargets(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		_, err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			ReadRole(context.Background(), db, "missing-role")
		require.ErrorIs(t, err, ErrAuthorizationRoleNotFound)
		var revisionCount int64
		require.NoError(t, db.Model(&models.ConfigRevision{}).
			Where("scope = ? AND owner_id = ?", authorizationRevisionScopeRole, "missing-role").
			Count(&revisionCount).Error)
		require.Zero(t, revisionCount)
	})

	t.Run("soft deleted", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		require.NoError(t, db.Delete(&models.Role{}, "id = ?", "role-a").Error)
		_, err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			ReadRole(context.Background(), db, "role-a")
		require.ErrorIs(t, err, ErrAuthorizationRoleNotFound)
	})
}

func TestAuthorizationReplaceRoleRejectsMissingSoftDeletedAndInactiveTargets(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		_, err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			ReplaceRole(context.Background(), db, "missing-role", []string{}, 0)
		require.ErrorIs(t, err, ErrAuthorizationRoleNotFound)
		var revisionCount int64
		require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
		require.Zero(t, revisionCount)
	})

	t.Run("soft deleted", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		require.NoError(t, db.Delete(&models.Role{}, "id = ?", "role-a").Error)
		_, err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			ReplaceMenu(context.Background(), db, "role-a", []string{}, 0)
		require.ErrorIs(t, err, ErrAuthorizationRoleNotFound)
	})

	t.Run("inactive", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		require.NoError(t, db.Model(&models.Role{}).Where("id = ?", "role-a").Update("status", bootenum.Disabled).Error)
		_, err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			ReplaceRole(context.Background(), db, "role-a", []string{}, 0)
		require.ErrorIs(t, err, ErrAuthorizationRoleInactive)
		var revisionCount int64
		require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
		require.Zero(t, revisionCount)
	})
}

func TestAuthorizationReplaceRoleAcceptsExplicitEmptySet(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	require.NoError(t, db.Create(&[]models.CasbinRule{
		{PType: "p", V0: "role-a", V1: adminpkg.MenuAccessType.String(), V2: "/menu", V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.ComponentAccessType.String(), V2: "/action", V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.APIAccessType.String(), V2: "/admin/api/a", V3: "GET"},
	}).Error)
	var reloads atomic.Int64
	var notifications atomic.Int64
	svc := newAuthorizationPolicyService(
		func() error { reloads.Add(1); return nil },
		func() error { notifications.Add(1); return nil },
	)
	revision0 := int64(0)
	resource, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{}, revision0)
	require.NoError(t, err)
	require.Empty(t, resource.Paths)
	require.NotNil(t, resource.Paths)
	require.Equal(t, "1", resource.Revision)
	var count int64
	require.NoError(t, db.Model(&models.CasbinRule{}).Where("v0 = ?", "role-a").Count(&count).Error)
	require.Zero(t, count)
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-a")))
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))
	require.Equal(t, int64(1), reloads.Load())
	require.Equal(t, int64(1), notifications.Load())
}

func TestAuthorizationReplaceRoleBuildsDerivedAPIRulesAndRejectsInactivePaths(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	menu := authorizationTestMenu("menu-a", "/menu-a", adminpkg.MenuAccessType, "")
	component := authorizationTestMenu("component-a", "/component-a", adminpkg.ComponentAccessType, "menu-a")
	api := authorizationTestMenu("api-a", "/admin/api/a", adminpkg.APIAccessType, "menu-a")
	disabled := authorizationTestMenu("menu-disabled", "/disabled", adminpkg.MenuAccessType, "")
	disabled.Status = bootenum.Disabled
	duplicateA := authorizationTestMenu("menu-duplicate-a", "/duplicate", adminpkg.MenuAccessType, "")
	duplicateB := authorizationTestMenu("menu-duplicate-b", "/duplicate", adminpkg.ComponentAccessType, "")
	insertAuthorizationTestMenus(t, db, menu, component, api, disabled, duplicateA, duplicateB)

	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	revision0 := int64(0)
	resource, err := svc.ReplaceRole(
		context.Background(), db, "role-a", []string{" /component-a ", "/menu-a", "/menu-a"}, revision0,
	)
	require.NoError(t, err)
	require.Equal(t, []string{"/component-a", "/menu-a"}, resource.Paths)
	var rules []models.CasbinRule
	require.NoError(t, db.Where("v0 = ?", "role-a").Order("v1, v2").Find(&rules).Error)
	require.Len(t, rules, 3)
	require.Equal(t, []string{adminpkg.APIAccessType.String(), adminpkg.ComponentAccessType.String(), adminpkg.MenuAccessType.String()},
		[]string{rules[0].V1, rules[1].V1, rules[2].V1})

	_, err = svc.ReplaceRole(context.Background(), db, "role-a", []string{"/disabled"}, 1)
	var invalid *InvalidAuthorizationPathsError
	require.ErrorAs(t, err, &invalid)
	require.Equal(t, []string{"/disabled"}, invalid.Paths)
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-a")))

	_, err = svc.ReplaceRole(context.Background(), db, "role-a", []string{"/duplicate"}, 1)
	require.ErrorAs(t, err, &invalid)
	require.Equal(t, []string{"/duplicate"}, invalid.Paths)
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-a")))
}

func TestAuthorizationReplaceRoleStaleRevisionReturnsCurrentWithoutMutation(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	insertAuthorizationTestMenus(t, db,
		authorizationTestMenu("menu-a", "/a", adminpkg.MenuAccessType, ""),
		authorizationTestMenu("menu-b", "/b", adminpkg.MenuAccessType, ""),
	)
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	revision0 := int64(0)
	first, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{"/a"}, revision0)
	require.NoError(t, err)
	require.Equal(t, "1", first.Revision)

	_, err = svc.ReplaceRole(context.Background(), db, "role-a", []string{"/b"}, revision0)
	var conflict *AuthorizationRevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, int64(0), conflict.Expected)
	require.Equal(t, int64(1), conflict.Actual)
	require.Equal(t, []string{"/a"}, conflict.Current.Paths)
	require.Equal(t, "1", conflict.Current.Revision)
	current, readErr := svc.ReadRole(context.Background(), db, "role-a")
	require.NoError(t, readErr)
	require.Equal(t, []string{"/a"}, current.Paths)
}

func TestAuthorizationConcurrentWritersWithSameRevisionAllowOneCommit(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	insertAuthorizationTestMenus(t, db,
		authorizationTestMenu("menu-a", "/a", adminpkg.MenuAccessType, ""),
		authorizationTestMenu("menu-b", "/b", adminpkg.MenuAccessType, ""),
	)
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	revision0 := int64(0)
	start := make(chan struct{})
	errorsCh := make(chan error, 2)
	var wait sync.WaitGroup
	for _, path := range []string{"/a", "/b"} {
		path := path
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{path}, revision0)
			errorsCh <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsCh)

	successes := 0
	conflicts := 0
	for err := range errorsCh {
		if err == nil {
			successes++
			continue
		}
		var conflict *AuthorizationRevisionConflictError
		if errors.As(err, &conflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent writer error: %v", err)
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-a")))
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))
}

func TestAuthorizationReloadFailureCommitsRevisionAndRetriesFailClosed(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	insertAuthorizationTestMenus(t, db, authorizationTestMenu("menu-a", "/a", adminpkg.MenuAccessType, ""))
	reloadErr := errors.New("reload unavailable")
	var reloads atomic.Int64
	var notifications atomic.Int64
	svc := newAuthorizationPolicyService(
		func() error { reloads.Add(1); return reloadErr },
		func() error { notifications.Add(1); return nil },
	)
	revision0 := int64(0)
	resource, err := svc.ReplaceRole(context.Background(), db, "role-a", []string{"/a"}, revision0)
	require.Equal(t, "1", resource.Revision)
	var propagation *AuthorizationPropagationError
	require.ErrorAs(t, err, &propagation)
	require.ErrorIs(t, err, reloadErr)
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))
	require.Equal(t, int64(1), reloads.Load())
	require.Zero(t, notifications.Load())

	// The failed reload never advances last-seen state. Every request remains
	// fail closed until a later reconciliation succeeds.
	require.Error(t, svc.EnsureCurrent(context.Background(), db))
	require.Equal(t, int64(2), reloads.Load())
	svc.reloadPolicy = func() error { reloads.Add(1); return nil }
	require.NoError(t, svc.EnsureCurrent(context.Background(), db))
	require.Equal(t, int64(3), reloads.Load())
	require.NoError(t, svc.EnsureCurrent(context.Background(), db))
	require.Equal(t, int64(3), reloads.Load(), "unchanged revision must not reload again")
}

func TestAuthorizationEnsureCurrentReloadsOnlyWhenDurableGlobalRevisionChanges(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	var reloads atomic.Int64
	svc := newAuthorizationPolicyService(func() error { reloads.Add(1); return nil }, func() error { return nil })

	require.NoError(t, svc.EnsureCurrent(context.Background(), db))
	require.NoError(t, svc.EnsureCurrent(context.Background(), db))
	require.Equal(t, int64(1), reloads.Load())
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		current, err := lockConfigRevision(tx, globalAuthorizationRevisionKey())
		if err != nil {
			return err
		}
		_, err = advanceConfigRevision(tx, globalAuthorizationRevisionKey(), current)
		return err
	}))
	require.NoError(t, svc.EnsureCurrent(context.Background(), db))
	require.Equal(t, int64(2), reloads.Load())
}

func TestAuthorizationReplaceMenuEmptySetRevokesDerivedAPIAndPreservesComponentRules(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	require.NoError(t, db.Create(&[]models.CasbinRule{
		{PType: "p", V0: "role-a", V1: adminpkg.MenuAccessType.String(), V2: "/menu", V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.ComponentAccessType.String(), V2: "/component", V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.APIAccessType.String(), V2: "/admin/api/a", V3: "GET"},
	}).Error)
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	revision0 := int64(0)
	resource, err := svc.ReplaceMenu(context.Background(), db, "role-a", []string{}, revision0)
	require.NoError(t, err)
	require.Equal(t, []string{"/component"}, resource.Paths)
	var rules []models.CasbinRule
	require.NoError(t, db.Where("v0 = ?", "role-a").Order("v1").Find(&rules).Error)
	require.Len(t, rules, 1)
	require.Equal(t, adminpkg.ComponentAccessType.String(), rules[0].V1)
}

func TestAuthorizationBindMenuAPIsExplicitEmptyRevokesAffectedRoleGrants(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	parent := authorizationTestMenu("menu-a", "/menu-a", adminpkg.MenuAccessType, "")
	oldAPI := authorizationTestMenu("old-api", "/admin/api/old", adminpkg.APIAccessType, parent.ID)
	insertAuthorizationTestMenus(t, db, parent, oldAPI)
	require.NoError(t, db.Create(&[]models.CasbinRule{
		{PType: "p", V0: "role-menu", V1: adminpkg.MenuAccessType.String(), V2: parent.Path, V3: "GET"},
		{PType: "p", V0: "role-menu", V1: adminpkg.APIAccessType.String(), V2: oldAPI.Path, V3: oldAPI.Method},
		{PType: "p", V0: "role-orphan", V1: adminpkg.APIAccessType.String(), V2: oldAPI.Path, V3: oldAPI.Method},
		{PType: "p", V0: "role-method-drift", V1: adminpkg.APIAccessType.String(), V2: oldAPI.Path, V3: "POST"},
	}).Error)
	var reloads atomic.Int64
	var notifications atomic.Int64
	svc := newAuthorizationPolicyService(
		func() error { reloads.Add(1); return nil },
		func() error { notifications.Add(1); return nil },
	)

	require.NoError(t, svc.BindMenuAPIs(context.Background(), db, parent.ID, []AuthorizationAPIReference{}))

	var childCount int64
	require.NoError(t, db.Unscoped().Model(&models.Menu{}).
		Where("parent_id = ? AND type = ?", parent.ID, adminpkg.APIAccessType).
		Count(&childCount).Error)
	require.Zero(t, childCount)
	var apiRuleCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).
		Where("v1 = ?", adminpkg.APIAccessType.String()).
		Count(&apiRuleCount).Error)
	require.Zero(t, apiRuleCount)
	var menuRuleCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).
		Where("v0 = ? AND v1 = ?", "role-menu", adminpkg.MenuAccessType.String()).
		Count(&menuRuleCount).Error)
	require.Equal(t, int64(1), menuRuleCount)
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-menu")))
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-orphan")))
	require.Equal(t, int64(1), authorizationRevision(t, db, roleAuthorizationRevisionKey("role-method-drift")))
	require.Equal(t, int64(1), authorizationRevision(t, db, globalAuthorizationRevisionKey()))
	require.Equal(t, int64(1), reloads.Load())
	require.Equal(t, int64(1), notifications.Load())
}

func TestAuthorizationBindMenuAPIsRecomputesSelectedMenuRules(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	parent := authorizationTestMenu("menu-a", "/menu-a", adminpkg.MenuAccessType, "")
	oldAPI := authorizationTestMenu("old-api", "/admin/api/old", adminpkg.APIAccessType, parent.ID)
	insertAuthorizationTestMenus(t, db, parent, oldAPI)
	registryAPI := models.API{Name: "new API", Path: "/admin/api/new", Method: "POST"}
	registryAPI.ID = "registry-new"
	require.NoError(t, db.Create(&registryAPI).Error)
	require.NoError(t, db.Create(&[]models.CasbinRule{
		{PType: "p", V0: "role-a", V1: adminpkg.MenuAccessType.String(), V2: parent.Path, V3: "GET"},
		{PType: "p", V0: "role-a", V1: adminpkg.APIAccessType.String(), V2: oldAPI.Path, V3: oldAPI.Method},
	}).Error)
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })

	require.NoError(t, svc.BindMenuAPIs(context.Background(), db, parent.ID, []AuthorizationAPIReference{{
		Method: " post ", Path: " /admin/api/new ",
	}}))

	var child models.Menu
	require.NoError(t, db.Where("parent_id = ? AND type = ?", parent.ID, adminpkg.APIAccessType).Take(&child).Error)
	require.Equal(t, registryAPI.Path, child.Path)
	require.Equal(t, registryAPI.Method, child.Method)
	var apiRules []models.CasbinRule
	require.NoError(t, db.Where("v0 = ? AND v1 = ?", "role-a", adminpkg.APIAccessType.String()).Find(&apiRules).Error)
	require.Len(t, apiRules, 1)
	require.Equal(t, registryAPI.Path, apiRules[0].V2)
	require.Equal(t, registryAPI.Method, apiRules[0].V3)
}

func TestAuthorizationBindMenuAPIsRejectsDuplicateHistoryAndIgnoresSoftDeletedRegistryRows(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	parent := authorizationTestMenu("menu-a", "/menu-a", adminpkg.MenuAccessType, "")
	oldAPI := authorizationTestMenu("old-api", "/admin/api/old", adminpkg.APIAccessType, parent.ID)
	insertAuthorizationTestMenus(t, db, parent, oldAPI)
	active := models.API{Name: "active", Path: "/admin/api/new", Method: "POST"}
	active.ID = "registry-active"
	history := models.API{Name: "history", Path: active.Path, Method: active.Method, History: true}
	history.ID = "registry-history"
	require.NoError(t, db.Create(&[]models.API{active, history}).Error)
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	references := []AuthorizationAPIReference{{Method: active.Method, Path: active.Path}}

	err := svc.BindMenuAPIs(context.Background(), db, parent.ID, references)
	require.ErrorIs(t, err, ErrAuthorizationAPIInvalid)
	var oldChildCount int64
	require.NoError(t, db.Model(&models.Menu{}).Where("id = ?", oldAPI.ID).Count(&oldChildCount).Error)
	require.Equal(t, int64(1), oldChildCount, "ambiguous registry input must roll back the existing binding")
	var revisionCount int64
	require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)

	require.NoError(t, db.Delete(&history).Error)
	require.NoError(t, svc.BindMenuAPIs(context.Background(), db, parent.ID, references))
	var newChild models.Menu
	require.NoError(t, db.Where("parent_id = ? AND type = ?", parent.ID, adminpkg.APIAccessType).Take(&newChild).Error)
	require.Equal(t, active.Path, newChild.Path)
	require.Equal(t, active.Method, newChild.Method)
}

func TestAuthorizationBindMenuAPIsRejectsDisabledMenuWithoutMutation(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	parent := authorizationTestMenu("menu-disabled", "/disabled", adminpkg.MenuAccessType, "")
	parent.Status = bootenum.Disabled
	oldAPI := authorizationTestMenu("old-api", "/admin/api/old", adminpkg.APIAccessType, parent.ID)
	insertAuthorizationTestMenus(t, db, parent, oldAPI)

	err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
		BindMenuAPIs(context.Background(), db, parent.ID, []AuthorizationAPIReference{})
	require.ErrorIs(t, err, ErrAuthorizationMenuNotFound)
	var childCount int64
	require.NoError(t, db.Model(&models.Menu{}).Where("parent_id = ?", parent.ID).Count(&childCount).Error)
	require.Equal(t, int64(1), childCount)
	var revisionCount int64
	require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount, "rejected binding must roll back the global lock row")
}

func TestAuthorizationMenuMetadataGuardsRejectReferencedSubtree(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	parent := authorizationTestMenu("menu-a", "/menu-a", adminpkg.MenuAccessType, "")
	child := authorizationTestMenu("api-a", "/admin/api/a", adminpkg.APIAccessType, parent.ID)
	unreferenced := authorizationTestMenu("menu-free", "/free", adminpkg.MenuAccessType, "")
	insertAuthorizationTestMenus(t, db, parent, child, unreferenced)
	require.NoError(t, db.Create(&models.CasbinRule{
		PType: "p", V0: "role-a", V1: adminpkg.APIAccessType.String(), V2: child.Path, V3: "POST",
	}).Error)

	displayOnly := parent
	displayOnly.Name = "renamed"
	require.NoError(t, ValidateMenuMetadataUpdate(context.Background(), db, &displayOnly))
	disabled := parent
	disabled.Status = bootenum.Disabled
	require.ErrorIs(t, ValidateMenuMetadataUpdate(context.Background(), db, &disabled), ErrAuthorizationMetadataInUse)
	require.ErrorIs(t, ValidateMenuMetadataDelete(context.Background(), db, parent.ID), ErrAuthorizationMetadataInUse)
	require.NoError(t, ValidateMenuMetadataDelete(context.Background(), db, unreferenced.ID))
	require.Error(t, ValidateMenuMetadataDelete(context.Background(), db, "batch"))
}

func TestAuthorizationMenuMetadataUpdateRejectsActiveCanonicalPathConflict(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	existing := authorizationTestMenu("menu-existing", "/canonical", adminpkg.MenuAccessType, "")
	target := authorizationTestMenu("component-target", "/target", adminpkg.ComponentAccessType, "")
	disabled := authorizationTestMenu("menu-disabled", "/disabled-only", adminpkg.MenuAccessType, "")
	disabled.Status = bootenum.Disabled
	insertAuthorizationTestMenus(t, db, existing, target, disabled)

	displayOnly := target
	displayOnly.Name = "renamed"
	require.NoError(t, ValidateMenuMetadataUpdate(context.Background(), db, &displayOnly))

	conflict := target
	conflict.Path = " /canonical "
	require.ErrorIs(t, ValidateMenuMetadataUpdate(context.Background(), db, &conflict), ErrAuthorizationMetadataConflict)
	require.Equal(t, existing.Path, conflict.Path, "update input must be normalized before conflict validation")

	normalized := target
	normalized.Path = " /normalized "
	require.NoError(t, ValidateMenuMetadataUpdate(context.Background(), db, &normalized))
	require.Equal(t, "/normalized", normalized.Path)

	disabledCandidate := target
	disabledCandidate.Path = disabled.Path
	require.NoError(t, ValidateMenuMetadataUpdate(context.Background(), db, &disabledCandidate))

	disabledTarget := target
	disabledTarget.Status = bootenum.Disabled
	disabledTarget.Path = existing.Path
	require.NoError(t, ValidateMenuMetadataUpdate(context.Background(), db, &disabledTarget))

	var persisted models.Menu
	require.NoError(t, db.Where("id = ?", target.ID).Take(&persisted).Error)
	require.Equal(t, target.Path, persisted.Path, "rejected validation must not mutate the canonical path")
}

func TestAuthorizationMenuMetadataCreateRejectsAmbiguousPermissionPath(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	existing := authorizationTestMenu("menu-existing", "/permission", adminpkg.MenuAccessType, "")
	disabled := authorizationTestMenu("menu-disabled", "/disabled-only", adminpkg.MenuAccessType, "")
	disabled.Status = bootenum.Disabled
	insertAuthorizationTestMenus(t, db, existing, disabled)
	duplicate := authorizationTestMenu("component-duplicate", existing.Path, adminpkg.ComponentAccessType, existing.ID)
	require.ErrorIs(t, ValidateMenuMetadataCreate(context.Background(), db, &duplicate), ErrAuthorizationMetadataConflict)
	whitespaceAlias := authorizationTestMenu("component-alias", " /permission ", adminpkg.ComponentAccessType, existing.ID)
	require.ErrorIs(t, ValidateMenuMetadataCreate(context.Background(), db, &whitespaceAlias), ErrAuthorizationMetadataConflict)
	require.Equal(t, existing.Path, whitespaceAlias.Path, "create input must be normalized before conflict validation")

	unique := authorizationTestMenu("component-unique", " /unique ", adminpkg.ComponentAccessType, existing.ID)
	require.NoError(t, ValidateMenuMetadataCreate(context.Background(), db, &unique))
	require.Equal(t, "/unique", unique.Path)

	defaultEnabled := models.Menu{Path: " /permission ", Type: adminpkg.MenuAccessType}
	require.ErrorIs(t, ValidateMenuMetadataCreate(context.Background(), db, &defaultEnabled), ErrAuthorizationMetadataConflict)
	require.Equal(t, existing.Path, defaultEnabled.Path)

	disabledCandidate := authorizationTestMenu("component-disabled-candidate", " /disabled-only ", adminpkg.ComponentAccessType, "")
	require.NoError(t, ValidateMenuMetadataCreate(context.Background(), db, &disabledCandidate))
	require.Equal(t, disabled.Path, disabledCandidate.Path)
}

func TestAuthorizationAPIMetadataGuardsRejectReferencedNaturalKeyChanges(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	registry := models.API{Name: "registry", Path: "/admin/api/resource", Method: "GET", Handler: "handler"}
	registry.ID = "registry-api"
	require.NoError(t, db.Create(&registry).Error)
	child := authorizationTestMenu("bound-api", registry.Path, adminpkg.APIAccessType, "menu-parent")
	child.Method = registry.Method
	insertAuthorizationTestMenus(t, db, child)

	displayOnly := registry
	displayOnly.Name = "renamed"
	require.NoError(t, ValidateAPIMetadataUpdate(context.Background(), db, &displayOnly))
	changedPath := registry
	changedPath.Path = "/admin/api/renamed"
	require.ErrorIs(t, ValidateAPIMetadataUpdate(context.Background(), db, &changedPath), ErrAuthorizationMetadataInUse)
	require.ErrorIs(t, ValidateAPIMetadataDelete(context.Background(), db, registry.ID), ErrAuthorizationMetadataInUse)

	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Delete(&child).Error)
	require.NoError(t, db.Create(&models.CasbinRule{
		PType: "p", V0: "role-a", V1: adminpkg.APIAccessType.String(), V2: registry.Path, V3: registry.Method,
	}).Error)
	changedMethod := registry
	changedMethod.Method = "POST"
	require.ErrorIs(t, ValidateAPIMetadataUpdate(context.Background(), db, &changedMethod), ErrAuthorizationMetadataInUse)
	require.ErrorIs(t, ValidateAPIMetadataDelete(context.Background(), db, registry.ID), ErrAuthorizationMetadataInUse)

	require.NoError(t, db.Where("v1 = ? AND v2 = ?", adminpkg.APIAccessType.String(), registry.Path).
		Delete(&models.CasbinRule{}).Error)
	require.NoError(t, ValidateAPIMetadataUpdate(context.Background(), db, &changedPath))
	require.NoError(t, ValidateAPIMetadataDelete(context.Background(), db, registry.ID))
	require.Error(t, ValidateAPIMetadataDelete(context.Background(), db, "batch"))

	var persisted models.API
	require.NoError(t, db.Where("id = ?", registry.ID).Take(&persisted).Error)
	require.Equal(t, registry.Path, persisted.Path, "rejected guard calls must not mutate registry metadata")
}

func TestAuthorizationMenuCreateDoesNotGrantProvisioningDefaultRole(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	defaultRole := models.Role{Name: "default", Status: bootenum.Enabled}
	defaultRole.ID = "default-role"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&defaultRole).Error)
	require.NoError(t, db.Exec("UPDATE mss_boot_roles SET `default` = ? WHERE id = ?", true, defaultRole.ID).Error)
	menu := authorizationTestMenu("menu-new", "/new", adminpkg.MenuAccessType, "")
	require.NoError(t, db.Create(&menu).Error)
	var policyCount int64
	require.NoError(t, db.Model(&models.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ?",
		"p", defaultRole.ID, adminpkg.MenuAccessType.String(), menu.Path,
	).Count(&policyCount).Error)
	require.Zero(t, policyCount)
	var revisionCount int64
	require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)
}

func TestAuthorizationDeleteRoleRejectsManagedPolicyAndUserReferences(t *testing.T) {
	newRole := func(t *testing.T, db *gorm.DB, roleID string) models.Role {
		t.Helper()
		role := models.Role{Name: roleID, Status: bootenum.Enabled}
		role.ID = roleID
		require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&role).Error)
		return role
	}

	t.Run("managed role", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		role := newRole(t, db, "root-role")
		require.NoError(t, db.Exec("UPDATE mss_boot_roles SET root = ? WHERE id = ?", true, role.ID).Error)
		err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			DeleteRole(context.Background(), db, role.ID)
		require.ErrorIs(t, err, ErrAuthorizationManagedRole)
	})

	t.Run("Casbin policy", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		role := newRole(t, db, "policy-role")
		require.NoError(t, db.Create(&models.CasbinRule{
			PType: "p", V0: role.ID, V1: adminpkg.MenuAccessType.String(), V2: "/menu", V3: "GET",
		}).Error)
		err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			DeleteRole(context.Background(), db, role.ID)
		require.ErrorIs(t, err, ErrAuthorizationRolePolicies)
		var count int64
		require.NoError(t, db.Model(&models.Role{}).Where("id = ?", role.ID).Count(&count).Error)
		require.Equal(t, int64(1), count)
	})

	t.Run("user assignment", func(t *testing.T) {
		db := setupAuthorizationServiceTest(t)
		role := newRole(t, db, "user-role")
		require.NoError(t, db.Exec(
			"INSERT INTO mss_boot_users (id, role_id) VALUES (?, ?)", "user-a", role.ID,
		).Error)
		err := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil }).
			DeleteRole(context.Background(), db, role.ID)
		require.ErrorIs(t, err, ErrAuthorizationRoleUsers)
	})
}

func TestAuthorizationDeleteRoleDeletesUnreferencedOrdinaryRole(t *testing.T) {
	db := setupAuthorizationServiceTest(t)
	role := models.Role{Name: "ordinary", Status: bootenum.Enabled}
	role.ID = "ordinary-role"
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&role).Error)
	svc := newAuthorizationPolicyService(func() error { return nil }, func() error { return nil })
	require.NoError(t, svc.DeleteRole(context.Background(), db, role.ID))
	var activeCount int64
	require.NoError(t, db.Model(&models.Role{}).Where("id = ?", role.ID).Count(&activeCount).Error)
	require.Zero(t, activeCount)
	var totalCount int64
	require.NoError(t, db.Unscoped().Model(&models.Role{}).Where("id = ?", role.ID).Count(&totalCount).Error)
	require.Equal(t, int64(1), totalCount)
	require.ErrorIs(t, svc.DeleteRole(context.Background(), db, role.ID), ErrAuthorizationRoleNotFound)
}
