package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	configRevisionResourceTheme         = "theme"
	configRevisionResourcePublicProfile = "public-profile"
)

var errConfigRevisionChanged = errors.New("configuration revision changed concurrently")

type configRevisionKey struct {
	scope    string
	ownerID  string
	resource string
}

func readConfigRevision(db *gorm.DB, key configRevisionKey) (int64, error) {
	var row models.ConfigRevision
	result := db.Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		key.scope,
		key.ownerID,
		key.resource,
	).Limit(1).Find(&row)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, nil
	}
	if row.Revision < 0 {
		return 0, fmt.Errorf("configuration revision is negative for %s/%s", key.scope, key.resource)
	}
	return row.Revision, nil
}

func readConfigRevisions(db *gorm.DB, keys ...configRevisionKey) (map[configRevisionKey]int64, error) {
	result := make(map[configRevisionKey]int64, len(keys))
	for _, key := range keys {
		revision, err := readConfigRevision(db, key)
		if err != nil {
			return nil, err
		}
		result[key] = revision
	}
	return result, nil
}

func lockConfigRevision(tx *gorm.DB, key configRevisionKey) (int64, error) {
	row := &models.ConfigRevision{
		Scope:     key.scope,
		OwnerID:   key.ownerID,
		Resource:  key.resource,
		Revision:  0,
		UpdatedAt: time.Now(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return 0, err
	}

	row = &models.ConfigRevision{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		key.scope,
		key.ownerID,
		key.resource,
	).Take(row).Error
	if err != nil {
		return 0, err
	}
	if row.Revision < 0 {
		return 0, fmt.Errorf("configuration revision is negative for %s/%s", key.scope, key.resource)
	}
	return row.Revision, nil
}

func advanceConfigRevision(tx *gorm.DB, key configRevisionKey, current int64) (int64, error) {
	if current < 0 || current == int64(^uint64(0)>>1) {
		return 0, fmt.Errorf("configuration revision cannot advance for %s/%s", key.scope, key.resource)
	}
	next := current + 1
	result := tx.Model(&models.ConfigRevision{}).
		Where(
			"scope = ? AND owner_id = ? AND resource = ? AND revision = ?",
			key.scope,
			key.ownerID,
			key.resource,
			current,
		).
		Updates(map[string]any{
			"revision":   next,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, errConfigRevisionChanged
	}
	return next, nil
}
