package business

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
)

func TestRequireAppliedMigrations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(new(migrationmodels.Migration)); err != nil {
		t.Fatalf("create migration table: %v", err)
	}
	applied := migrationmodels.Migration{}
	applied.SetVersion("202608190201")
	if err := db.Create(&applied).Error; err != nil {
		t.Fatalf("record migration: %v", err)
	}

	if err := RequireAppliedMigrations(
		context.Background(),
		db,
		migration.MigrationID("202608190201"),
	); err != nil {
		t.Fatalf("applied migration readiness: %v", err)
	}
	err = RequireAppliedMigrations(
		context.Background(),
		db,
		migration.MigrationID("202608190201"),
		migration.MigrationID("202608190202"),
	)
	if err == nil || !strings.Contains(err.Error(), "202608190202") {
		t.Fatalf("missing migration readiness error = %v", err)
	}
}
