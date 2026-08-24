package system

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
)

const initialAdministratorMigrationID migration.MigrationID = "1691847581348"

func TestInitialAdministratorMigrationRequiresStrongCredentialsOnlyWhilePending(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "initial-administrator.db")),
		&gorm.Config{Logger: logger.Discard},
	)
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	if err := db.AutoMigrate(
		&migrationmodels.Migration{},
		&models.SystemConfig{},
		&models.Role{},
		&models.User{},
	); err != nil {
		t.Fatalf("create initial migration schema: %v", err)
	}

	runner := migration.New()
	if err := runner.Register(initialAdministratorMigrationID, _1691847581348Migrate); err != nil {
		t.Fatalf("register initial administrator migration: %v", err)
	}
	runner.SetDb(db)
	runner.SetModel(&migrationmodels.Migration{})

	t.Cleanup(func() {
		runner.SetDb(nil)
		runner.SetModel(nil)
	})
	const username = "admin"
	weakCredentialsContext := ContextWithInitialAdministratorCredentials(
		t.Context(),
		username,
		"123456",
	)
	err = runner.MigrateContext(weakCredentialsContext)
	if !errors.Is(err, ErrInitialAdministratorCredentials) {
		t.Fatalf("weak initial password error = %v, want credential requirement", err)
	}
	if strings.Contains(err.Error(), "123456") {
		t.Fatal("initial credential error exposed the rejected password")
	}
	assertInitialAdministratorCounts(t, db, username, 0, 0)

	const strongPassword = "PackageFirstAdmin2026"
	strongCredentialsContext := ContextWithInitialAdministratorCredentials(
		t.Context(),
		username,
		strongPassword,
	)
	if err := runner.MigrateContext(strongCredentialsContext); err != nil {
		t.Fatalf("run initial administrator migration: %v", err)
	}
	assertInitialAdministratorCounts(t, db, username, 1, 1)
	var administrator models.User
	if err := db.Where("username = ?", username).First(&administrator).Error; err != nil {
		t.Fatalf("load initial administrator: %v", err)
	}
	if administrator.PasswordHash == "" || administrator.PasswordHash == strongPassword {
		t.Fatal("initial administrator password was not stored as a one-way verifier")
	}
	verified, err := security.VerifyPassword(
		strongPassword,
		administrator.Salt,
		administrator.PasswordHash,
	)
	if err != nil {
		t.Fatalf("verify initial administrator password: %v", err)
	}
	if !verified {
		t.Fatal("initial administrator verifier does not match the supplied password")
	}

	repeatContext := ContextWithInitialAdministratorCredentials(
		t.Context(),
		username,
		"",
	)
	if err := runner.MigrateContext(repeatContext); err != nil {
		t.Fatalf("repeat completed migration without bootstrap password: %v", err)
	}
	assertInitialAdministratorCounts(t, db, username, 1, 1)
}

func TestInitialAdministratorCredentialsAreIsolatedAcrossConcurrentDatabases(t *testing.T) {
	t.Parallel()

	type bootstrapCase struct {
		username string
		password string
	}
	cases := []bootstrapCase{
		{username: "admin-one", password: "ConcurrentAdminOne2026"},
		{username: "admin-two", password: "ConcurrentAdminTwo2026"},
	}

	start := make(chan struct{})
	errorsByCase := make(chan error, len(cases))
	var ready sync.WaitGroup
	ready.Add(len(cases))
	for index, testCase := range cases {
		index, testCase := index, testCase
		go func() {
			db, err := openInitialAdministratorMigrationDB(
				t,
				filepath.Join(t.TempDir(), fmt.Sprintf("concurrent-%d.db", index)),
			)
			if err != nil {
				errorsByCase <- err
				ready.Done()
				return
			}
			runner := migration.New()
			if err := runner.Register(initialAdministratorMigrationID, _1691847581348Migrate); err != nil {
				errorsByCase <- err
				ready.Done()
				return
			}
			runner.SetDb(db)
			executionContext := ContextWithInitialAdministratorCredentials(
				t.Context(),
				testCase.username,
				testCase.password,
			)
			runner.SetModel(&migrationmodels.Migration{})
			ready.Done()
			<-start
			if err := runner.MigrateContext(executionContext); err != nil {
				errorsByCase <- err
				return
			}
			var administrator models.User
			if err := db.Where("username = ?", testCase.username).First(&administrator).Error; err != nil {
				errorsByCase <- err
				return
			}
			verified, err := security.VerifyPassword(
				testCase.password,
				administrator.Salt,
				administrator.PasswordHash,
			)
			if err != nil {
				errorsByCase <- err
				return
			}
			if !verified {
				errorsByCase <- fmt.Errorf("password verifier for %s used another execution's credentials", testCase.username)
				return
			}
			errorsByCase <- nil
		}()
	}
	ready.Wait()
	close(start)
	for range cases {
		if err := <-errorsByCase; err != nil {
			t.Fatal(err)
		}
	}
}

func openInitialAdministratorMigrationDB(t *testing.T, path string) (*gorm.DB, error) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&migrationmodels.Migration{},
		&models.SystemConfig{},
		&models.Role{},
		&models.User{},
	); err != nil {
		return nil, err
	}
	return db, nil
}

func assertInitialAdministratorCounts(
	t *testing.T,
	db *gorm.DB,
	username string,
	wantUsers, wantVersions int64,
) {
	t.Helper()
	var userCount int64
	if err := db.Model(&models.User{}).Where("username = ?", username).Count(&userCount).Error; err != nil {
		t.Fatalf("count initial administrators: %v", err)
	}
	if userCount != wantUsers {
		t.Fatalf("initial administrator count = %d, want %d", userCount, wantUsers)
	}
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", initialAdministratorMigrationID.String()).
		Count(&versionCount).Error; err != nil {
		t.Fatalf("count initial administrator migration versions: %v", err)
	}
	if versionCount != wantVersions {
		t.Fatalf("initial administrator migration version count = %d, want %d", versionCount, wantVersions)
	}
}
