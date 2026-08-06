package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"runtime"
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260806200000RemoveRuntimeDeveloperTools)
}

type retiredRuntimeRoute struct {
	Path   string
	Method string
}

type runtimeDeveloperToolsRemovalReport struct {
	RemovedMenus            int64
	RemovedAPIs             int64
	RemovedPolicies         int64
	ReparentedMenus         int64
	CopiedPolicies          int64
	CopiedLocaleDefinitions int64
}

type menuLocaleRemap struct {
	OldPrefix string
	NewPrefix string
}

var invalidateRuntimeToolsLanguageCache = func(ctx context.Context, names ...string) error {
	return pkg.InvalidateLanguageCache(ctx, center.GetCache(), names...)
}

var retiredRuntimeRoutes = []retiredRuntimeRoute{
	// The legacy Virtual controller mounted its dynamic model key directly
	// below /admin/api. SaveAPI normalized both Gin parameters to "*" in the
	// API inventory, while some Casbin rows retained the parameter form.
	{Path: "/admin/api/:key", Method: "GET"},
	{Path: "/admin/api/:key", Method: "POST"},
	{Path: "/admin/api/:key/:id", Method: "GET"},
	{Path: "/admin/api/:key/:id", Method: "PUT"},
	{Path: "/admin/api/:key/:id", Method: "DELETE"},
	{Path: "/admin/api/documentation/:key", Method: "GET"},
	{Path: "/admin/api/models", Method: "GET"},
	{Path: "/admin/api/models", Method: "POST"},
	{Path: "/admin/api/models/:id", Method: "GET"},
	{Path: "/admin/api/models/:id", Method: "PUT"},
	{Path: "/admin/api/models/:id", Method: "DELETE"},
	{Path: "/admin/api/fields", Method: "GET"},
	{Path: "/admin/api/fields", Method: "POST"},
	{Path: "/admin/api/fields/:id", Method: "GET"},
	{Path: "/admin/api/fields/:id", Method: "PUT"},
	{Path: "/admin/api/fields/:id", Method: "DELETE"},
	{Path: "/admin/api/model/generate-data", Method: "PUT"},
	{Path: "/admin/api/template/get-branches", Method: "GET"},
	{Path: "/admin/api/template/get-path", Method: "GET"},
	{Path: "/admin/api/template/get-params", Method: "GET"},
	{Path: "/admin/api/template/generate", Method: "POST"},
	{Path: "/admin/api/github/get-login-url", Method: "GET"},
}

type retiredPolicyTuple struct {
	AccessType string
	Path       string
	Method     string
}

func _20260806200000RemoveRuntimeDeveloperTools(db *gorm.DB, version string) error {
	report, err := removeRuntimeDeveloperToolsWithReport(db, version)
	if err == nil {
		slog.Info("removed retired Admin runtime developer tools",
			"removedMenus", report.RemovedMenus,
			"removedAPIs", report.RemovedAPIs,
			"removedPolicies", report.RemovedPolicies,
			"reparentedMenus", report.ReparentedMenus,
			"copiedPolicies", report.CopiedPolicies,
			"copiedLocaleDefinitions", report.CopiedLocaleDefinitions,
		)
	}
	return err
}

func removeRuntimeDeveloperToolsWithReport(db *gorm.DB, version string) (runtimeDeveloperToolsRemovalReport, error) {
	report := runtimeDeveloperToolsRemovalReport{}
	changedLanguages := make([]string, 0)
	err := db.Transaction(func(tx *gorm.DB) error {
		var menus []models.Menu
		if err := tx.Unscoped().Find(&menus).Error; err != nil {
			return fmt.Errorf("remove runtime developer tools: load menu metadata: %w", err)
		}

		retiredIDs := collectRetiredRuntimeMenuIDs(menus)
		policyTuples := collectRetiredRuntimePolicyTuples(menus, retiredIDs)

		// /develop is a built-in container, but downstream applications may have
		// attached unrelated top-level extensions to it. Preserve those nodes by
		// moving direct survivors to the retired directory's parent before the
		// directory itself is hard-deleted.
		localeRemaps := make([]menuLocaleRemap, 0)
		for i := range menus {
			if menus[i].Path != "/develop" {
				continue
			}
			develop := menus[i]
			parentLocalePrefix, canRemapLocale := menuLocaleParentPrefix(menus, develop.ParentID)
			for j := range menus {
				child := menus[j]
				if child.ParentID != develop.ID || retiredIDs[child.ID] {
					continue
				}

				updates := map[string]any{"parent_id": develop.ParentID}
				if preservedPath, changed := preservedDevelopChildPath(develop.Path, child); changed {
					copied, err := copyMenuPoliciesForPath(tx, child.Type.String(), child.Path, preservedPath)
					if err != nil {
						return fmt.Errorf("remove runtime developer tools: copy menu policies for %q: %w", child.Path, err)
					}
					report.CopiedPolicies += copied
					updates["path"] = preservedPath
					if child.Permission == child.Path {
						updates["permission"] = preservedPath
					}
				}
				if remap, ok := developMenuLocaleRemap(child.Name, parentLocalePrefix); ok && canRemapLocale {
					localeRemaps = append(localeRemaps, remap)
				}
				result := tx.Unscoped().Model(&models.Menu{}).
					Where("id = ?", child.ID).
					Updates(updates)
				if result.Error != nil {
					return fmt.Errorf("remove runtime developer tools: reparent menu %q: %w", child.Path, result.Error)
				}
				report.ReparentedMenus += result.RowsAffected
			}
			retiredIDs[develop.ID] = true
			policyTuples[menuPolicyTuple(develop)] = struct{}{}
		}
		copiedLocaleDefinitions, transactionChangedLanguages, err := copyDevelopMenuLocaleDefinitions(tx, localeRemaps)
		if err != nil {
			return fmt.Errorf("remove runtime developer tools: copy preserved menu locale definitions: %w", err)
		}
		report.CopiedLocaleDefinitions += copiedLocaleDefinitions
		changedLanguages = append(changedLanguages, transactionChangedLanguages...)

		for _, route := range retiredRuntimeRoutes {
			addRetiredAPIPolicyTuples(policyTuples, route)
		}
		for tuple := range policyTuples {
			result := tx.Where(
				"ptype = ? AND v1 = ? AND v2 = ? AND v3 = ?",
				"p", tuple.AccessType, tuple.Path, tuple.Method,
			).Delete(&models.CasbinRule{})
			if result.Error != nil {
				return fmt.Errorf("remove runtime developer tools: delete policy %s %s %s: %w", tuple.AccessType, tuple.Method, tuple.Path, result.Error)
			}
			report.RemovedPolicies += result.RowsAffected
		}

		inventoryRoutes := make(map[retiredRuntimeRoute]struct{}, len(retiredRuntimeRoutes)+len(retiredIDs))
		for _, route := range retiredRuntimeRoutes {
			inventoryRoutes[retiredRuntimeRoute{Path: normalizeRouteInventoryPath(route.Path), Method: normalizedMethod(route.Method)}] = struct{}{}
		}
		for i := range menus {
			if !retiredIDs[menus[i].ID] || menus[i].Type != pkg.APIAccessType {
				continue
			}
			inventoryRoutes[retiredRuntimeRoute{
				Path:   normalizeRouteInventoryPath(menus[i].Path),
				Method: normalizedMethod(menus[i].Method),
			}] = struct{}{}
		}
		for route := range inventoryRoutes {
			result := tx.Unscoped().Where("path = ? AND method = ?", route.Path, route.Method).Delete(&models.API{})
			if result.Error != nil {
				return fmt.Errorf("remove runtime developer tools: delete API inventory %s %s: %w", route.Method, route.Path, result.Error)
			}
			report.RemovedAPIs += result.RowsAffected
		}

		ids := make([]string, 0, len(retiredIDs))
		for id := range retiredIDs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			result := tx.Session(&gorm.Session{SkipHooks: true}).Unscoped().Where("id IN ?", ids).Delete(&models.Menu{})
			if result.Error != nil {
				return fmt.Errorf("remove runtime developer tools: delete menu metadata: %w", result.Error)
			}
			report.RemovedMenus += result.RowsAffected
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
	if err != nil {
		return runtimeDeveloperToolsRemovalReport{}, err
	}
	// Cache invalidation must happen after the transaction commits. Otherwise a
	// concurrent Profile request can refill the old snapshot from the database
	// between cache deletion and commit, leaving stale locale data indefinitely.
	if err := invalidateRuntimeToolsLanguageCache(context.Background(), changedLanguages...); err != nil {
		return report, markRuntimeToolsRemovalRetryable(db, version,
			fmt.Errorf("remove runtime developer tools: invalidate language cache: %w", err))
	}

	if gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			return report, markRuntimeToolsRemovalRetryable(db, version,
				fmt.Errorf("remove runtime developer tools: reload Casbin policy: %w", err))
		}
	}
	return report, nil
}

func markRuntimeToolsRemovalRetryable(db *gorm.DB, version string, cause error) error {
	cleanupErr := db.Where("version = ?", version).Delete(&migrationmodels.Migration{}).Error
	if cleanupErr == nil {
		return cause
	}
	return errors.Join(cause, fmt.Errorf("remove runtime developer tools: clear migration version for retry: %w", cleanupErr))
}

func collectRetiredRuntimeMenuIDs(menus []models.Menu) map[string]bool {
	retired := make(map[string]bool)
	for i := range menus {
		menu := menus[i]
		if isRetiredRuntimeProductPath(menu.Path) || isRetiredRuntimeRoute(menu.Path, menu.Method) {
			retired[menu.ID] = true
		}
	}

	for changed := true; changed; {
		changed = false
		for i := range menus {
			menu := menus[i]
			if retired[menu.ID] || !retired[menu.ParentID] {
				continue
			}
			retired[menu.ID] = true
			changed = true
		}
	}
	return retired
}

func isRetiredRuntimeProductPath(path string) bool {
	return path == "/model" || path == "/generator" || path == "/field" ||
		strings.HasPrefix(path, "/field/") || path == "/virtual" || strings.HasPrefix(path, "/virtual/")
}

func isRetiredRuntimeRoute(path, method string) bool {
	method = normalizedMethod(method)
	for _, route := range retiredRuntimeRoutes {
		if (path == route.Path || normalizeRouteInventoryPath(path) == normalizeRouteInventoryPath(route.Path)) &&
			method == normalizedMethod(route.Method) {
			return true
		}
	}
	return false
}

func collectRetiredRuntimePolicyTuples(menus []models.Menu, retiredIDs map[string]bool) map[retiredPolicyTuple]struct{} {
	tuples := make(map[retiredPolicyTuple]struct{})
	for i := range menus {
		if retiredIDs[menus[i].ID] {
			tuples[menuPolicyTuple(menus[i])] = struct{}{}
			if menus[i].Type == pkg.APIAccessType {
				addRetiredAPIPolicyTuples(tuples, retiredRuntimeRoute{Path: menus[i].Path, Method: menus[i].Method})
			}
		}
	}
	return tuples
}

func menuPolicyTuple(menu models.Menu) retiredPolicyTuple {
	return retiredPolicyTuple{
		AccessType: menu.Type.String(),
		Path:       menu.Path,
		Method:     normalizedMethod(menu.Method),
	}
}

func addRetiredAPIPolicyTuples(tuples map[retiredPolicyTuple]struct{}, route retiredRuntimeRoute) {
	method := normalizedMethod(route.Method)
	tuples[retiredPolicyTuple{AccessType: pkg.APIAccessType.String(), Path: route.Path, Method: method}] = struct{}{}
	normalizedPath := normalizeRouteInventoryPath(route.Path)
	if normalizedPath != route.Path {
		tuples[retiredPolicyTuple{AccessType: pkg.APIAccessType.String(), Path: normalizedPath, Method: method}] = struct{}{}
	}
}

func normalizedMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return "GET"
	}
	return method
}

func normalizeRouteInventoryPath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		if strings.HasPrefix(parts[i], ":") || strings.HasPrefix(parts[i], "*") {
			parts[i] = "*"
		}
	}
	return strings.Join(parts, "/")
}

func preservedDevelopChildPath(developPath string, child models.Menu) (string, bool) {
	if child.Type != pkg.DirectoryAccessType && child.Type != pkg.MenuAccessType {
		return child.Path, false
	}
	if child.Path == "" || strings.HasPrefix(child.Path, "/") || strings.HasPrefix(child.Path, "#") {
		return child.Path, false
	}
	parsed, err := url.Parse(child.Path)
	if err == nil && (parsed.IsAbs() || parsed.Host != "") {
		return child.Path, false
	}
	base, err := url.Parse(strings.TrimRight(developPath, "/") + "/")
	if err != nil || parsed == nil {
		return strings.TrimRight(developPath, "/") + "/" + child.Path, true
	}
	return base.ResolveReference(parsed).String(), true
}

func copyMenuPoliciesForPath(tx *gorm.DB, accessType, oldPath, newPath string) (int64, error) {
	if oldPath == newPath {
		return 0, nil
	}
	var sourceRules []models.CasbinRule
	if err := tx.Where("ptype = ? AND v1 = ? AND v2 = ?", "p", accessType, oldPath).
		Find(&sourceRules).Error; err != nil {
		return 0, err
	}

	var copied int64
	for i := range sourceRules {
		source := sourceRules[i]
		var existing int64
		if err := tx.Model(&models.CasbinRule{}).Where(
			"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ? AND v4 = ? AND v5 = ?",
			source.PType, source.V0, source.V1, newPath, source.V3, source.V4, source.V5,
		).Count(&existing).Error; err != nil {
			return copied, err
		}
		if existing > 0 {
			continue
		}
		clone := source
		clone.ID = 0
		clone.V2 = newPath
		if err := tx.Create(&clone).Error; err != nil {
			return copied, err
		}
		copied++
	}
	return copied, nil
}

func developMenuLocaleRemap(name, parentPrefix string) (menuLocaleRemap, bool) {
	name, ok := menuLocaleSegment(name)
	if !ok {
		return menuLocaleRemap{}, false
	}
	newPrefix := name
	oldPrefix := "develop." + name
	if parentPrefix != "" {
		newPrefix = parentPrefix + "." + name
		oldPrefix = parentPrefix + ".develop." + name
	}
	return menuLocaleRemap{OldPrefix: oldPrefix, NewPrefix: newPrefix}, true
}

func menuLocaleParentPrefix(menus []models.Menu, parentID string) (string, bool) {
	if parentID == "" {
		return "", true
	}
	byID := make(map[string]models.Menu, len(menus))
	for i := range menus {
		byID[menus[i].ID] = menus[i]
	}

	seen := make(map[string]struct{})
	segments := make([]string, 0)
	for parentID != "" {
		if _, exists := seen[parentID]; exists {
			return "", false
		}
		seen[parentID] = struct{}{}
		parent, exists := byID[parentID]
		if !exists {
			return "", false
		}
		segment, ok := menuLocaleSegment(parent.Name)
		if !ok {
			return "", false
		}
		segments = append(segments, segment)
		parentID = parent.ParentID
	}
	for left, right := 0, len(segments)-1; left < right; left, right = left+1, right-1 {
		segments[left], segments[right] = segments[right], segments[left]
	}
	return strings.Join(segments, "."), true
}

func menuLocaleSegment(name string) (string, bool) {
	name = strings.TrimPrefix(name, "menu.")
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	if name == "" {
		return "", false
	}
	return name, true
}

func copyDevelopMenuLocaleDefinitions(tx *gorm.DB, remaps []menuLocaleRemap) (int64, []string, error) {
	if len(remaps) == 0 {
		return 0, nil, nil
	}
	uniqueRemaps := make(map[menuLocaleRemap]struct{}, len(remaps))
	for _, remap := range remaps {
		uniqueRemaps[remap] = struct{}{}
	}
	orderedRemaps := make([]menuLocaleRemap, 0, len(uniqueRemaps))
	for remap := range uniqueRemaps {
		orderedRemaps = append(orderedRemaps, remap)
	}
	sort.Slice(orderedRemaps, func(i, j int) bool {
		if orderedRemaps[i].OldPrefix == orderedRemaps[j].OldPrefix {
			return orderedRemaps[i].NewPrefix < orderedRemaps[j].NewPrefix
		}
		return orderedRemaps[i].OldPrefix < orderedRemaps[j].OldPrefix
	})

	var languages []models.Language
	if err := tx.Find(&languages).Error; err != nil {
		return 0, nil, err
	}
	var copied int64
	changedLanguages := make([]string, 0)
	for i := range languages {
		if languages[i].Defines == nil {
			continue
		}
		defines := *languages[i].Defines
		existing := make(map[string]struct{}, len(defines))
		for _, definition := range defines {
			if definition != nil {
				existing[definition.Group+"\x00"+definition.Key] = struct{}{}
			}
		}

		changed := false
		for _, definition := range defines {
			if definition == nil || definition.Group != "menu" {
				continue
			}
			for _, remap := range orderedRemaps {
				if definition.Key != remap.OldPrefix && !strings.HasPrefix(definition.Key, remap.OldPrefix+".") {
					continue
				}
				newKey := remap.NewPrefix + strings.TrimPrefix(definition.Key, remap.OldPrefix)
				indexKey := definition.Group + "\x00" + newKey
				if _, ok := existing[indexKey]; ok {
					continue
				}
				clone := *definition
				clone.ID = pkg.SimpleID()
				clone.Key = newKey
				defines = append(defines, &clone)
				existing[indexKey] = struct{}{}
				changed = true
				copied++
			}
		}
		if !changed {
			continue
		}
		languages[i].Defines = &defines
		if err := tx.Session(&gorm.Session{SkipHooks: true}).Model(&languages[i]).
			Update("defines", languages[i].Defines).Error; err != nil {
			return copied, changedLanguages, err
		}
		changedLanguages = append(changedLanguages, languages[i].Name)
	}
	return copied, changedLanguages, nil
}
