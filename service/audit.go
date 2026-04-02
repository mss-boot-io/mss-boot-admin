package service

import (
	"time"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

type AuditService struct{}

var Audit = &AuditService{}

func (s *AuditService) LogLogin(db *gorm.DB, userID, username, ip, userAgent, message string, success bool) error {
	status := enum.Status("enabled")
	if !success {
		status = enum.Status("disabled")
	}

	log := &models.LoginLog{
		UserID:    userID,
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Status:    status,
		Message:   message,
		LoginAt:   time.Now(),
	}

	return db.Create(log).Error
}

func (s *AuditService) LogLogout(db *gorm.DB, userID string) error {
	now := time.Now()
	return db.Model(&models.LoginLog{}).
		Where("user_id = ? AND logout_at IS NULL", userID).
		Update("logout_at", now).Error
}

func (s *AuditService) Log(db *gorm.DB, log *models.AuditLog) error {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	return db.Create(log).Error
}

func (s *AuditService) LogWithContext(db *gorm.DB, logType models.AuditLogType, userID, username, action, resource, method, path, ip, userAgent string, status enum.Status, message string, duration int64) error {
	return s.Log(db, &models.AuditLog{
		Type:      logType,
		UserID:    userID,
		Username:  username,
		Action:    action,
		Resource:  resource,
		Method:    method,
		Path:      path,
		IP:        ip,
		UserAgent: userAgent,
		Status:    status,
		Message:   message,
		Duration:  duration,
	})
}

func (s *AuditService) GetLoginLogs(db *gorm.DB, userID string, page, pageSize int) ([]*models.LoginLog, int64, error) {
	var logs []*models.LoginLog
	var total int64

	query := db.Model(&models.LoginLog{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("login_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (s *AuditService) GetAuditLogs(db *gorm.DB, userID string, logType models.AuditLogType, page, pageSize int) ([]*models.AuditLog, int64, error) {
	var logs []*models.AuditLog
	var total int64

	query := db.Model(&models.AuditLog{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if logType != "" {
		query = query.Where("type = ?", logType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (s *AuditService) CleanOldLogs(db *gorm.DB, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)

	if err := db.Where("created_at < ?", cutoff).Delete(&models.AuditLog{}).Error; err != nil {
		return err
	}

	return db.Where("login_at < ?", cutoff).Delete(&models.LoginLog{}).Error
}
