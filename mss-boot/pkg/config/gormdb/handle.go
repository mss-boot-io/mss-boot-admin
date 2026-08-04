package gormdb

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/search/gorms"
)

var (
	defaultHandleMu sync.RWMutex
	defaultHandle   *Handle

	// DB is the legacy process-wide GORM handle. New code should keep the
	// *Handle returned by Database.Open and inject Handle.DB explicitly.
	DB *gorm.DB
	// Enforcer is the legacy process-wide Casbin enforcer. New code should use
	// the enforcer owned by a Handle.
	Enforcer casbin.IEnforcer
)

// Handle owns one initialized database stack and its primary sql.DB pool.
//
// A Handle is safe to share between goroutines. Close is idempotent. The
// compatibility globals DB and Enforcer do not own the Handle and therefore do
// not close it automatically.
type Handle struct {
	DB       *gorm.DB
	Enforcer casbin.IEnforcer
	Driver   string

	sqlDB     *sql.DB
	closeOnce sync.Once
	closeErr  error
}

func newHandle(db *gorm.DB, enforcer casbin.IEnforcer, driverName string) (*Handle, error) {
	if db == nil {
		return nil, errors.New("gormdb: database is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	return &Handle{
		DB:       db,
		Enforcer: enforcer,
		Driver:   driverName,
		sqlDB:    sqlDB,
	}, nil
}

// SQLDB returns the owned primary database/sql pool.
func (h *Handle) SQLDB() *sql.DB {
	if h == nil {
		return nil
	}
	return h.sqlDB
}

// Close closes the primary database/sql pool exactly once.
func (h *Handle) Close() error {
	if h == nil {
		return nil
	}
	h.closeOnce.Do(func() {
		if h.sqlDB != nil {
			h.closeErr = h.sqlDB.Close()
		}
	})
	return h.closeErr
}

// InstallDefault publishes handle through the deprecated process-wide
// compatibility globals and returns the previously installed handle. Ownership
// remains with the caller; neither handle is closed by this function.
func InstallDefault(handle *Handle) *Handle {
	defaultHandleMu.Lock()
	defer defaultHandleMu.Unlock()

	previous := defaultHandle
	defaultHandle = handle
	if handle == nil {
		DB = nil
		Enforcer = nil
		return previous
	}
	DB = handle.DB
	Enforcer = handle.Enforcer
	if handle.Driver != "" {
		gorms.Driver = handle.Driver
	}
	return previous
}

// ClearDefault clears the compatibility globals only when expected is still
// installed. Passing nil clears the current default unconditionally.
func ClearDefault(expected *Handle) bool {
	defaultHandleMu.Lock()
	defer defaultHandleMu.Unlock()

	if expected != nil && defaultHandle != expected {
		return false
	}
	defaultHandle = nil
	DB = nil
	Enforcer = nil
	return true
}

// DefaultHandle returns the currently installed compatibility handle.
func DefaultHandle() *Handle {
	defaultHandleMu.RLock()
	defer defaultHandleMu.RUnlock()
	return defaultHandle
}
