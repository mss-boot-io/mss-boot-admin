package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

type revisionTestLogger struct {
	logger.Interface
	recordNotFound atomic.Int64
}

func (e *revisionTestLogger) Trace(
	ctx context.Context,
	begin time.Time,
	fc func() (string, int64),
	err error,
) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		e.recordNotFound.Add(1)
	}
	e.Interface.Trace(ctx, begin, fc, err)
}

func TestReadConfigRevisionMissingIsZeroWithoutRecordNotFoundLog(t *testing.T) {
	observer := &revisionTestLogger{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: observer})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ConfigRevision{}))

	revision, err := readConfigRevision(db, applicationThemeRevisionKey())
	require.NoError(t, err)
	require.Zero(t, revision)
	require.Zero(t, observer.recordNotFound.Load())
}
