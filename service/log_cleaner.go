package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"gorm.io/gorm"
)

type LogCleanerService struct{}

var LogCleaner = &LogCleanerService{}

func init() {
	models.TaskFuncMap["log_cleaner"] = func(ctx context.Context, args ...string) error {
		retentionDaysDB := 30
		retentionDaysFiles := 7
		logDir := "logs"

		if len(args) >= 1 {
			if _, err := fmt.Sscanf(args[0], "%d", &retentionDaysDB); err != nil {
				slog.Warn("invalid retention_days_db argument", "arg", args[0], "error", err)
			}
		}
		if len(args) >= 2 {
			if _, err := fmt.Sscanf(args[1], "%d", &retentionDaysFiles); err != nil {
				slog.Warn("invalid retention_days_files argument", "arg", args[1], "error", err)
			}
		}
		if len(args) >= 3 {
			logDir = args[2]
		}

		slog.Info("log_cleaner task starting", "retention_db", retentionDaysDB, "retention_files", retentionDaysFiles, "log_dir", logDir)

		if err := LogCleaner.CleanOldLogs(gormdb.DB, retentionDaysDB); err != nil {
			slog.Error("log_cleaner failed to clean database logs", "error", err)
			return err
		}

		if err := LogCleaner.CleanOldRuntimeLogs(logDir, retentionDaysFiles); err != nil {
			slog.Error("log_cleaner failed to clean runtime log files", "error", err)
		}

		slog.Info("log_cleaner task completed")
		return nil
	}
}

func (s *LogCleanerService) CleanOldLogs(db *gorm.DB, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result := db.Where("created_at < ?", cutoff).Delete(&models.AuditLog{})
	if result.Error != nil {
		slog.Error("Failed to clean audit logs", "error", result.Error)
		return result.Error
	}
	slog.Info("Cleaned audit logs", "count", result.RowsAffected, "older_than", cutoff.Format("2006-01-02"))

	result = db.Where("login_at < ?", cutoff).Delete(&models.LoginLog{})
	if result.Error != nil {
		slog.Error("Failed to clean login logs", "error", result.Error)
		return result.Error
	}
	slog.Info("Cleaned login logs", "count", result.RowsAffected, "older_than", cutoff.Format("2006-01-02"))

	return nil
}

func (s *LogCleanerService) CleanOldRuntimeLogs(logDir string, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	files, err := getLogFiles(logDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.ModTime.Before(cutoff) {
			if err := deleteLogFile(file.Path); err != nil {
				slog.Error("Failed to delete log file", "path", file.Path, "error", err)
				continue
			}
			slog.Info("Deleted old log file", "path", file.Path, "mod_time", file.ModTime.Format("2006-01-02"))
		}
	}
	return nil
}

type LogFileInfo struct {
	Path    string
	ModTime time.Time
}

func getLogFiles(logDir string) ([]LogFileInfo, error) {
	files := []LogFileInfo{}
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, LogFileInfo{
			Path:    filepath.Join(logDir, entry.Name()),
			ModTime: info.ModTime(),
		})
	}
	return files, nil
}

func deleteLogFile(path string) error {
	return os.Remove(path)
}