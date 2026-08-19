package business

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
)

// RequireAppliedMigrations verifies the exact migration identities required by
// one business module. It performs no schema mutation.
func RequireAppliedMigrations(
	ctx context.Context,
	db *gorm.DB,
	ids ...migration.MigrationID,
) error {
	if ctx == nil {
		return errors.New("business migration readiness context is required")
	}
	if db == nil {
		return errors.New("business migration readiness database is required")
	}
	if len(ids) == 0 {
		return nil
	}
	wanted := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		parsed, err := migration.ParseMigrationID(id.String())
		if err != nil {
			return fmt.Errorf("business migration readiness ID: %w", err)
		}
		if _, exists := seen[parsed.String()]; exists {
			continue
		}
		seen[parsed.String()] = struct{}{}
		wanted = append(wanted, parsed.String())
	}

	var applied []migrationmodels.Migration
	if err := db.WithContext(ctx).
		Where("version IN ?", wanted).
		Find(&applied).Error; err != nil {
		return fmt.Errorf("read applied business migrations: %w", err)
	}
	found := make(map[string]struct{}, len(applied))
	for _, version := range applied {
		found[version.Version] = struct{}{}
	}
	missing := make([]string, 0, len(wanted))
	for _, version := range wanted {
		if _, exists := found[version]; !exists {
			missing = append(missing, version)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("unapplied business migrations: %s", strings.Join(missing, ", "))
	}
	return nil
}
