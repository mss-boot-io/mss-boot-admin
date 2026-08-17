package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	bootenum "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	runtimeeventbus "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/eventbus"
)

const (
	authorizationRevisionScopeRole     = "role"
	authorizationRevisionScopeGlobal   = "global"
	authorizationRevisionResource      = "authorization"
	authorizationStableReadMaxAttempts = 4
)

var (
	ErrAuthorizationMenuNotFound     = errors.New("authorization menu not found")
	ErrAuthorizationAPIInvalid       = errors.New("authorization API reference is invalid or ambiguous")
	ErrAuthorizationMetadataInUse    = errors.New("authorization metadata is referenced by active policy")
	ErrAuthorizationMetadataConflict = errors.New("authorization metadata permission code is ambiguous")
	ErrAuthorizationRoleNotFound     = errors.New("authorization role not found")
	ErrAuthorizationRoleInactive     = errors.New("authorization role is inactive")
	ErrAuthorizationManagedRole      = errors.New("managed role lifecycle is immutable")
	ErrAuthorizationRolePolicies     = errors.New("role still has Casbin policies")
	ErrAuthorizationRoleUsers        = errors.New("role is still assigned to users")
)

var errAuthorizationSnapshotChanged = errors.New("authorization resource changed during stable read")

// InvalidAuthorizationPathsError reports submitted paths that are not active,
// assignable MENU or COMPONENT metadata. The service never partially applies
// the valid subset.
type InvalidAuthorizationPathsError struct {
	Paths []string
}

func (e *InvalidAuthorizationPathsError) Error() string {
	return fmt.Sprintf("authorization paths are invalid: %v", e.Paths)
}

// AuthorizationRevisionConflictError is returned without mutating policy when
// If-Match names an older role authorization revision.
type AuthorizationRevisionConflictError struct {
	Expected int64
	Actual   int64
	Current  *dto.GetAuthorizeResponse
}

func (e *AuthorizationRevisionConflictError) Error() string {
	return fmt.Sprintf("role authorization revision changed: expected %d, actual %d", e.Expected, e.Actual)
}

// AuthorizationPropagationError means the database transaction committed but
// the local reload or watcher publication did not complete. The durable global
// revision lets every request fail closed and retry reconciliation.
type AuthorizationPropagationError struct {
	Current *dto.GetAuthorizeResponse
	Err     error
}

func (e *AuthorizationPropagationError) Error() string {
	return fmt.Sprintf("authorization policy committed but propagation failed: %v", e.Err)
}

func (e *AuthorizationPropagationError) Unwrap() error { return e.Err }

// AuthorizationPolicyService owns the canonical role-policy resource and the
// process-local durable-revision reconciliation state.
type AuthorizationPolicyService struct {
	reconcileMu sync.Mutex
	initialized bool
	lastDB      *sql.DB
	lastGlobal  int64

	reloadPolicy  func() error
	notifyWatcher func() error

	eventMu  sync.RWMutex
	eventBus runtimeeventbus.EventBus[AuthorizationRevisionEvent]
}

func NewAuthorizationPolicyService() *AuthorizationPolicyService {
	return newAuthorizationPolicyService(gormdb.ReloadPolicy, gormdb.NotifyPolicyWatcher)
}

func newAuthorizationPolicyService(reloadPolicy, notifyWatcher func() error) *AuthorizationPolicyService {
	return &AuthorizationPolicyService{
		reloadPolicy:  reloadPolicy,
		notifyWatcher: notifyWatcher,
	}
}

// AuthorizationPolicies is the shared runtime policy resource. Middleware and
// mutation handlers intentionally use the same last-seen revision state.
var AuthorizationPolicies = NewAuthorizationPolicyService()

func roleAuthorizationRevisionKey(roleID string) configRevisionKey {
	return configRevisionKey{
		scope:    authorizationRevisionScopeRole,
		ownerID:  strings.TrimSpace(roleID),
		resource: authorizationRevisionResource,
	}
}

func globalAuthorizationRevisionKey() configRevisionKey {
	return configRevisionKey{
		scope:    authorizationRevisionScopeGlobal,
		ownerID:  "",
		resource: authorizationRevisionResource,
	}
}

// EnsureAuthorizationGlobalRevision creates the durable reconciliation row if
// an upgraded installation has not mutated authorization yet, then returns its
// authoritative value.
func EnsureAuthorizationGlobalRevision(ctx context.Context, db *gorm.DB) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("authorization database is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := globalAuthorizationRevisionKey()
	row := &models.ConfigRevision{
		Scope:    key.scope,
		OwnerID:  key.ownerID,
		Resource: key.resource,
		Revision: 0,
	}
	db = db.WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return 0, err
	}
	return readConfigRevision(db, key)
}

// EnsureCurrent reconciles the synchronized in-memory enforcer only when the
// durable global authorization revision changes. A before/reload/after loop
// prevents a commit concurrent with LoadPolicy from being marked current.
// Any database or reload error is returned so request middleware can fail
// closed instead of authorizing from an uncertain snapshot.
func (e *AuthorizationPolicyService) EnsureCurrent(ctx context.Context, db *gorm.DB) error {
	if e == nil {
		return fmt.Errorf("authorization policy service is nil")
	}
	if db == nil {
		return fmt.Errorf("authorization database is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("resolve authorization database identity: %w", err)
	}

	e.reconcileMu.Lock()
	defer e.reconcileMu.Unlock()

	current, err := EnsureAuthorizationGlobalRevision(ctx, db)
	if err != nil {
		return fmt.Errorf("ensure authorization global revision: %w", err)
	}
	if e.initialized && e.lastDB == sqlDB && e.lastGlobal == current {
		return nil
	}

	for attempt := 0; attempt < authorizationStableReadMaxAttempts; attempt++ {
		before, err := readConfigRevision(db.WithContext(ctx), globalAuthorizationRevisionKey())
		if err != nil {
			return fmt.Errorf("read authorization revision before reload: %w", err)
		}
		if e.reloadPolicy == nil {
			return fmt.Errorf("authorization policy reload is not configured")
		}
		if err := e.reloadPolicy(); err != nil {
			return fmt.Errorf("reload authorization policy: %w", err)
		}
		after, err := readConfigRevision(db.WithContext(ctx), globalAuthorizationRevisionKey())
		if err != nil {
			return fmt.Errorf("read authorization revision after reload: %w", err)
		}
		if before != after {
			continue
		}
		e.initialized = true
		e.lastDB = sqlDB
		e.lastGlobal = after
		return nil
	}
	return fmt.Errorf("authorization policy changed during %d reload attempts", authorizationStableReadMaxAttempts)
}

func (e *AuthorizationPolicyService) reconcileCommitted(ctx context.Context, db *gorm.DB, revision int64) error {
	e.eventMu.RLock()
	bus := e.eventBus
	e.eventMu.RUnlock()
	if bus != nil {
		if revision <= 0 {
			return fmt.Errorf("authorization committed revision is invalid")
		}
		return bus.Publish(ctx, runtimeeventbus.Event[AuthorizationRevisionEvent]{
			Revision: runtimeeventbus.Revision(revision),
			Payload:  AuthorizationRevisionEvent{},
		})
	}

	// Compatibility fallback for callers that have not installed the D5
	// EventBus runtime yet. Production composition binds eventBus and therefore
	// does not use WorkQueue watcher acknowledgement semantics.
	if err := e.EnsureCurrent(ctx, db); err != nil {
		return err
	}
	if e.notifyWatcher == nil {
		return fmt.Errorf("authorization watcher notification is not configured")
	}
	if err := e.notifyWatcher(); err != nil {
		return fmt.Errorf("notify authorization policy watcher: %w", err)
	}
	return nil
}

// DeleteRole removes an ordinary, unreferenced role under a serializable
// transaction. Managed roles, any residual Casbin policy, and user references
// fail closed. Keeping the durable role revision row preserves monotonic ETag
// history if an external provisioning process ever reuses the identifier.
func (e *AuthorizationPolicyService) DeleteRole(ctx context.Context, db *gorm.DB, roleID string) error {
	if e == nil {
		return fmt.Errorf("authorization policy service is nil")
	}
	if db == nil {
		return fmt.Errorf("authorization database is nil")
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return ErrAuthorizationRoleNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role models.Role
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", roleID).Take(&role).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAuthorizationRoleNotFound
		}
		if err != nil {
			return err
		}
		if role.Root || role.Default {
			return ErrAuthorizationManagedRole
		}
		var policyCount int64
		if err := tx.Model(&models.CasbinRule{}).
			Where("v0 = ? OR (ptype LIKE ? AND v1 = ?)", roleID, "g%", roleID).
			Count(&policyCount).Error; err != nil {
			return err
		}
		if policyCount > 0 {
			return ErrAuthorizationRolePolicies
		}
		var userCount int64
		if err := tx.Table("mss_boot_users").Where("role_id = ?", roleID).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount > 0 {
			return ErrAuthorizationRoleUsers
		}
		return tx.Delete(&role).Error
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

// ReadRole returns one stable database-backed snapshot. It does not use the
// in-memory enforcer because a GET and its ETag must name the same committed
// role revision even while another process is updating policy.
func (e *AuthorizationPolicyService) ReadRole(
	ctx context.Context,
	db *gorm.DB,
	roleID string,
) (*dto.GetAuthorizeResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("authorization database is nil")
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return nil, fmt.Errorf("authorization role ID is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db = db.WithContext(ctx)
	key := roleAuthorizationRevisionKey(roleID)
	for attempt := 0; attempt < authorizationStableReadMaxAttempts; attempt++ {
		var resource *dto.GetAuthorizeResponse
		err := db.Transaction(func(tx *gorm.DB) error {
			if _, err := loadAuthorizationRole(tx, roleID, "SHARE"); err != nil {
				return err
			}
			row := &models.ConfigRevision{Scope: key.scope, OwnerID: key.ownerID, Resource: key.resource}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
				return err
			}
			before, err := readConfigRevision(tx, key)
			if err != nil {
				return err
			}
			resource, err = loadRoleAuthorizationResource(tx, roleID, before)
			if err != nil {
				return err
			}
			after, err := readConfigRevision(tx, key)
			if err != nil {
				return err
			}
			if before != after {
				return errAuthorizationSnapshotChanged
			}
			resource.Revision = strconv.FormatInt(after, 10)
			return nil
		})
		if errors.Is(err, errAuthorizationSnapshotChanged) {
			continue
		}
		return resource, err
	}
	return nil, fmt.Errorf("role authorization changed during read")
}

// ReplaceRole atomically replaces MENU, COMPONENT, and derived API rules for a
// role under a mandatory strong revision precondition.
func (e *AuthorizationPolicyService) ReplaceRole(
	ctx context.Context,
	db *gorm.DB,
	roleID string,
	paths []string,
	expectedRevision int64,
) (*dto.GetAuthorizeResponse, error) {
	return e.replaceRolePolicy(ctx, db, roleID, paths, expectedRevision, rolePolicyReplacement{
		selectedTypes: []adminpkg.AccessType{adminpkg.MenuAccessType, adminpkg.ComponentAccessType},
		deleteScopes:  []string{adminpkg.MenuAccessType.String(), adminpkg.ComponentAccessType.String(), adminpkg.APIAccessType.String()},
		includeAPIs:   true,
	})
}

// ReplaceMenu atomically replaces the role's MENU rules and their derived API
// rules while preserving COMPONENT rules. It advances the same canonical
// role/global revision resources so all writers and readers observe one
// concurrency domain.
func (e *AuthorizationPolicyService) ReplaceMenu(
	ctx context.Context,
	db *gorm.DB,
	roleID string,
	paths []string,
	expectedRevision int64,
) (*dto.GetAuthorizeResponse, error) {
	return e.replaceRolePolicy(ctx, db, roleID, paths, expectedRevision, rolePolicyReplacement{
		selectedTypes: []adminpkg.AccessType{adminpkg.MenuAccessType},
		deleteScopes:  []string{adminpkg.MenuAccessType.String(), adminpkg.APIAccessType.String()},
		includeAPIs:   true,
	})
}

// AuthorizationAPIReference identifies one discovered API by its canonical
// method/path natural key.
type AuthorizationAPIReference struct {
	Method string
	Path   string
}

// BindMenuAPIs replaces a MENU node's API metadata and recomputes API rules for
// every role affected by the old binding or by the selected MENU. Metadata,
// policies, role revisions, and the global revision commit atomically.
func (e *AuthorizationPolicyService) BindMenuAPIs(
	ctx context.Context,
	db *gorm.DB,
	menuID string,
	references []AuthorizationAPIReference,
) error {
	if e == nil {
		return fmt.Errorf("authorization policy service is nil")
	}
	if db == nil {
		return fmt.Errorf("authorization database is nil")
	}
	menuID = strings.TrimSpace(menuID)
	if menuID == "" {
		return ErrAuthorizationMenuNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, reference := range references {
		if strings.TrimSpace(reference.Method) == "" || strings.TrimSpace(reference.Path) == "" {
			return ErrAuthorizationAPIInvalid
		}
	}
	references = canonicalAuthorizationAPIReferences(references)
	db = db.WithContext(ctx)
	var committedGlobalRevision int64
	err := db.Transaction(func(tx *gorm.DB) error {
		globalRevision, err := lockConfigRevision(tx, globalAuthorizationRevisionKey())
		if err != nil {
			return err
		}

		var parent models.Menu
		result := tx.Where(
			"id = ? AND type = ? AND status = ?",
			menuID,
			adminpkg.MenuAccessType,
			bootenum.Enabled,
		).Limit(1).Find(&parent)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAuthorizationMenuNotFound
		}

		apis := make([]models.API, 0, len(references))
		for _, reference := range references {
			var matches []models.API
			if err := tx.Where("method = ? AND path = ?", reference.Method, reference.Path).
				Limit(2).
				Find(&matches).Error; err != nil {
				return err
			}
			if len(matches) != 1 {
				return fmt.Errorf("%w: %s %s", ErrAuthorizationAPIInvalid, reference.Method, reference.Path)
			}
			apis = append(apis, matches[0])
		}

		var oldChildren []models.Menu
		if err := tx.Where("parent_id = ? AND type = ?", parent.ID, adminpkg.APIAccessType).
			Find(&oldChildren).Error; err != nil {
			return err
		}
		affectedRoles, err := loadAffectedAPIBindingRoles(tx, parent, oldChildren)
		if err != nil {
			return err
		}

		if err := tx.Session(&gorm.Session{SkipHooks: true}).
			Where("parent_id = ? AND type = ?", parent.ID, adminpkg.APIAccessType).
			Unscoped().
			Delete(&models.Menu{}).Error; err != nil {
			return err
		}
		children := make([]models.Menu, 0, len(apis))
		for i := range apis {
			child := models.Menu{
				ParentID:   parent.ID,
				Name:       apis[i].Name,
				Path:       apis[i].Path,
				Method:     apis[i].Method,
				Type:       adminpkg.APIAccessType,
				Status:     bootenum.Enabled,
				HideInMenu: true,
			}
			child.ID = adminpkg.SimpleID()
			children = append(children, child)
		}
		if len(children) > 0 {
			if err := tx.Session(&gorm.Session{SkipHooks: true}).Create(&children).Error; err != nil {
				return err
			}
		}

		for _, roleID := range affectedRoles {
			roleRevision, err := lockConfigRevision(tx, roleAuthorizationRevisionKey(roleID))
			if err != nil {
				return err
			}
			derived, err := loadDerivedAPIRulesForRole(tx, roleID)
			if err != nil {
				return err
			}
			if err := tx.Where("ptype = ? AND v0 = ? AND v1 = ?", "p", roleID, adminpkg.APIAccessType.String()).
				Delete(&models.CasbinRule{}).Error; err != nil {
				return err
			}
			if len(derived) > 0 {
				if err := tx.Create(&derived).Error; err != nil {
					return err
				}
			}
			if _, err := advanceConfigRevision(tx, roleAuthorizationRevisionKey(roleID), roleRevision); err != nil {
				return err
			}
		}
		committedGlobalRevision, err = advanceConfigRevision(tx, globalAuthorizationRevisionKey(), globalRevision)
		return err
	})
	if err != nil {
		return err
	}
	if err := e.reconcileCommitted(ctx, db, committedGlobalRevision); err != nil {
		return &AuthorizationPropagationError{Err: err}
	}
	return nil
}

func canonicalAuthorizationAPIReferences(references []AuthorizationAPIReference) []AuthorizationAPIReference {
	result := make([]AuthorizationAPIReference, 0, len(references))
	seen := make(map[string]struct{}, len(references))
	for _, reference := range references {
		reference.Method = strings.ToUpper(strings.TrimSpace(reference.Method))
		reference.Path = strings.TrimSpace(reference.Path)
		if reference.Method == "" || reference.Path == "" {
			continue
		}
		key := reference.Method + "\x00" + reference.Path
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, reference)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Method < result[j].Method
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func loadAffectedAPIBindingRoles(db *gorm.DB, parent models.Menu, oldChildren []models.Menu) ([]string, error) {
	roles := make(map[string]struct{})
	var menuPolicies []models.CasbinRule
	if err := db.Where("ptype = ? AND v1 = ? AND v2 = ?", "p", adminpkg.MenuAccessType.String(), parent.Path).
		Find(&menuPolicies).Error; err != nil {
		return nil, err
	}
	for i := range menuPolicies {
		if strings.TrimSpace(menuPolicies[i].V0) != "" {
			roles[menuPolicies[i].V0] = struct{}{}
		}
	}
	oldPaths := make(map[string]struct{}, len(oldChildren))
	for i := range oldChildren {
		oldPaths[strings.TrimSpace(oldChildren[i].Path)] = struct{}{}
	}
	if len(oldPaths) > 0 {
		var apiPolicies []models.CasbinRule
		if err := db.Where("ptype = ? AND v1 = ?", "p", adminpkg.APIAccessType.String()).Find(&apiPolicies).Error; err != nil {
			return nil, err
		}
		for i := range apiPolicies {
			if _, affected := oldPaths[strings.TrimSpace(apiPolicies[i].V2)]; affected &&
				strings.TrimSpace(apiPolicies[i].V0) != "" {
				roles[apiPolicies[i].V0] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(roles))
	for roleID := range roles {
		result = append(result, roleID)
	}
	sort.Strings(result)
	return result, nil
}

func loadDerivedAPIRulesForRole(db *gorm.DB, roleID string) ([]models.CasbinRule, error) {
	var policies []models.CasbinRule
	if err := db.Where("ptype = ? AND v0 = ? AND v1 = ?", "p", roleID, adminpkg.MenuAccessType.String()).
		Find(&policies).Error; err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(policies))
	seenPaths := make(map[string]struct{}, len(policies))
	for i := range policies {
		path := strings.TrimSpace(policies[i].V2)
		if path == "" {
			continue
		}
		if _, exists := seenPaths[path]; exists {
			continue
		}
		seenPaths[path] = struct{}{}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return make([]models.CasbinRule, 0), nil
	}
	var menus []models.Menu
	if err := db.Where("path IN ? AND type = ? AND status = ?", paths, adminpkg.MenuAccessType, bootenum.Enabled).
		Find(&menus).Error; err != nil {
		return nil, err
	}
	menuCounts := make(map[string]int, len(menus))
	for i := range menus {
		menuCounts[strings.TrimSpace(menus[i].Path)]++
	}
	for _, count := range menuCounts {
		if count > 1 {
			return nil, ErrAuthorizationMetadataConflict
		}
	}
	parentIDs := make([]string, 0, len(menus))
	for i := range menus {
		parentIDs = append(parentIDs, menus[i].ID)
	}
	if len(parentIDs) == 0 {
		return make([]models.CasbinRule, 0), nil
	}
	var children []models.Menu
	if err := db.Where("parent_id IN ? AND type = ? AND status = ?", parentIDs, adminpkg.APIAccessType, bootenum.Enabled).
		Order("path ASC, method ASC, id ASC").
		Find(&children).Error; err != nil {
		return nil, err
	}
	rules := make([]models.CasbinRule, 0, len(children))
	seen := make(map[string]struct{}, len(children))
	for i := range children {
		method := normalizeAuthorizationMethod(children[i].Method)
		key := authorizationAPITuple(children[i].Path, method)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		rules = append(rules, models.CasbinRule{
			PType: "p", V0: roleID, V1: adminpkg.APIAccessType.String(), V2: children[i].Path, V3: method,
		})
	}
	return rules, nil
}

func authorizationAPITuple(path, method string) string {
	return strings.TrimSpace(path) + "\x00" + normalizeAuthorizationMethod(method)
}

func normalizeAuthorizationMethod(method string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return ".*"
	}
	return method
}

// ValidateAPIMetadataUpdate prevents the generic API registry controller from
// changing the method/path natural key while that tuple is projected into a
// MENU API child or an active Casbin API policy. Non-authority fields such as
// the display name and handler remain editable by root administrators.
func ValidateAPIMetadataUpdate(ctx context.Context, db *gorm.DB, next *models.API) error {
	if db == nil || next == nil || strings.TrimSpace(next.ID) == "" {
		return fmt.Errorf("authorization API metadata update is invalid")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var current models.API
	if err := db.WithContext(ctx).Where("id = ?", next.ID).Take(&current).Error; err != nil {
		return err
	}
	if current.Path == next.Path && current.Method == next.Method {
		return nil
	}
	return rejectReferencedAPIMetadata(ctx, db, current)
}

// ValidateAPIMetadataDelete requires root administrators to remove the MENU
// binding through the canonical bind-api resource before deleting a registry
// row. Batch deletion is deliberately rejected because the legacy generic
// hook does not expose the already-bound identifier list.
func ValidateAPIMetadataDelete(ctx context.Context, db *gorm.DB, apiID string) error {
	apiID = strings.TrimSpace(apiID)
	if db == nil || apiID == "" || apiID == "batch" {
		return fmt.Errorf("generic batch API metadata deletion is not supported")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var current models.API
	if err := db.WithContext(ctx).Where("id = ?", apiID).Take(&current).Error; err != nil {
		return err
	}
	return rejectReferencedAPIMetadata(ctx, db, current)
}

func rejectReferencedAPIMetadata(ctx context.Context, db *gorm.DB, current models.API) error {
	path := strings.TrimSpace(current.Path)
	method := strings.TrimSpace(current.Method)
	if path == "" || method == "" {
		return ErrAuthorizationMetadataConflict
	}
	var children []models.Menu
	if err := db.WithContext(ctx).
		Where("type = ? AND path = ?", adminpkg.APIAccessType, path).
		Find(&children).Error; err != nil {
		return err
	}
	for i := range children {
		if strings.TrimSpace(children[i].Method) == method {
			return ErrAuthorizationMetadataInUse
		}
	}
	var policies []models.CasbinRule
	if err := db.WithContext(ctx).
		Where("ptype = ? AND v1 = ? AND v2 = ?", "p", adminpkg.APIAccessType.String(), path).
		Find(&policies).Error; err != nil {
		return err
	}
	for i := range policies {
		if strings.TrimSpace(policies[i].V3) == method {
			return ErrAuthorizationMetadataInUse
		}
	}
	return nil
}

// ValidateMenuMetadataCreate prevents a new assignable MENU/COMPONENT node
// from reusing an active permission path. The canonical role resource carries
// paths without a type discriminator, so such duplicates would be ambiguous.
func ValidateMenuMetadataCreate(ctx context.Context, db *gorm.DB, menu *models.Menu) error {
	if db == nil || menu == nil {
		return fmt.Errorf("authorization menu create is invalid")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return normalizeAndRejectAmbiguousAuthorizationPath(ctx, db, menu)
}

// ValidateMenuMetadataUpdate blocks an authority-identity/status mutation when
// the target subtree is still referenced by any active Casbin policy. The root
// operator must first replace affected role resources, preventing stale grants.
func ValidateMenuMetadataUpdate(ctx context.Context, db *gorm.DB, next *models.Menu) error {
	if db == nil || next == nil || strings.TrimSpace(next.ID) == "" {
		return fmt.Errorf("authorization menu update is invalid")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := normalizeAndRejectAmbiguousAuthorizationPath(ctx, db, next); err != nil {
		return err
	}
	var current models.Menu
	if err := db.WithContext(ctx).Where("id = ?", next.ID).Take(&current).Error; err != nil {
		return err
	}
	if current.Path == next.Path && current.Method == next.Method && current.Type == next.Type &&
		current.Status == next.Status && current.ParentID == next.ParentID {
		return nil
	}
	return rejectReferencedMenuSubtree(ctx, db, current.ID)
}

func normalizeAndRejectAmbiguousAuthorizationPath(ctx context.Context, db *gorm.DB, next *models.Menu) error {
	if next.Type != adminpkg.MenuAccessType && next.Type != adminpkg.ComponentAccessType {
		return nil
	}
	canonicalPath := strings.TrimSpace(next.Path)
	next.Path = canonicalPath
	if canonicalPath == "" {
		return ErrAuthorizationMetadataConflict
	}
	// BeforeCreate applies the enabled default inside GORM after the controller
	// hook, so an empty status must be validated as the active value it will
	// become. Explicitly disabled metadata is normalized but is not assignable.
	if next.Status != "" && next.Status != bootenum.Enabled {
		return nil
	}
	var candidates []models.Menu
	if err := db.WithContext(ctx).
		Select("id", "path").
		Where("id <> ? AND status = ? AND type IN ?", next.ID, bootenum.Enabled, []adminpkg.AccessType{
			adminpkg.MenuAccessType,
			adminpkg.ComponentAccessType,
		}).
		Find(&candidates).Error; err != nil {
		return err
	}
	for i := range candidates {
		if strings.TrimSpace(candidates[i].Path) == canonicalPath {
			return ErrAuthorizationMetadataConflict
		}
	}
	return nil
}

// ValidateMenuMetadataDelete blocks single-row generic deletion while the
// target or any descendant still has a Casbin policy. Batch generic deletion is
// deliberately unsupported because the legacy hook does not expose bound IDs.
func ValidateMenuMetadataDelete(ctx context.Context, db *gorm.DB, menuID string) error {
	menuID = strings.TrimSpace(menuID)
	if menuID == "" || menuID == "batch" {
		return fmt.Errorf("generic batch menu deletion is not supported")
	}
	return rejectReferencedMenuSubtree(ctx, db, menuID)
}

func rejectReferencedMenuSubtree(ctx context.Context, db *gorm.DB, menuID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	menus, err := loadMenuSubtree(db.WithContext(ctx), menuID)
	if err != nil {
		return err
	}
	tuples := make(map[string]struct{}, len(menus))
	for i := range menus {
		tuples[string(menus[i].Type)+"\x00"+menus[i].Path] = struct{}{}
	}
	if len(tuples) == 0 {
		return nil
	}
	var policies []models.CasbinRule
	if err := db.WithContext(ctx).Where("ptype = ?", "p").Find(&policies).Error; err != nil {
		return err
	}
	for i := range policies {
		key := policies[i].V1 + "\x00" + policies[i].V2
		if _, referenced := tuples[key]; referenced {
			return ErrAuthorizationMetadataInUse
		}
	}
	return nil
}

func loadMenuSubtree(db *gorm.DB, rootID string) ([]models.Menu, error) {
	var root models.Menu
	result := db.Where("id = ?", rootID).Limit(1).Find(&root)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	resultMenus := []models.Menu{root}
	frontier := []string{root.ID}
	seen := map[string]struct{}{root.ID: {}}
	for len(frontier) > 0 {
		var children []models.Menu
		if err := db.Where("parent_id IN ?", frontier).Find(&children).Error; err != nil {
			return nil, err
		}
		frontier = frontier[:0]
		for i := range children {
			if _, exists := seen[children[i].ID]; exists {
				continue
			}
			seen[children[i].ID] = struct{}{}
			resultMenus = append(resultMenus, children[i])
			frontier = append(frontier, children[i].ID)
		}
	}
	return resultMenus, nil
}

type rolePolicyReplacement struct {
	selectedTypes []adminpkg.AccessType
	deleteScopes  []string
	includeAPIs   bool
}

func (e *AuthorizationPolicyService) replaceRolePolicy(
	ctx context.Context,
	db *gorm.DB,
	roleID string,
	paths []string,
	expectedRevision int64,
	replacement rolePolicyReplacement,
) (*dto.GetAuthorizeResponse, error) {
	if e == nil {
		return nil, fmt.Errorf("authorization policy service is nil")
	}
	if db == nil {
		return nil, fmt.Errorf("authorization database is nil")
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return nil, fmt.Errorf("authorization role ID is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	paths = canonicalAuthorizationPaths(paths)
	db = db.WithContext(ctx)

	var resource *dto.GetAuthorizeResponse
	var committedGlobalRevision int64
	err := db.Transaction(func(tx *gorm.DB) error {
		role, err := loadAuthorizationRole(tx, roleID, "UPDATE")
		if err != nil {
			return err
		}
		if role.Status != bootenum.Enabled {
			return ErrAuthorizationRoleInactive
		}
		globalRevision, err := lockConfigRevision(tx, globalAuthorizationRevisionKey())
		if err != nil {
			return err
		}
		roleRevision, err := lockConfigRevision(tx, roleAuthorizationRevisionKey(roleID))
		if err != nil {
			return err
		}
		if expectedRevision != roleRevision {
			current, loadErr := loadRoleAuthorizationResource(tx, roleID, roleRevision)
			if loadErr != nil {
				return loadErr
			}
			return &AuthorizationRevisionConflictError{
				Expected: expectedRevision,
				Actual:   roleRevision,
				Current:  current,
			}
		}

		menus, err := loadAssignableAuthorizationMenus(tx, paths, replacement.selectedTypes)
		if err != nil {
			return err
		}
		if invalid := invalidAuthorizationPaths(paths, menus); len(invalid) > 0 {
			return &InvalidAuthorizationPathsError{Paths: invalid}
		}
		if replacement.includeAPIs {
			if err := loadAssignableAuthorizationAPIs(tx, menus); err != nil {
				return err
			}
		}
		rules := buildAuthorizationRules(roleID, menus, replacement.includeAPIs)

		if err := tx.Where("ptype = ?", "p").
			Where("v0 = ?", roleID).
			Where("v1 IN ?", replacement.deleteScopes).
			Delete(&models.CasbinRule{}).Error; err != nil {
			return err
		}
		if len(rules) > 0 {
			if err := tx.Create(&rules).Error; err != nil {
				return err
			}
		}
		nextRoleRevision, err := advanceConfigRevision(tx, roleAuthorizationRevisionKey(roleID), roleRevision)
		if err != nil {
			return err
		}
		committedGlobalRevision, err = advanceConfigRevision(tx, globalAuthorizationRevisionKey(), globalRevision)
		if err != nil {
			return err
		}
		resource, err = loadRoleAuthorizationResource(tx, roleID, nextRoleRevision)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := e.reconcileCommitted(ctx, db, committedGlobalRevision); err != nil {
		return resource, &AuthorizationPropagationError{Current: resource, Err: err}
	}
	return resource, nil
}

func loadAuthorizationRole(db *gorm.DB, roleID, lockStrength string) (*models.Role, error) {
	query := db.Select("id", "status").Where("id = ?", roleID)
	if lockStrength != "" {
		query = query.Clauses(clause.Locking{Strength: lockStrength})
	}
	var role models.Role
	if err := query.Take(&role).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAuthorizationRoleNotFound
	} else if err != nil {
		return nil, err
	}
	return &role, nil
}

func canonicalAuthorizationPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func loadRoleAuthorizationResource(db *gorm.DB, roleID string, revision int64) (*dto.GetAuthorizeResponse, error) {
	var rows []models.CasbinRule
	err := db.Where("ptype = ?", "p").
		Where("v0 = ?", roleID).
		Where("v1 IN ?", []string{adminpkg.MenuAccessType.String(), adminpkg.ComponentAccessType.String()}).
		Order("v2 ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for i := range rows {
		path := strings.TrimSpace(rows[i].V2)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return &dto.GetAuthorizeResponse{
		RoleID:   roleID,
		Paths:    paths,
		Revision: strconv.FormatInt(revision, 10),
	}, nil
}

func loadAssignableAuthorizationMenus(
	db *gorm.DB,
	paths []string,
	types []adminpkg.AccessType,
) ([]models.Menu, error) {
	menus := make([]models.Menu, 0, len(paths))
	if len(paths) == 0 {
		return menus, nil
	}
	err := db.Where("path IN ?", paths).
		Where("type IN ?", types).
		Where("status = ?", bootenum.Enabled).
		Order("path ASC, type ASC, id ASC").
		Find(&menus).Error
	return menus, err
}

func invalidAuthorizationPaths(paths []string, menus []models.Menu) []string {
	loaded := make(map[string]int, len(menus))
	for i := range menus {
		loaded[menus[i].Path]++
	}
	invalid := make([]string, 0)
	for _, path := range paths {
		if loaded[path] != 1 {
			invalid = append(invalid, path)
		}
	}
	return invalid
}

func loadAssignableAuthorizationAPIs(db *gorm.DB, menus []models.Menu) error {
	parentIDs := make([]string, 0, len(menus))
	for i := range menus {
		if menus[i].Type == adminpkg.MenuAccessType {
			parentIDs = append(parentIDs, menus[i].ID)
		}
	}
	if len(parentIDs) == 0 {
		return nil
	}
	var children []models.Menu
	if err := db.Where("parent_id IN ?", parentIDs).
		Where("type = ?", adminpkg.APIAccessType).
		Where("status = ?", bootenum.Enabled).
		Order("parent_id ASC, path ASC, method ASC, id ASC").
		Find(&children).Error; err != nil {
		return err
	}
	byParent := make(map[string][]*models.Menu, len(parentIDs))
	for i := range children {
		child := children[i]
		byParent[child.ParentID] = append(byParent[child.ParentID], &child)
	}
	for i := range menus {
		menus[i].Children = byParent[menus[i].ID]
	}
	return nil
}

func buildAuthorizationRules(roleID string, menus []models.Menu, includeAPIs bool) []models.CasbinRule {
	rules := make([]models.CasbinRule, 0, len(menus)*2)
	seen := make(map[string]struct{}, len(menus)*2)
	appendRule := func(accessType, path, method string) {
		method = strings.TrimSpace(method)
		if method == "" {
			method = ".*"
		}
		key := accessType + "\x00" + path + "\x00" + method
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		rules = append(rules, models.CasbinRule{
			PType: "p",
			V0:    roleID,
			V1:    accessType,
			V2:    path,
			V3:    method,
		})
	}
	for i := range menus {
		appendRule(menus[i].Type.String(), menus[i].Path, menus[i].Method)
		if !includeAPIs {
			continue
		}
		for j := range menus[i].Children {
			if menus[i].Children[j].Type == adminpkg.APIAccessType {
				appendRule(adminpkg.APIAccessType.String(), menus[i].Children[j].Path, menus[i].Children[j].Method)
			}
		}
	}
	return rules
}
