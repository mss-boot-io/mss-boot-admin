package migration

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/26 11:04:27
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/26 11:04:27
 */

const legacyMigrationIDLength = 13

var (
	ErrInvalidMigrationID   = errors.New("invalid migration ID")
	ErrDuplicateMigrationID = errors.New("duplicate migration ID")
	ErrMigrationNotReady    = errors.New("migration runner is not configured")
)

// MigrationID is the lossless, persisted identity of a migration.
//
// IDs are decimal strings rather than machine-sized integers so their value is
// stable across architectures and can grow without overflowing. Ordering is by
// numeric value (length first, then lexical order), without parsing into an int.
type MigrationID string

func (id MigrationID) String() string {
	return string(id)
}

// ParseMigrationID validates and returns a migration ID without normalizing or
// truncating it.
func ParseMigrationID(value string) (MigrationID, error) {
	id := MigrationID(value)
	if err := id.validate(); err != nil {
		return id, err
	}
	return id, nil
}

func (id MigrationID) validate() error {
	if id == "" {
		return fmt.Errorf("%w: value is empty", ErrInvalidMigrationID)
	}
	if len(id) > 1 && id[0] == '0' {
		return fmt.Errorf("%w %q: leading zeroes are not allowed", ErrInvalidMigrationID, id)
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return fmt.Errorf("%w %q: only decimal digits are allowed", ErrInvalidMigrationID, id)
		}
	}
	return nil
}

// MigrationIDFromFilename reads the complete numeric filename stem before the
// first underscore. It deliberately does not reproduce the pre-v1.1 behavior
// that truncated every filename to 13 characters.
func MigrationIDFromFilename(path string) (MigrationID, error) {
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	value, _, _ := strings.Cut(stem, "_")
	return ParseMigrationID(value)
}

// FilenameMigrationID returns the complete numeric filename stem. Invalid
// values are retained so Register can report them during migration preflight
// instead of panicking in package init. Callers that need immediate validation
// should use MigrationIDFromFilename.
func FilenameMigrationID(path string) MigrationID {
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	value, _, _ := strings.Cut(stem, "_")
	return MigrationID(value)
}

// GetFilename preserves the v1.0 public helper for downstream source
// compatibility. It intentionally returns the old, collision-prone 13-byte int
// value and must not be used by new registrations.
//
// Deprecated: use FilenameMigrationID or MigrationIDFromFilename.
func GetFilename(path string) int {
	base := filepath.Base(path)
	if len(base) < legacyMigrationIDLength {
		return 0
	}
	value, err := strconv.Atoi(base[:legacyMigrationIDLength])
	if err != nil {
		return 0
	}
	return value
}

// V100MigrationIDFromFilename reproduces the persisted ID calculation used by
// mss-boot v1.0.0 for an explicitly identified historical migration. The old
// implementation converted the first 13 filename bytes to int; a non-numeric
// prefix (including the published 10-digit filename) therefore became "0".
// This helper is for reviewed v1.0.0 registrations only and must not be used by
// the generator for new migrations.
func V100MigrationIDFromFilename(path string) (MigrationID, error) {
	base := filepath.Base(path)
	if len(base) < legacyMigrationIDLength {
		return "", fmt.Errorf(
			"%w: v1.0.0 filename %q is shorter than %d bytes",
			ErrInvalidMigrationID,
			base,
			legacyMigrationIDLength,
		)
	}
	legacyValue := base[:legacyMigrationIDLength]
	for _, r := range legacyValue {
		if r < '0' || r > '9' {
			return MigrationID("0"), nil
		}
	}
	legacyValue = strings.TrimLeft(legacyValue, "0")
	if legacyValue == "" {
		legacyValue = "0"
	}
	return MigrationID(legacyValue), nil
}

// MigrationFunc applies one migration. The database passed by MigrateContext
// carries the caller's context.
type MigrationFunc func(db *gorm.DB, version string) error

type Version interface {
	schema.Tabler
	SetVersion(string)
	Done(*gorm.DB) (bool, error)
}
