package system

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	cacheconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const runtimeDeveloperToolsRemovalTestVersion = "2026080620000"

func TestPreservedDevelopChildPathKeepsEffectiveRoute(t *testing.T) {
	tests := []struct {
		name        string
		child       models.Menu
		want        string
		wantChanged bool
	}{
		{name: "relative", child: models.Menu{Path: "reports", Type: pkg.MenuAccessType}, want: "/develop/reports", wantChanged: true},
		{name: "relative dot segment", child: models.Menu{Path: "../audit", Type: pkg.MenuAccessType}, want: "/audit", wantChanged: true},
		{name: "relative query", child: models.Menu{Path: "reports?range=week", Type: pkg.MenuAccessType}, want: "/develop/reports?range=week", wantChanged: true},
		{name: "absolute", child: models.Menu{Path: "/reports", Type: pkg.MenuAccessType}, want: "/reports"},
		{name: "external", child: models.Menu{Path: "https://example.com/reports", Type: pkg.MenuAccessType}, want: "https://example.com/reports"},
		{name: "hash", child: models.Menu{Path: "#reports", Type: pkg.MenuAccessType}, want: "#reports"},
		{name: "component", child: models.Menu{Path: "reports", Type: pkg.ComponentAccessType}, want: "reports"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed := preservedDevelopChildPath("/develop", test.child)
			if got != test.want || changed != test.wantChanged {
				t.Fatalf("preservedDevelopChildPath() = (%q, %v), want (%q, %v)", got, changed, test.want, test.wantChanged)
			}
		})
	}
}

func TestRuntimeDeveloperToolsRemovalPreservesDataAndUnrelatedMetadata(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)

	createRemovalMenu(t, db, "develop", "/develop", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "model", "/model", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "model-component", "/model/generate-data", "model", pkg.ComponentAccessType, "GET", false)
	createRemovalMenu(t, db, "model-api", "/admin/api/model/generate-data", "model-component", pkg.APIAccessType, "PUT", false)
	createRemovalMenu(t, db, "orphan-model-api", "/admin/api/models/*", "", pkg.APIAccessType, "PUT", false)
	createRemovalMenu(t, db, "generator", "/generator", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "virtual", "/virtual/demo", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "virtual-api", "/admin/api/demo/:id", "virtual", pkg.APIAccessType, "DELETE", true)
	createRemovalMenu(t, db, "custom", "/custom", "", pkg.MenuAccessType, "GET", false)

	createRemovalAPI(t, db, "model-inventory", "/admin/api/models/*", "PUT")
	createRemovalAPI(t, db, "virtual-inventory", "/admin/api/demo/*", "DELETE")
	createRemovalAPI(t, db, "virtual-search-inventory", "/admin/api/*", "GET")
	createRemovalAPI(t, db, "virtual-create-inventory", "/admin/api/*", "POST")
	createRemovalAPI(t, db, "virtual-get-inventory", "/admin/api/*/*", "GET")
	createRemovalAPI(t, db, "virtual-update-inventory", "/admin/api/*/*", "PUT")
	createRemovalAPI(t, db, "virtual-delete-inventory", "/admin/api/*/*", "DELETE")
	createRemovalAPI(t, db, "virtual-documentation-inventory", "/admin/api/documentation/*", "GET")
	createRemovalAPI(t, db, "template-inventory", "/admin/api/template/generate", "POST")
	createRemovalAPI(t, db, "same-path-other-method", "/admin/api/models", "PATCH")
	createRemovalAPI(t, db, "custom-inventory", "/admin/api/models-extra", "GET")

	createRemovalPolicy(t, db, "role-retired", pkg.MenuAccessType.String(), "/model", "GET")
	createRemovalPolicy(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/model/generate-data", "PUT")
	createRemovalPolicy(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/demo/*", "DELETE")
	createRemovalPolicy(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/:key", "GET")
	createRemovalPolicy(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/*/*", "PUT")
	createRemovalPolicy(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/documentation/*", "GET")
	createRemovalPolicy(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/template/generate", "POST")
	createRemovalPolicy(t, db, "role-safe", pkg.MenuAccessType.String(), "/custom", "GET")
	createRemovalPolicy(t, db, "role-safe", pkg.APIAccessType.String(), "/admin/api/models-extra", "GET")

	createLegacyRuntimeData(t, db)

	report, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("remove runtime developer tools: %v", err)
	}
	if report.RemovedMenus == 0 || report.RemovedAPIs == 0 || report.RemovedPolicies == 0 {
		t.Fatalf("incomplete removal report: %+v", report)
	}
	if report.ReparentedMenus != 0 {
		t.Fatalf("reparented menus = %d, want 0", report.ReparentedMenus)
	}

	assertRemovalMenuMissing(t, db, "develop")
	assertRemovalMenuMissing(t, db, "model")
	assertRemovalMenuMissing(t, db, "model-component")
	assertRemovalMenuMissing(t, db, "model-api")
	assertRemovalMenuState(t, db, "orphan-model-api", "", "/admin/api/models/*", "/admin/api/models/*")
	assertRemovalMenuMissing(t, db, "generator")
	assertRemovalMenuMissing(t, db, "virtual")
	assertRemovalMenuMissing(t, db, "virtual-api")
	assertRemovalMenuPresent(t, db, "custom", "")

	assertRemovalAPIMissing(t, db, "/admin/api/models/*", "PUT")
	assertRemovalAPIMissing(t, db, "/admin/api/demo/*", "DELETE")
	assertRemovalAPIMissing(t, db, "/admin/api/*", "GET")
	assertRemovalAPIMissing(t, db, "/admin/api/*", "POST")
	assertRemovalAPIMissing(t, db, "/admin/api/*/*", "GET")
	assertRemovalAPIMissing(t, db, "/admin/api/*/*", "PUT")
	assertRemovalAPIMissing(t, db, "/admin/api/*/*", "DELETE")
	assertRemovalAPIMissing(t, db, "/admin/api/documentation/*", "GET")
	assertRemovalAPIMissing(t, db, "/admin/api/template/generate", "POST")
	assertRemovalAPIPresent(t, db, "/admin/api/models", "PATCH")
	assertRemovalAPIPresent(t, db, "/admin/api/models-extra", "GET")

	assertRemovalPolicyMissing(t, db, "role-retired", pkg.MenuAccessType.String(), "/model", "GET")
	assertRemovalPolicyMissing(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/demo/*", "DELETE")
	assertRemovalPolicyMissing(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/:key", "GET")
	assertRemovalPolicyMissing(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/*/*", "PUT")
	assertRemovalPolicyMissing(t, db, "role-retired", pkg.APIAccessType.String(), "/admin/api/documentation/*", "GET")
	assertRemovalPolicyPresent(t, db, "role-safe", pkg.MenuAccessType.String(), "/custom", "GET")
	assertRemovalPolicyPresent(t, db, "role-safe", pkg.APIAccessType.String(), "/admin/api/models-extra", "GET")
	assertLegacyRuntimeDataPreserved(t, db)
	assertRuntimeToolsRemovalVersionCount(t, db, 1)

	secondReport, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("rerun runtime developer tools removal: %v", err)
	}
	if secondReport != (runtimeDeveloperToolsRemovalReport{}) {
		t.Fatalf("second removal report = %+v, want no changes", secondReport)
	}
	assertRuntimeToolsRemovalVersionCount(t, db, 1)
}

func TestRuntimeDeveloperToolsRemovalReparentsUnrelatedDevelopChildren(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)
	createRemovalMenu(t, db, "foundation", "/foundation", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "develop", "/develop", "foundation", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "model", "/model", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-tool", "/custom-tool", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "relative-tool", "reports", "develop", pkg.MenuAccessType, "GET", false)
	if err := db.Model(&models.Menu{}).Where("id = ?", "relative-tool").
		Update("name", "menu.develop.reports").Error; err != nil {
		t.Fatalf("set relative menu name: %v", err)
	}
	createRemovalPolicy(t, db, "role-custom", pkg.MenuAccessType.String(), "/custom-tool", "GET")
	createRemovalPolicy(t, db, "role-relative", pkg.MenuAccessType.String(), "reports", "GET")
	createRemovalPolicy(t, db, "role-existing", pkg.MenuAccessType.String(), "reports", "GET")
	createRemovalPolicy(t, db, "role-existing", pkg.MenuAccessType.String(), "/develop/reports", "GET")
	createRemovalPolicy(t, db, "role-retired", pkg.DirectoryAccessType.String(), "/develop", "GET")
	createRemovalLanguage(t, db, "language-en", "en-US",
		&models.LanguageDefine{ID: "old-base", Group: "menu", Key: "foundation.develop.reports", Value: "Old reports"},
		&models.LanguageDefine{ID: "old-control", Group: "menu", Key: "foundation.develop.reports.control", Value: "Manage reports"},
		&models.LanguageDefine{ID: "existing-base", Group: "menu", Key: "foundation.reports", Value: "Existing reports"},
		&models.LanguageDefine{ID: "unrelated-root", Group: "menu", Key: "reports", Value: "Root reports"},
		&models.LanguageDefine{ID: "unrelated", Group: "menu", Key: "develop.other", Value: "Other tool"},
	)

	report, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("remove runtime developer tools: %v", err)
	}
	if report.ReparentedMenus != 2 {
		t.Fatalf("reparented menus = %d, want 2", report.ReparentedMenus)
	}
	if report.CopiedPolicies != 1 {
		t.Fatalf("copied policies = %d, want 1", report.CopiedPolicies)
	}
	if report.CopiedLocaleDefinitions != 1 {
		t.Fatalf("copied locale definitions = %d, want 1", report.CopiedLocaleDefinitions)
	}
	assertRemovalMenuMissing(t, db, "develop")
	assertRemovalMenuMissing(t, db, "model")
	assertRemovalMenuState(t, db, "custom-tool", "foundation", "/custom-tool", "/custom-tool")
	assertRemovalMenuState(t, db, "relative-tool", "foundation", "/develop/reports", "/develop/reports")
	assertRemovalPolicyPresent(t, db, "role-custom", pkg.MenuAccessType.String(), "/custom-tool", "GET")
	assertRemovalPolicyPresent(t, db, "role-relative", pkg.MenuAccessType.String(), "reports", "GET")
	assertRemovalPolicyPresent(t, db, "role-relative", pkg.MenuAccessType.String(), "/develop/reports", "GET")
	assertRemovalPolicyPresent(t, db, "role-existing", pkg.MenuAccessType.String(), "reports", "GET")
	assertRemovalPolicyPresent(t, db, "role-existing", pkg.MenuAccessType.String(), "/develop/reports", "GET")
	assertRemovalPolicyMissing(t, db, "role-retired", pkg.DirectoryAccessType.String(), "/develop", "GET")
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "foundation.develop.reports", "Old reports", 1)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "foundation.develop.reports.control", "Manage reports", 1)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "foundation.reports", "Existing reports", 1)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "foundation.reports.control", "Manage reports", 1)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "reports", "Root reports", 1)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "reports.control", "", 0)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "develop.other", "Other tool", 1)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "other", "", 0)

	secondReport, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("rerun runtime developer tools removal: %v", err)
	}
	if secondReport != (runtimeDeveloperToolsRemovalReport{}) {
		t.Fatalf("second removal report = %+v, want no changes", secondReport)
	}
	assertRemovalPolicyPresent(t, db, "role-relative", pkg.MenuAccessType.String(), "/develop/reports", "GET")
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "foundation.reports.control", "Manage reports", 1)
}

func TestRuntimeDeveloperToolsRemovalPreservesSamePathsOutsideDevelopTree(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)
	createRemovalMenu(t, db, "develop", "/develop", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "built-in-model", "/model", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-root", "/custom", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-model", "/model", "custom-root", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-field", "/field", "custom-model", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-virtual", "/virtual", "custom-field", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-generator", "/generator", "custom-virtual", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-leaf", "/custom/leaf", "custom-generator", pkg.ComponentAccessType, "GET", false)

	createRemovalPolicy(t, db, "role-develop", pkg.DirectoryAccessType.String(), "/develop", "GET")
	createRemovalPolicy(t, db, "role-custom-model", pkg.MenuAccessType.String(), "/model", "GET")
	createRemovalPolicy(t, db, "role-custom-field", pkg.MenuAccessType.String(), "/field", "GET")
	createRemovalPolicy(t, db, "role-custom-virtual", pkg.MenuAccessType.String(), "/virtual", "GET")
	createRemovalPolicy(t, db, "role-custom-generator", pkg.MenuAccessType.String(), "/generator", "GET")
	createRemovalPolicy(t, db, "role-custom-leaf", pkg.ComponentAccessType.String(), "/custom/leaf", "GET")

	report, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("remove runtime developer tools: %v", err)
	}
	if report.RemovedMenus != 2 {
		t.Fatalf("removed menus = %d, want 2", report.RemovedMenus)
	}

	assertRemovalMenuMissing(t, db, "develop")
	assertRemovalMenuMissing(t, db, "built-in-model")
	assertRemovalMenuState(t, db, "custom-root", "", "/custom", "/custom")
	assertRemovalMenuState(t, db, "custom-model", "custom-root", "/model", "/model")
	assertRemovalMenuState(t, db, "custom-field", "custom-model", "/field", "/field")
	assertRemovalMenuState(t, db, "custom-virtual", "custom-field", "/virtual", "/virtual")
	assertRemovalMenuState(t, db, "custom-generator", "custom-virtual", "/generator", "/generator")
	assertRemovalMenuState(t, db, "custom-leaf", "custom-generator", "/custom/leaf", "/custom/leaf")
	assertRemovalPolicyMissing(t, db, "role-develop", pkg.DirectoryAccessType.String(), "/develop", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-model", pkg.MenuAccessType.String(), "/model", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-field", pkg.MenuAccessType.String(), "/field", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-virtual", pkg.MenuAccessType.String(), "/virtual", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-generator", pkg.MenuAccessType.String(), "/generator", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-leaf", pkg.ComponentAccessType.String(), "/custom/leaf", "GET")
}

func TestRuntimeDeveloperToolsRemovalPreservesRetiredLookingSubtreeOfCustomDevelopChild(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)
	createRemovalMenu(t, db, "develop", "/develop", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "built-in-generator", "/generator", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-tool", "/custom-tool", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-model", "/model", "custom-tool", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "custom-child", "/custom/model-child", "custom-model", pkg.ComponentAccessType, "GET", false)

	createRemovalPolicy(t, db, "role-develop", pkg.DirectoryAccessType.String(), "/develop", "GET")
	createRemovalPolicy(t, db, "role-generator", pkg.MenuAccessType.String(), "/generator", "GET")
	createRemovalPolicy(t, db, "role-custom-tool", pkg.MenuAccessType.String(), "/custom-tool", "GET")
	createRemovalPolicy(t, db, "role-custom-model", pkg.MenuAccessType.String(), "/model", "GET")
	createRemovalPolicy(t, db, "role-custom-child", pkg.ComponentAccessType.String(), "/custom/model-child", "GET")

	report, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("remove runtime developer tools: %v", err)
	}
	if report.RemovedMenus != 2 {
		t.Fatalf("removed menus = %d, want 2", report.RemovedMenus)
	}
	if report.ReparentedMenus != 1 {
		t.Fatalf("reparented menus = %d, want 1", report.ReparentedMenus)
	}

	assertRemovalMenuMissing(t, db, "develop")
	assertRemovalMenuMissing(t, db, "built-in-generator")
	assertRemovalMenuState(t, db, "custom-tool", "", "/custom-tool", "/custom-tool")
	assertRemovalMenuState(t, db, "custom-model", "custom-tool", "/model", "/model")
	assertRemovalMenuState(t, db, "custom-child", "custom-model", "/custom/model-child", "/custom/model-child")
	assertRemovalPolicyMissing(t, db, "role-develop", pkg.DirectoryAccessType.String(), "/develop", "GET")
	assertRemovalPolicyMissing(t, db, "role-generator", pkg.MenuAccessType.String(), "/generator", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-tool", pkg.MenuAccessType.String(), "/custom-tool", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-model", pkg.MenuAccessType.String(), "/model", "GET")
	assertRemovalPolicyPresent(t, db, "role-custom-child", pkg.ComponentAccessType.String(), "/custom/model-child", "GET")
}

func TestRuntimeDeveloperToolsRemovalCopiesRootDevelopChildLocale(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)
	createRemovalMenu(t, db, "develop", "/develop", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "relative-tool", "reports", "develop", pkg.MenuAccessType, "GET", false)
	if err := db.Model(&models.Menu{}).Where("id = ?", "relative-tool").
		Update("name", "menu.develop.reports").Error; err != nil {
		t.Fatalf("set relative menu name: %v", err)
	}
	createRemovalLanguage(t, db, "language-en", "en-US",
		&models.LanguageDefine{ID: "old-base", Group: "menu", Key: "develop.reports", Value: "Reports"},
		&models.LanguageDefine{ID: "old-control", Group: "menu", Key: "develop.reports.control", Value: "Manage reports"},
	)

	report, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("remove runtime developer tools: %v", err)
	}
	if report.CopiedLocaleDefinitions != 2 {
		t.Fatalf("copied locale definitions = %d, want 2", report.CopiedLocaleDefinitions)
	}
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "reports", "Reports", 1)
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "reports.control", "Manage reports", 1)
}

func TestRuntimeDeveloperToolsRemovalInvalidatesCachedLanguageSnapshot(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)
	createRemovalMenu(t, db, "develop", "/develop", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "relative-tool", "reports", "develop", pkg.MenuAccessType, "GET", false)
	if err := db.Model(&models.Menu{}).Where("id = ?", "relative-tool").
		Update("name", "menu.develop.reports").Error; err != nil {
		t.Fatalf("set relative menu name: %v", err)
	}
	createRemovalLanguage(t, db, "language-en", "en-US",
		&models.LanguageDefine{ID: "old-base", Group: "menu", Key: "develop.reports", Value: "Reports"},
	)

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	cache, err := cacheconfig.NewRedis(redisClient, nil)
	if err != nil {
		t.Fatalf("create Redis cache: %v", err)
	}
	previousCache := center.GetCache()
	center.SetCache(cache)
	t.Cleanup(func() {
		center.SetCache(previousCache)
		_ = cache.Close()
	})
	ctx := context.Background()
	staleProfile := pkg.LanguageProfile{
		"en-US": {"menu.develop.reports": "Cached reports"},
		"zh-CN": {"menu.welcome": "Welcome"},
	}
	if stored, err := pkg.StoreLanguageProfileCache(ctx, cache, 0, staleProfile); err != nil || !stored {
		t.Fatalf("seed language profile cache = (%v, %v), want (true, nil)", stored, err)
	}

	actualInvalidator := invalidateRuntimeToolsLanguageCache
	invalidationStarted := make(chan struct{})
	allowInvalidation := make(chan struct{})
	invalidateRuntimeToolsLanguageCache = func(ctx context.Context, names ...string) error {
		close(invalidationStarted)
		<-allowInvalidation
		return actualInvalidator(ctx, names...)
	}
	t.Cleanup(func() { invalidateRuntimeToolsLanguageCache = actualInvalidator })
	reportResult := make(chan runtimeDeveloperToolsRemovalReport, 1)
	errResult := make(chan error, 1)
	go func() {
		report, migrationErr := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
		reportResult <- report
		errResult <- migrationErr
	}()
	select {
	case <-invalidationStarted:
	case <-time.After(time.Second):
		t.Fatal("post-commit language cache invalidation did not start")
	}

	// The invalidator is blocked, so this models a Profile request publishing at
	// the commit boundary. It must already observe the committed definition; its
	// generation-0 snapshot becomes invisible once invalidation advances to 1.
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "reports", "Reports", 1)
	refilledProfile := pkg.LanguageProfile{
		"en-US": {"menu.reports": "Reports"},
		"zh-CN": {"menu.welcome": "Welcome"},
	}
	if stored, err := pkg.StoreLanguageProfileCache(ctx, cache, 0, refilledProfile); err != nil || !stored {
		t.Fatalf("simulate post-commit profile publication = (%v, %v), want (true, nil)", stored, err)
	}
	close(allowInvalidation)
	report := <-reportResult
	err = <-errResult
	if err != nil {
		t.Fatalf("remove runtime developer tools: %v", err)
	}
	if report.CopiedLocaleDefinitions != 1 {
		t.Fatalf("copied locale definitions = %d, want 1", report.CopiedLocaleDefinitions)
	}
	loaded, generation, hit, err := pkg.LoadLanguageProfileCache(ctx, cache)
	if err != nil || hit || generation != 1 || loaded != nil {
		t.Fatalf("load invalidated language profile = (%v, %d, %v, %v)", loaded, generation, hit, err)
	}
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "reports", "Reports", 1)
}

func TestRuntimeDeveloperToolsRemovalFailsOpenWhenLanguageCacheInvalidationFails(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)
	createRemovalMenu(t, db, "develop", "/develop", "", pkg.DirectoryAccessType, "GET", false)
	createRemovalMenu(t, db, "model", "/model", "develop", pkg.MenuAccessType, "GET", false)
	createRemovalMenu(t, db, "relative-tool", "reports", "develop", pkg.MenuAccessType, "GET", false)
	if err := db.Model(&models.Menu{}).Where("id = ?", "relative-tool").
		Update("name", "menu.develop.reports").Error; err != nil {
		t.Fatalf("set relative menu name: %v", err)
	}
	createRemovalPolicy(t, db, "role-retired", pkg.MenuAccessType.String(), "/model", "GET")
	createRemovalLanguage(t, db, "language-en", "en-US",
		&models.LanguageDefine{ID: "old-base", Group: "menu", Key: "develop.reports", Value: "Reports"},
	)

	actualInvalidator := invalidateRuntimeToolsLanguageCache
	invalidationCalls := 0
	invalidateRuntimeToolsLanguageCache = func(context.Context, ...string) error {
		invalidationCalls++
		return errors.New("injected language cache invalidation failure")
	}
	t.Cleanup(func() { invalidateRuntimeToolsLanguageCache = actualInvalidator })

	report, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("remove runtime developer tools with unavailable language cache: %v", err)
	}
	if report.RemovedMenus != 2 || report.ReparentedMenus != 1 || report.CopiedLocaleDefinitions != 1 {
		t.Fatalf("unexpected removal report: %+v", report)
	}
	assertRemovalMenuMissing(t, db, "develop")
	assertRemovalMenuMissing(t, db, "model")
	assertRemovalMenuState(t, db, "relative-tool", "", "/develop/reports", "/develop/reports")
	assertRemovalPolicyMissing(t, db, "role-retired", pkg.MenuAccessType.String(), "/model", "GET")
	assertRemovalLanguageDefine(t, db, "en-US", "menu", "reports", "Reports", 1)
	assertRuntimeToolsRemovalVersionCount(t, db, 1)

	secondReport, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion)
	if err != nil {
		t.Fatalf("rerun runtime developer tools removal with unavailable language cache: %v", err)
	}
	if secondReport != (runtimeDeveloperToolsRemovalReport{}) {
		t.Fatalf("second removal report = %+v, want no changes", secondReport)
	}
	if invalidationCalls != 2 {
		t.Fatalf("language cache invalidation calls = %d, want 2", invalidationCalls)
	}
	assertRuntimeToolsRemovalVersionCount(t, db, 1)
}

func TestRuntimeDeveloperToolsRemovalRollsBackOnInventoryFailure(t *testing.T) {
	db := setupRuntimeDeveloperToolsRemovalTest(t)
	createRemovalMenu(t, db, "model", "/model", "", pkg.MenuAccessType, "GET", false)
	createRemovalPolicy(t, db, "role-retired", pkg.MenuAccessType.String(), "/model", "GET")
	if err := db.Migrator().DropTable(&models.API{}); err != nil {
		t.Fatalf("drop API inventory table: %v", err)
	}

	if _, err := removeRuntimeDeveloperToolsWithReport(db, runtimeDeveloperToolsRemovalTestVersion); err == nil {
		t.Fatal("removal succeeded without the required API inventory table")
	}
	assertRemovalMenuPresent(t, db, "model", "")
	assertRemovalPolicyPresent(t, db, "role-retired", pkg.MenuAccessType.String(), "/model", "GET")
	assertRuntimeToolsRemovalVersionCount(t, db, 0)
}

func setupRuntimeDeveloperToolsRemovalTest(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "runtime-tools-removal.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	previousEnforcer := gormdb.Enforcer
	gormdb.Enforcer = nil
	t.Cleanup(func() { gormdb.Enforcer = previousEnforcer })

	if err := db.AutoMigrate(
		&models.Menu{},
		&models.API{},
		&models.CasbinRule{},
		&models.Language{},
		&migrationmodels.Migration{},
	); err != nil {
		t.Fatalf("migrate removal test schema: %v", err)
	}
	return db
}

func createRemovalMenu(t *testing.T, db *gorm.DB, id, path, parentID string, accessType pkg.AccessType, method string, deleted bool) {
	t.Helper()
	menu := &models.Menu{
		ParentID:   parentID,
		Name:       "fixture." + id,
		Path:       path,
		Method:     method,
		Component:  "./Fixture",
		Type:       accessType,
		Permission: path,
	}
	menu.ID = id
	if deleted {
		menu.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Unscoped().Create(menu).Error; err != nil {
		t.Fatalf("create menu %q: %v", path, err)
	}
}

func createRemovalAPI(t *testing.T, db *gorm.DB, id, path, method string) {
	t.Helper()
	api := &models.API{Name: path, Path: path, Method: method}
	api.ID = id
	if err := db.Create(api).Error; err != nil {
		t.Fatalf("create API inventory %s %s: %v", method, path, err)
	}
}

func createRemovalPolicy(t *testing.T, db *gorm.DB, roleID, accessType, path, method string) {
	t.Helper()
	if err := db.Create(&models.CasbinRule{
		PType: "p",
		V0:    roleID,
		V1:    accessType,
		V2:    path,
		V3:    method,
	}).Error; err != nil {
		t.Fatalf("create policy %s %s %s: %v", accessType, method, path, err)
	}
}

func createRemovalLanguage(
	t *testing.T,
	db *gorm.DB,
	id string,
	name string,
	definitions ...*models.LanguageDefine,
) {
	t.Helper()
	defines := models.LanguageDefines(definitions)
	language := &models.Language{Name: name, Defines: &defines}
	language.ID = id
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(language).Error; err != nil {
		t.Fatalf("create language %q: %v", name, err)
	}
}

func createLegacyRuntimeData(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mss_boot_models (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE mss_boot_fields (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE mss_boot_demo (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
		`INSERT INTO mss_boot_models (id, name) VALUES ('model-1', 'demo')`,
		`INSERT INTO mss_boot_fields (id, name) VALUES ('field-1', 'name')`,
		`INSERT INTO mss_boot_demo (id, name) VALUES ('row-1', 'preserved')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare legacy data with %q: %v", statement, err)
		}
	}
}

func assertLegacyRuntimeDataPreserved(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []string{"mss_boot_models", "mss_boot_fields", "mss_boot_demo"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("legacy table %q was removed", table)
		}
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count preserved table %q: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("preserved table %q count = %d, want 1", table, count)
		}
	}
}

func assertRemovalMenuMissing(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	var count int64
	if err := db.Unscoped().Model(&models.Menu{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count removed menu %q: %v", id, err)
	}
	if count != 0 {
		t.Fatalf("menu %q still exists", id)
	}
}

func assertRemovalMenuPresent(t *testing.T, db *gorm.DB, id, parentID string) {
	t.Helper()
	var menu models.Menu
	if err := db.Unscoped().First(&menu, "id = ?", id).Error; err != nil {
		t.Fatalf("load preserved menu %q: %v", id, err)
	}
	if menu.ParentID != parentID {
		t.Fatalf("menu %q parent = %q, want %q", id, menu.ParentID, parentID)
	}
}

func assertRemovalMenuState(t *testing.T, db *gorm.DB, id, parentID, path, permission string) {
	t.Helper()
	var menu models.Menu
	if err := db.Unscoped().First(&menu, "id = ?", id).Error; err != nil {
		t.Fatalf("load preserved menu %q: %v", id, err)
	}
	if menu.ParentID != parentID || menu.Path != path || menu.Permission != permission {
		t.Fatalf(
			"menu %q state = parent %q path %q permission %q, want parent %q path %q permission %q",
			id, menu.ParentID, menu.Path, menu.Permission, parentID, path, permission,
		)
	}
}

func assertRemovalAPIMissing(t *testing.T, db *gorm.DB, path, method string) {
	t.Helper()
	var count int64
	if err := db.Unscoped().Model(&models.API{}).Where("path = ? AND method = ?", path, method).Count(&count).Error; err != nil {
		t.Fatalf("count removed API %s %s: %v", method, path, err)
	}
	if count != 0 {
		t.Fatalf("API inventory %s %s still exists", method, path)
	}
}

func assertRemovalAPIPresent(t *testing.T, db *gorm.DB, path, method string) {
	t.Helper()
	var count int64
	if err := db.Model(&models.API{}).Where("path = ? AND method = ?", path, method).Count(&count).Error; err != nil {
		t.Fatalf("count preserved API %s %s: %v", method, path, err)
	}
	if count != 1 {
		t.Fatalf("preserved API %s %s count = %d, want 1", method, path, count)
	}
}

func assertRemovalPolicyMissing(t *testing.T, db *gorm.DB, roleID, accessType, path, method string) {
	t.Helper()
	if countRemovalPolicies(t, db, roleID, accessType, path, method) != 0 {
		t.Fatalf("policy %s %s %s for %q still exists", accessType, method, path, roleID)
	}
}

func assertRemovalPolicyPresent(t *testing.T, db *gorm.DB, roleID, accessType, path, method string) {
	t.Helper()
	if count := countRemovalPolicies(t, db, roleID, accessType, path, method); count != 1 {
		t.Fatalf("preserved policy %s %s %s for %q count = %d, want 1", accessType, method, path, roleID, count)
	}
}

func countRemovalPolicies(t *testing.T, db *gorm.DB, roleID, accessType, path, method string) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&models.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p", roleID, accessType, path, method,
	).Count(&count).Error; err != nil {
		t.Fatalf("count policy %s %s %s for %q: %v", accessType, method, path, roleID, err)
	}
	return count
}

func assertRemovalLanguageDefine(
	t *testing.T,
	db *gorm.DB,
	languageName string,
	group string,
	key string,
	wantValue string,
	wantCount int,
) {
	t.Helper()
	var language models.Language
	if err := db.First(&language, "name = ?", languageName).Error; err != nil {
		t.Fatalf("load language %q: %v", languageName, err)
	}
	count := 0
	for _, definition := range *language.Defines {
		if definition == nil || definition.Group != group || definition.Key != key {
			continue
		}
		count++
		if definition.Value != wantValue {
			t.Fatalf(
				"language %q definition %s.%s value = %q, want %q",
				languageName, group, key, definition.Value, wantValue,
			)
		}
	}
	if count != wantCount {
		t.Fatalf(
			"language %q definition %s.%s count = %d, want %d",
			languageName, group, key, count, wantCount,
		)
	}
}

func assertRuntimeToolsRemovalVersionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", runtimeDeveloperToolsRemovalTestVersion).
		Count(&count).Error; err != nil {
		t.Fatalf("count runtime tools removal version: %v", err)
	}
	if count != want {
		t.Fatalf("runtime tools removal version count = %d, want %d", count, want)
	}
}
