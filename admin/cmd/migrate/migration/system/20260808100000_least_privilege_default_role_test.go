package system

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const leastPrivilegeDefaultRoleTestVersion = "2026080810000-test"

func TestLeastPrivilegeDefaultRoleMigrationFreshAndIdempotent(t *testing.T) {
	db := openLeastPrivilegeDefaultRoleTestDB(t)

	for attempt := 0; attempt < 2; attempt++ {
		if err := migrateLeastPrivilegeDefaultRole(db, leastPrivilegeDefaultRoleTestVersion); err != nil {
			t.Fatalf("fresh migration attempt %d: %v", attempt+1, err)
		}
	}

	role := requireLeastPrivilegeDefaultRole(t, db)
	assertLeastPrivilegeWelcomeRules(t, db, role.ID)
	assertLeastPrivilegeAuthorizationRevisions(t, db, role.ID, 1, 1)
	assertLeastPrivilegeMigrationVersion(t, db, leastPrivilegeDefaultRoleTestVersion, 1)

	var roleCount int64
	if err := db.Model(&models.Role{}).Count(&roleCount).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if roleCount != 1 {
		t.Fatalf("roles after idempotent migration = %d, want 1", roleCount)
	}
}

func TestLeastPrivilegeDefaultRoleMigrationUpgradesCanonicalAdmin(t *testing.T) {
	db := openLeastPrivilegeDefaultRoleTestDB(t)
	adminRole := createLeastPrivilegeRoleFixture(t, db, "admin-role", "admin", true, true)
	createLeastPrivilegeUserFixture(t, db, "admin-user", adminRole.ID)

	if err := migrateLeastPrivilegeDefaultRole(db, leastPrivilegeDefaultRoleTestVersion); err != nil {
		t.Fatalf("upgrade migration: %v", err)
	}

	var reloadedAdmin models.Role
	if err := db.First(&reloadedAdmin, "id = ?", adminRole.ID).Error; err != nil {
		t.Fatalf("reload admin role: %v", err)
	}
	if reloadedAdmin.Default || !reloadedAdmin.Root {
		t.Fatalf("historical admin role flags = default:%v root:%v, want false/true", reloadedAdmin.Default, reloadedAdmin.Root)
	}
	role := requireLeastPrivilegeDefaultRole(t, db)
	if role.ID == adminRole.ID {
		t.Fatal("provisioning reused the root admin role")
	}
	assertLeastPrivilegeWelcomeRules(t, db, role.ID)
	assertLeastPrivilegeAuthorizationRevisions(t, db, role.ID, 1, 1)

	var adminUser models.User
	if err := db.First(&adminUser, "id = ?", "admin-user").Error; err != nil {
		t.Fatalf("reload admin user: %v", err)
	}
	if adminUser.RoleID != adminRole.ID {
		t.Fatalf("admin user role changed to %q, want %q", adminUser.RoleID, adminRole.ID)
	}
}

func TestLeastPrivilegeDefaultRoleMigrationAdvancesExistingAuthorizationRevisionsOnce(t *testing.T) {
	db := openLeastPrivilegeDefaultRoleTestDB(t)
	role := createLeastPrivilegeRoleFixture(t, db, "existing-default", "user", false, true)
	if err := seedLeastPrivilegeWelcome(db, role.ID); err != nil {
		t.Fatalf("preseed provisioning policies: %v", err)
	}
	rows := []models.ConfigRevision{
		{
			Scope: leastPrivilegeAuthorizationScopeGlobal, Resource: leastPrivilegeAuthorizationResource,
			Revision: 11,
		},
		{
			Scope: leastPrivilegeAuthorizationScopeRole, OwnerID: role.ID,
			Resource: leastPrivilegeAuthorizationResource, Revision: 7,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed authorization revisions: %v", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := migrateLeastPrivilegeDefaultRole(db, leastPrivilegeDefaultRoleTestVersion); err != nil {
			t.Fatalf("upgrade migration attempt %d: %v", attempt+1, err)
		}
	}

	assertLeastPrivilegeAuthorizationRevisions(t, db, role.ID, 12, 8)
	assertLeastPrivilegeWelcomeRules(t, db, role.ID)
	assertLeastPrivilegeMigrationVersion(t, db, leastPrivilegeDefaultRoleTestVersion, 1)
}

func TestLeastPrivilegeDefaultRoleMigrationRejectsAmbiguousDefaults(t *testing.T) {
	db := openLeastPrivilegeDefaultRoleTestDB(t)
	createLeastPrivilegeRoleFixture(t, db, "default-one", "member-one", false, true)
	createLeastPrivilegeRoleFixture(t, db, "default-two", "member-two", false, true)

	err := migrateLeastPrivilegeDefaultRole(db, leastPrivilegeDefaultRoleTestVersion)
	if err == nil || !strings.Contains(err.Error(), "ambiguous default roles count=2") {
		t.Fatalf("ambiguous defaults error = %v", err)
	}
	assertLeastPrivilegeMigrationVersion(t, db, leastPrivilegeDefaultRoleTestVersion, 0)
	assertLeastPrivilegeRuleCount(t, db, 0)
}

func TestLeastPrivilegeDefaultRoleMigrationRejectsSharedRootDefault(t *testing.T) {
	db := openLeastPrivilegeDefaultRoleTestDB(t)
	adminRole := createLeastPrivilegeRoleFixture(t, db, "admin-role", "admin", true, true)
	createLeastPrivilegeUserFixture(t, db, "admin-user", adminRole.ID)
	createLeastPrivilegeUserFixture(t, db, "unexpected-root-user", adminRole.ID)

	err := migrateLeastPrivilegeDefaultRole(db, leastPrivilegeDefaultRoleTestVersion)
	if err == nil || !strings.Contains(err.Error(), "assigned to 2 users") {
		t.Fatalf("shared root-default error = %v", err)
	}
	assertLeastPrivilegeMigrationVersion(t, db, leastPrivilegeDefaultRoleTestVersion, 0)
	assertLeastPrivilegeRuleCount(t, db, 0)

	var reloaded models.Role
	if err := db.First(&reloaded, "id = ?", adminRole.ID).Error; err != nil {
		t.Fatalf("reload rejected admin role: %v", err)
	}
	if !reloaded.Default || !reloaded.Root {
		t.Fatalf("rejected migration mutated admin flags to default:%v root:%v", reloaded.Default, reloaded.Root)
	}
}

func TestLeastPrivilegeDefaultRoleMigrationRejectsNonCanonicalRootDefault(t *testing.T) {
	db := openLeastPrivilegeDefaultRoleTestDB(t)
	rootRole := createLeastPrivilegeRoleFixture(t, db, "other-root", "superuser", true, true)
	createLeastPrivilegeUserFixture(t, db, "root-user", rootRole.ID)

	err := migrateLeastPrivilegeDefaultRole(db, leastPrivilegeDefaultRoleTestVersion)
	if err == nil || !strings.Contains(err.Error(), "is not canonical admin") {
		t.Fatalf("non-canonical root-default error = %v", err)
	}
	assertLeastPrivilegeMigrationVersion(t, db, leastPrivilegeDefaultRoleTestVersion, 0)
}

func openLeastPrivilegeDefaultRoleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "least-privilege-default-role.db")),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open migration test database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.CasbinRule{},
		&models.ConfigRevision{},
		&migrationmodels.Migration{},
	); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	return db
}

func assertLeastPrivilegeAuthorizationRevisions(
	t *testing.T,
	db *gorm.DB,
	roleID string,
	wantGlobal int64,
	wantRole int64,
) {
	t.Helper()
	wants := []struct {
		scope    string
		ownerID  string
		revision int64
	}{
		{scope: leastPrivilegeAuthorizationScopeGlobal, ownerID: "", revision: wantGlobal},
		{scope: leastPrivilegeAuthorizationScopeRole, ownerID: roleID, revision: wantRole},
	}
	for _, want := range wants {
		var row models.ConfigRevision
		if err := db.Where(
			"scope = ? AND owner_id = ? AND resource = ?",
			want.scope,
			want.ownerID,
			leastPrivilegeAuthorizationResource,
		).Take(&row).Error; err != nil {
			t.Fatalf("load authorization revision %s/%s: %v", want.scope, want.ownerID, err)
		}
		if row.Revision != want.revision {
			t.Fatalf(
				"authorization revision %s/%s = %d, want %d",
				want.scope,
				want.ownerID,
				row.Revision,
				want.revision,
			)
		}
	}
}

func createLeastPrivilegeRoleFixture(
	t *testing.T,
	db *gorm.DB,
	id string,
	name string,
	root bool,
	defaultRole bool,
) *models.Role {
	t.Helper()
	role := &models.Role{Name: name, Status: enum.Enabled}
	role.ID = id
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(role).Error; err != nil {
		t.Fatalf("create role %q: %v", id, err)
	}
	if err := db.Exec(
		`UPDATE mss_boot_roles SET root = ?, "default" = ? WHERE id = ?`,
		root,
		defaultRole,
		id,
	).Error; err != nil {
		t.Fatalf("set role %q flags: %v", id, err)
	}
	role.Root = root
	role.Default = defaultRole
	return role
}

func createLeastPrivilegeUserFixture(t *testing.T, db *gorm.DB, id, roleID string) {
	t.Helper()
	user := &models.User{UserLogin: models.UserLogin{
		RoleID:   roleID,
		Username: id,
		Status:   enum.Enabled,
	}}
	user.ID = id
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(user).Error; err != nil {
		t.Fatalf("create user %q: %v", id, err)
	}
}

func requireLeastPrivilegeDefaultRole(t *testing.T, db *gorm.DB) models.Role {
	t.Helper()
	var roles []models.Role
	if err := db.Where(map[string]any{"default": true}).Find(&roles).Error; err != nil {
		t.Fatalf("find default roles: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("default roles = %d, want 1", len(roles))
	}
	role := roles[0]
	if role.Name != leastPrivilegeDefaultRoleName || role.Root || role.Status != enum.Enabled {
		t.Fatalf("default role = %#v, want enabled non-root %q", role, leastPrivilegeDefaultRoleName)
	}
	return role
}

func assertLeastPrivilegeWelcomeRules(t *testing.T, db *gorm.DB, roleID string) {
	t.Helper()
	var rules []models.CasbinRule
	if err := db.Where("v0 = ?", roleID).Order("v1, v2, v3").Find(&rules).Error; err != nil {
		t.Fatalf("load provisioning rules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("provisioning rules = %#v, want exactly two welcome rules", rules)
	}
	want := map[string]bool{
		pkg.MenuAccessType.String() + " /welcome GET":          false,
		pkg.APIAccessType.String() + " /admin/api/monitor GET": false,
	}
	for _, rule := range rules {
		key := rule.V1 + " " + rule.V2 + " " + rule.V3
		if _, exists := want[key]; !exists || rule.PType != "p" {
			t.Fatalf("unexpected provisioning rule: %#v", rule)
		}
		want[key] = true
	}
	for key, found := range want {
		if !found {
			t.Fatalf("missing provisioning rule %s", key)
		}
	}
}

func assertLeastPrivilegeMigrationVersion(t *testing.T, db *gorm.DB, version string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&count).Error; err != nil {
		t.Fatalf("count migration version: %v", err)
	}
	if count != want {
		t.Fatalf("migration version count = %d, want %d", count, want)
	}
}

func assertLeastPrivilegeRuleCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&models.CasbinRule{}).Count(&count).Error; err != nil {
		t.Fatalf("count Casbin rules: %v", err)
	}
	if count != want {
		t.Fatalf("Casbin rule count = %d, want %d", count, want)
	}
}
