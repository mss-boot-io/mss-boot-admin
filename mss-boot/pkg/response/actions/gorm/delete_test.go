package gorm

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
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

func TestDeleteRunsReferenceGuardInTransactionAndMapsItsError(t *testing.T) {
	db, err := gormpkg.Open(sqlite.Open(":memory:"), &gormpkg.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&deleteCommitRecord{}, &deleteCommitAudit{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	record := &deleteCommitRecord{Name: "protected"}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

	sensitive := errors.New("foreign key driver secret")
	var seenOperation actions.WriteOperation
	guard := func(ctx *gin.Context, tx *gormpkg.DB, _ schema.Tabler) error {
		value, exists := ctx.Get("ids")
		ids, ok := value.([]string)
		if !exists || !ok || len(ids) != 1 || ids[0] != fmt.Sprint(record.ID) {
			return fmt.Errorf("bound delete identifiers = %#v", value)
		}
		if err := tx.Create(&deleteCommitAudit{RecordID: record.ID}).Error; err != nil {
			return err
		}
		return sensitive
	}
	mapper := func(
		_ *gin.Context,
		operation actions.WriteOperation,
		cause error,
	) (actions.PublicWriteError, bool) {
		if !errors.Is(cause, sensitive) {
			return actions.PublicWriteError{}, false
		}
		seenOperation = operation
		return actions.PublicWriteError{
			Status: http.StatusConflict,
			Error:  response.NewError("DELETE_IN_USE", "record is in use"),
		}, true
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	deleteAction := NewDelete(
		WithModel(&deleteCommitRecord{}),
		WithKey("id"),
		WithBeforeDelete(guard),
		WithWriteErrorMapper(mapper),
	)
	router.DELETE("/records/:id", deleteAction.Handler()...)
	result := httptest.NewRecorder()
	router.ServeHTTP(
		result,
		httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/records/%d", record.ID), nil),
	)

	if result.Code != http.StatusConflict {
		t.Fatalf("delete response = %d: %s", result.Code, result.Body.String())
	}
	if seenOperation != actions.WriteOperationBeforeDelete {
		t.Fatalf("mapped operation = %q, want before-delete", seenOperation)
	}
	if body := result.Body.String(); !strings.Contains(body, "DELETE_IN_USE") || strings.Contains(body, "driver secret") {
		t.Fatalf("delete response was not fixed and redacted: %s", body)
	}
	var remaining int64
	if err := db.Model(&deleteCommitRecord{}).Where("id = ?", record.ID).Count(&remaining).Error; err != nil {
		t.Fatalf("count record: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("protected record count = %d, want 1", remaining)
	}
	var audits int64
	if err := db.Model(&deleteCommitAudit{}).Count(&audits).Error; err != nil {
		t.Fatalf("count rolled-back audit: %v", err)
	}
	if audits != 0 {
		t.Fatalf("guard audit count = %d, want transaction rollback", audits)
	}
}

func TestDeleteBatchBindsJSONAndCannotBypassReferenceGuard(t *testing.T) {
	db, err := gormpkg.Open(sqlite.Open(":memory:"), &gormpkg.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&deleteCommitRecord{}, &deleteCommitAudit{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	records := []*deleteCommitRecord{{Name: "safe"}, {Name: "referenced"}}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed record: %v", err)
		}
	}

	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

	referenced := errors.New("referenced record driver detail")
	guard := func(ctx *gin.Context, tx *gormpkg.DB, _ schema.Tabler) error {
		value, exists := ctx.Get("ids")
		ids, ok := value.([]string)
		if !exists || !ok || len(ids) != 2 || ids[0] != fmt.Sprint(records[0].ID) ||
			ids[1] != fmt.Sprint(records[1].ID) {
			return fmt.Errorf("bound batch identifiers = %#v", value)
		}
		if err := tx.Create(&deleteCommitAudit{RecordID: records[0].ID}).Error; err != nil {
			return err
		}
		return referenced
	}
	mapper := func(
		_ *gin.Context,
		operation actions.WriteOperation,
		cause error,
	) (actions.PublicWriteError, bool) {
		if operation != actions.WriteOperationBeforeDelete || !errors.Is(cause, referenced) {
			return actions.PublicWriteError{}, false
		}
		return actions.PublicWriteError{
			Status: http.StatusConflict,
			Error:  response.NewError("DELETE_IN_USE", "record is in use"),
		}, true
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	deleteAction := NewDelete(
		WithModel(&deleteCommitRecord{}),
		WithKey("id"),
		WithBeforeDelete(guard),
		WithWriteErrorMapper(mapper),
	)
	router.DELETE("/records/:id", deleteAction.Handler()...)
	body := fmt.Sprintf(`["%d","%d"]`, records[0].ID, records[1].ID)
	request := httptest.NewRequest(http.MethodDelete, "/records/batch", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)

	if result.Code != http.StatusConflict {
		t.Fatalf("batch delete response = %d: %s", result.Code, result.Body.String())
	}
	if body := result.Body.String(); !strings.Contains(body, "DELETE_IN_USE") || strings.Contains(body, "driver detail") {
		t.Fatalf("batch delete response was not fixed and redacted: %s", body)
	}
	var remaining int64
	if err := db.Model(&deleteCommitRecord{}).Count(&remaining).Error; err != nil {
		t.Fatalf("count records: %v", err)
	}
	if remaining != int64(len(records)) {
		t.Fatalf("remaining record count = %d, want %d", remaining, len(records))
	}
	var audits int64
	if err := db.Model(&deleteCommitAudit{}).Count(&audits).Error; err != nil {
		t.Fatalf("count rolled-back audit: %v", err)
	}
	if audits != 0 {
		t.Fatalf("guard audit count = %d, want transaction rollback", audits)
	}
}
