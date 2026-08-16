package service

import (
	"context"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLogCleanerTaskRequiresLeasedDatabase(t *testing.T) {
	if err := models.TaskFuncMap["log_cleaner"](context.Background(), "30", "7", t.TempDir()); err == nil {
		t.Fatal("log_cleaner task succeeded without its leased database")
	}
}

func TestLogCleanerTaskUsesDatabaseFromExecutionContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:log-cleaner-task?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open task database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.AuditLog{},
		&models.LoginLog{},
		&models.TaskRun{},
		&models.TaskRunLog{},
	); err != nil {
		t.Fatalf("migrate log tables: %v", err)
	}
	old := time.Now().AddDate(0, 0, -60)
	if err := db.Create(&models.AuditLog{CreatedAt: old}).Error; err != nil {
		t.Fatalf("create old audit log: %v", err)
	}
	if err := db.Create(&models.LoginLog{LoginAt: old}).Error; err != nil {
		t.Fatalf("create old login log: %v", err)
	}
	run := &models.TaskRun{CreatedAt: old, TaskID: "old-task"}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create old task run: %v", err)
	}
	if err := db.Create(&models.TaskRunLog{
		TaskRunID: run.ID,
		CreatedAt: old,
		Content:   "token=must-not-survive-retention",
	}).Error; err != nil {
		t.Fatalf("create old task run log: %v", err)
	}

	ctx := pkg.WithTaskDatabase(context.Background(), db)
	if err := models.TaskFuncMap["log_cleaner"](ctx, "30", "7", t.TempDir()); err != nil {
		t.Fatalf("run log_cleaner task: %v", err)
	}
	for _, model := range []any{
		&models.AuditLog{},
		&models.LoginLog{},
		&models.TaskRun{},
		&models.TaskRunLog{},
	} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatalf("count cleaned %T rows: %v", model, err)
		}
		if count != 0 {
			t.Fatalf("cleaned %T row count = %d, want 0", model, count)
		}
	}
}
