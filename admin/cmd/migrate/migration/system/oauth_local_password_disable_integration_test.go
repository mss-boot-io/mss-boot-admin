package system

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	configgormdb "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/gorm"
)

const oauthLocalPasswordIntegrationVersion = "20260806171000"

type oauthLocalPasswordIntegrationState struct {
	ID                    string
	PasswordHash          string
	Salt                  string
	LocalPasswordDisabled bool
}

func TestOAuthLocalPasswordDisableMigrationMySQLIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(patMigrationMySQLDSNEnv))
	if dsn == "" {
		t.Skip(patMigrationMySQLDSNEnv + " is not set")
	}

	db := openPATMigrationIntegrationDB(t, "mysql", dsn)
	runOAuthLocalPasswordMigrationIntegrationContract(t, db, "mysql")
}

func TestOAuthLocalPasswordDisableMigrationPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(patMigrationPostgresDSNEnv))
	if dsn == "" {
		t.Skip(patMigrationPostgresDSNEnv + " is not set")
	}

	db := openPATMigrationIntegrationDB(t, "postgres", dsn)
	runOAuthLocalPasswordMigrationIntegrationContract(t, db, "postgres")
}

func runOAuthLocalPasswordMigrationIntegrationContract(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	t.Cleanup(func() {
		if err := dropOAuthLocalPasswordMigrationIntegrationTables(db); err != nil {
			t.Errorf("clean up %s OAuth local-password migration tables failed", dialect)
		}
	})
	resetOAuthLocalPasswordMigrationIntegrationTables(t, db)
	createLegacyOAuthLocalPasswordTables(t, db)

	fixtures := []oauthLocalPasswordIntegrationState{
		{
			ID:           "oauth-linked",
			PasswordHash: "legacy-provider-derived-hash",
			Salt:         "legacy-provider-salt",
		},
		{
			ID:           "local-only",
			PasswordHash: "legacy-local-password-hash",
			Salt:         "legacy-local-salt",
		},
		{
			ID:           "previously-linked",
			PasswordHash: "legacy-previously-linked-hash",
			Salt:         "legacy-previously-linked-salt",
		},
	}
	for _, fixture := range fixtures {
		if err := db.Exec(
			`INSERT INTO mss_boot_users (id, username, password_hash, salt)
			 VALUES (?, ?, ?, ?)`,
			fixture.ID,
			fixture.ID,
			fixture.PasswordHash,
			fixture.Salt,
		).Error; err != nil {
			t.Fatalf("insert %s legacy user %q failed", dialect, fixture.ID)
		}
	}
	bindings := []struct {
		id        string
		userID    string
		provider  string
		deletedAt any
	}{
		{id: "linked-binding", userID: "oauth-linked", provider: "github"},
		{id: "deleted-binding", userID: "previously-linked", provider: "lark", deletedAt: "2026-08-01 00:00:00"},
		{id: "orphan-binding", userID: "missing-user", provider: "lark"},
		{id: "empty-binding", userID: "", provider: "github"},
	}
	for _, binding := range bindings {
		if err := db.Exec(
			`INSERT INTO mss_boot_user_oauth2 (id, user_id, provider, deleted_at)
			 VALUES (?, ?, ?, ?)`,
			binding.id,
			binding.userID,
			binding.provider,
			binding.deletedAt,
		).Error; err != nil {
			t.Fatalf("insert %s legacy OAuth binding %q failed", dialect, binding.id)
		}
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateOAuthLocalPasswordDisable(db, oauthLocalPasswordIntegrationVersion); err != nil {
			t.Fatalf("%s OAuth local-password migration attempt %d failed", dialect, attempt)
		}
	}

	if !db.Migrator().HasColumn(&models.User{}, "LocalPasswordDisabled") {
		t.Fatalf("%s OAuth local-password migration did not add the disable flag", dialect)
	}
	assertOAuthLocalPasswordMigrationVersionCount(t, db, 1)

	linked := loadOAuthLocalPasswordIntegrationState(t, db, "oauth-linked")
	if !linked.LocalPasswordDisabled {
		t.Fatalf("%s OAuth-linked legacy account retained local-password login", dialect)
	}
	if linked.PasswordHash != fixtures[0].PasswordHash || linked.Salt != fixtures[0].Salt {
		t.Fatalf("%s migration changed the OAuth-linked account's legacy credential", dialect)
	}

	localOnly := loadOAuthLocalPasswordIntegrationState(t, db, "local-only")
	if localOnly.LocalPasswordDisabled {
		t.Fatalf("%s migration disabled an unlinked local account", dialect)
	}
	if localOnly.PasswordHash != fixtures[1].PasswordHash || localOnly.Salt != fixtures[1].Salt {
		t.Fatalf("%s migration changed the unlinked account's local credential", dialect)
	}

	previouslyLinked := loadOAuthLocalPasswordIntegrationState(t, db, "previously-linked")
	if !previouslyLinked.LocalPasswordDisabled {
		t.Fatalf("%s account with a soft-deleted OAuth binding retained local-password login", dialect)
	}
	if previouslyLinked.PasswordHash != fixtures[2].PasswordHash || previouslyLinked.Salt != fixtures[2].Salt {
		t.Fatalf("%s migration changed the previously linked account's legacy credential", dialect)
	}

	previousDB := configgormdb.DB
	configgormdb.DB = db
	defer func() { configgormdb.DB = previousDB }()
	const replacementPassword = "integration-recovery-password"
	if err := models.PasswordReset(context.Background(), linked.ID, replacementPassword); err != nil {
		t.Fatalf("%s password reset after OAuth migration failed: %v", dialect, err)
	}

	recovered := loadOAuthLocalPasswordIntegrationState(t, db, linked.ID)
	if recovered.LocalPasswordDisabled {
		t.Fatalf("%s password reset did not re-enable the local credential", dialect)
	}
	if recovered.Salt == "" || recovered.Salt == linked.Salt {
		t.Fatalf("%s password reset did not rotate the password salt", dialect)
	}
	wantHash, err := security.SetPassword(replacementPassword, recovered.Salt)
	if err != nil {
		t.Fatalf("derive %s recovered password hash failed", dialect)
	}
	if recovered.PasswordHash != wantHash {
		t.Fatalf("%s password reset stored an invalid replacement password hash", dialect)
	}
	assertOAuthLocalPasswordMigrationVersionCount(t, db, 1)
}

func createLegacyOAuthLocalPasswordTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE mss_boot_users (
		id VARCHAR(64) PRIMARY KEY,
		username VARCHAR(128) NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		salt VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NULL,
		updated_at TIMESTAMP NULL,
		deleted_at TIMESTAMP NULL
	)`).Error; err != nil {
		t.Fatal("create legacy user integration table failed")
	}
	if err := db.Exec(`CREATE TABLE mss_boot_user_oauth2 (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NULL,
		provider VARCHAR(32) NULL,
		deleted_at TIMESTAMP NULL
	)`).Error; err != nil {
		t.Fatal("create legacy OAuth integration table failed")
	}
}

func resetOAuthLocalPasswordMigrationIntegrationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := dropOAuthLocalPasswordMigrationIntegrationTables(db); err != nil {
		t.Fatal("reset OAuth local-password migration integration tables failed")
	}
}

func dropOAuthLocalPasswordMigrationIntegrationTables(db *gorm.DB) error {
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_user_oauth2").Error; err != nil {
		return fmt.Errorf("drop OAuth binding table: %w", err)
	}
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_users").Error; err != nil {
		return fmt.Errorf("drop user table: %w", err)
	}
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_migration").Error; err != nil {
		return fmt.Errorf("drop migration table: %w", err)
	}
	return nil
}

func loadOAuthLocalPasswordIntegrationState(
	t *testing.T,
	db *gorm.DB,
	userID string,
) oauthLocalPasswordIntegrationState {
	t.Helper()
	state := oauthLocalPasswordIntegrationState{}
	if err := db.Table((&models.User{}).TableName()).
		Select("id", "password_hash", "salt", "local_password_disabled").
		Where("id = ?", userID).
		Take(&state).Error; err != nil {
		t.Fatalf("load migrated user %q failed", userID)
	}
	return state
}

func assertOAuthLocalPasswordMigrationVersionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", oauthLocalPasswordIntegrationVersion).
		Count(&count).Error; err != nil {
		t.Fatal("count OAuth local-password migration version rows failed")
	}
	if count != want {
		t.Fatalf("OAuth local-password migration version row count = %d, want %d", count, want)
	}
}
