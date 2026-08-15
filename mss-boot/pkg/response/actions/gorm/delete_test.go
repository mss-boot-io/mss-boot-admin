package gorm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	gormpkg "gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type deleteCommitRecord struct {
	ID        int64
	Name      string
	DeletedAt gormpkg.DeletedAt `gorm:"index"`
}

type deleteCommitAudit struct {
	ID       int64
	RecordID int64
}

func (*deleteCommitRecord) TableName() string { return "delete_commit_records" }

func TestDeleteRunsAfterDeleteHookAfterTransactionCommit(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "delete-after-commit.db") +
		"?_pragma=busy_timeout(100)&_pragma=journal_mode(WAL)"
	db, err := gormpkg.Open(sqlite.Open(dsn), &gormpkg.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	observer, err := gormpkg.Open(sqlite.Open(dsn), &gormpkg.Config{})
	if err != nil {
		t.Fatalf("open observer sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&deleteCommitRecord{}, &deleteCommitAudit{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	record := &deleteCommitRecord{Name: "delete me"}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("seed delete record: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	hookCalled := false
	hook := func(_ *gin.Context, _ *gormpkg.DB, _ schema.Tabler) error {
		hookCalled = true
		var committed deleteCommitRecord
		if err := observer.Unscoped().First(&committed, record.ID).Error; err != nil {
			return err
		}
		if !committed.DeletedAt.Valid {
			return fmt.Errorf("observer saw an uncommitted delete")
		}
		return observer.Create(&deleteCommitAudit{RecordID: record.ID}).Error
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	deleteAction := NewDelete(WithModel(&deleteCommitRecord{}), WithKey("id"), WithAfterDelete(hook))
	router.DELETE("/records/:id", deleteAction.Handler()...)
	request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/records/%d", record.ID), nil)
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)

	if result.Code < http.StatusOK || result.Code >= http.StatusMultipleChoices {
		t.Fatalf("delete response = %d: %s", result.Code, result.Body.String())
	}
	if !hookCalled {
		t.Fatal("after-delete hook was not called")
	}
	var audits int64
	if err := observer.Model(&deleteCommitAudit{}).Where("record_id = ?", record.ID).Count(&audits).Error; err != nil {
		t.Fatalf("count delete audits: %v", err)
	}
	if audits != 1 {
		t.Fatalf("delete audit count = %d, want 1", audits)
	}
}
