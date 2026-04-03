package service

import (
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.AuditLog{}, &models.LoginLog{})
	assert.NoError(t, err)

	return db
}

func TestAuditService_LogLogin(t *testing.T) {
	db := setupTestDB(t)
	svc := &AuditService{}

	err := svc.LogLogin(db, "user-1", "testuser", "192.168.1.1", "Mozilla/5.0", "Login successful", true)
	assert.NoError(t, err)

	var logs []models.LoginLog
	result := db.Find(&logs)
	assert.NoError(t, result.Error)
	assert.Len(t, logs, 1)
	assert.Equal(t, "user-1", logs[0].UserID)
	assert.Equal(t, "testuser", logs[0].Username)
	assert.Equal(t, "192.168.1.1", logs[0].IP)
	assert.Equal(t, "Login successful", logs[0].Message)
}

func TestAuditService_LogLogin_Failed(t *testing.T) {
	db := setupTestDB(t)
	svc := &AuditService{}

	err := svc.LogLogin(db, "user-1", "testuser", "192.168.1.1", "Mozilla/5.0", "Invalid password", false)
	assert.NoError(t, err)

	var logs []models.LoginLog
	result := db.Find(&logs)
	assert.NoError(t, result.Error)
	assert.Len(t, logs, 1)
	assert.Equal(t, enum.Status("disabled"), logs[0].Status)
}

func TestAuditService_LogLogout(t *testing.T) {
	db := setupTestDB(t)
	svc := &AuditService{}

	err := svc.LogLogin(db, "user-1", "testuser", "192.168.1.1", "Mozilla/5.0", "Login successful", true)
	assert.NoError(t, err)

	err = svc.LogLogout(db, "user-1")
	assert.NoError(t, err)

	var logs []models.LoginLog
	result := db.Find(&logs)
	assert.NoError(t, result.Error)
	assert.NotNil(t, logs[0].LogoutAt)
}

func TestAuditService_Log(t *testing.T) {
	db := setupTestDB(t)
	svc := &AuditService{}

	log := &models.AuditLog{
		UserID:    "user-1",
		Username:  "testuser",
		Type:      models.AuditLogTypeCreate,
		Action:    "create_user",
		Resource:  "/admin/api/users",
		Method:    "POST",
		Path:      "/admin/api/users",
		IP:        "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		Status:    "enabled",
		Message:   "Created user successfully",
		Duration:  150,
	}

	err := svc.Log(db, log)
	assert.NoError(t, err)

	var logs []models.AuditLog
	result := db.Find(&logs)
	assert.NoError(t, result.Error)
	assert.Len(t, logs, 1)
	assert.Equal(t, models.AuditLogTypeCreate, logs[0].Type)
	assert.Equal(t, "create_user", logs[0].Action)
	assert.NotZero(t, logs[0].CreatedAt)
}

func TestAuditService_GetLoginLogs(t *testing.T) {
	db := setupTestDB(t)
	svc := &AuditService{}

	for i := 0; i < 5; i++ {
		err := svc.LogLogin(db, "user-1", "testuser", "192.168.1.1", "Mozilla/5.0", "Login", true)
		assert.NoError(t, err)
	}

	logs, total, err := svc.GetLoginLogs(db, "user-1", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, logs, 5)
}

func TestAuditService_GetAuditLogs(t *testing.T) {
	db := setupTestDB(t)
	svc := &AuditService{}

	for i := 0; i < 3; i++ {
		log := &models.AuditLog{
			UserID:   "user-1",
			Username: "testuser",
			Type:     models.AuditLogTypeCreate,
			Action:   "create",
			Resource: "/test",
			Method:   "POST",
			Path:     "/test",
			IP:       "192.168.1.1",
			Status:   "enabled",
		}
		err := svc.Log(db, log)
		assert.NoError(t, err)
	}

	for i := 0; i < 2; i++ {
		log := &models.AuditLog{
			UserID:   "user-1",
			Username: "testuser",
			Type:     models.AuditLogTypeUpdate,
			Action:   "update",
			Resource: "/test",
			Method:   "PUT",
			Path:     "/test",
			IP:       "192.168.1.1",
			Status:   "enabled",
		}
		err := svc.Log(db, log)
		assert.NoError(t, err)
	}

	logs, total, err := svc.GetAuditLogs(db, "user-1", "", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)

	logs, total, err = svc.GetAuditLogs(db, "user-1", models.AuditLogTypeCreate, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, logs, 3)
}

func TestAuditService_CleanOldLogs(t *testing.T) {
	db := setupTestDB(t)
	svc := &AuditService{}

	oldLog := &models.AuditLog{
		UserID:    "user-1",
		Type:      models.AuditLogTypeCreate,
		Action:    "old_action",
		CreatedAt: time.Now().AddDate(0, 0, -31),
	}
	err := db.Create(oldLog).Error
	assert.NoError(t, err)

	newLog := &models.AuditLog{
		UserID:    "user-1",
		Type:      models.AuditLogTypeCreate,
		Action:    "new_action",
		CreatedAt: time.Now(),
	}
	err = db.Create(newLog).Error
	assert.NoError(t, err)

	err = svc.CleanOldLogs(db, 30)
	assert.NoError(t, err)

	var logs []models.AuditLog
	result := db.Find(&logs)
	assert.NoError(t, result.Error)
	assert.Len(t, logs, 1)
	assert.Equal(t, "new_action", logs[0].Action)
}
