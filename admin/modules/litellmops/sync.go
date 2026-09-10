package litellmops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// SyncReport summarizes one synchronization pass.
type SyncReport struct {
	Users        int       `json:"users"`
	Keys         int       `json:"keys"`
	UsersRetired int       `json:"users_retired"`
	KeysRetired  int       `json:"keys_retired"`
	SyncedAt     time.Time `json:"synced_at"`
}

// SyncSnapshots refreshes the local read-only snapshots from the LiteLLM
// Admin API. Records missing upstream are soft-deleted; records that reappear
// are restored. Complete key material never leaves the request scope — only
// the hash prefix is persisted.
func SyncSnapshots(ctx context.Context, db *gorm.DB, client *Client) (*SyncReport, error) {
	if db == nil || client == nil {
		return nil, errors.New("litellmops sync requires a database and a LiteLLM client")
	}
	users, err := client.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := client.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	emailByUser := make(map[string]string, len(users))
	for _, user := range users {
		emailByUser[user.UserID] = user.Email
	}
	now := time.Now().UTC()
	report := &SyncReport{SyncedAt: now}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seenUsers := make([]string, 0, len(users))
		for _, user := range users {
			seenUsers = append(seenUsers, user.UserID)
			modelsJSON, marshalErr := json.Marshal(user.Models)
			if marshalErr != nil {
				return marshalErr
			}
			values := map[string]any{
				"user_id":         user.UserID,
				"email":           user.Email,
				"user_role":       user.Role,
				"models":          string(modelsJSON),
				"max_budget":      user.MaxBudget,
				"budget_duration": user.BudgetDuration,
				"budget_reset_at": user.BudgetResetAt,
				"spend":           user.Spend,
				"tpm_limit":       user.TPMLimit,
				"rpm_limit":       user.RPMLimit,
				"blocked":         user.Blocked,
				"synced_at":       now,
				"updated_at":      now,
				"deleted_at":      nil,
			}
			if err := upsertSnapshot(tx, &UserSnapshot{}, "user_id = ?", user.UserID, values); err != nil {
				return err
			}
		}
		report.Users = len(seenUsers)

		seenKeys := make([]string, 0, len(keys))
		for _, key := range keys {
			prefix := key.Prefix()
			if prefix == "" {
				continue
			}
			seenKeys = append(seenKeys, prefix)
			var alias *string
			if trimmed := key.Alias; trimmed != "" {
				alias = &trimmed
			}
			modelsJSON, marshalErr := json.Marshal(key.Models)
			if marshalErr != nil {
				return marshalErr
			}
			values := map[string]any{
				"key_hash_prefix":       prefix,
				"alias":                 alias,
				"user_id":               key.UserID,
				"user_email":            emailByUser[key.UserID],
				"max_budget":            key.MaxBudget,
				"spend":                 key.Spend,
				"tpm_limit":             key.TPMLimit,
				"rpm_limit":             key.RPMLimit,
				"max_parallel_requests": key.MaxParallel,
				"expires":               key.Expires,
				"is_session_key":        key.IsSession(),
				"models":                string(modelsJSON),
				"blocked":               key.Blocked,
				"synced_at":             now,
				"updated_at":            now,
				"deleted_at":            nil,
			}
			if err := upsertSnapshot(tx, &KeySnapshot{}, "key_hash_prefix = ?", prefix, values); err != nil {
				return err
			}
		}
		report.Keys = len(seenKeys)

		if report.UsersRetired, err = retireMissing(tx, &UserSnapshot{}, "user_id", seenUsers, now); err != nil {
			return err
		}
		if report.KeysRetired, err = retireMissing(tx, &KeySnapshot{}, "key_hash_prefix", seenKeys, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// upsertSnapshot inserts or updates one snapshot row by its business key and
// clears any soft-delete marker so reappearing records are restored.
func upsertSnapshot(db *gorm.DB, model any, where string, arg any, values map[string]any) error {
	probe := db.Unscoped().Model(model).Where(where, arg).Limit(1).Find(model)
	if probe.Error != nil {
		return probe.Error
	}
	if probe.RowsAffected == 0 {
		values["id"] = newSnapshotID()
		values["created_at"] = values["updated_at"]
		return db.Model(model).Create(values).Error
	}
	return db.Unscoped().Model(model).Where(where, arg).Updates(values).Error
}

// retireMissing soft-deletes snapshots that disappeared upstream.
func retireMissing(db *gorm.DB, model any, column string, keep []string, now time.Time) (int, error) {
	tx := db.Unscoped().Model(model).Where("deleted_at IS NULL")
	if len(keep) > 0 {
		tx = tx.Where(column+" NOT IN ?", keep)
	}
	result := tx.Update("deleted_at", now)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// newSnapshotID returns a 128-bit random hex identifier without adding a
// dependency for UUID generation.
func newSnapshotID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		panic("litellmops: crypto/rand unavailable")
	}
	return hex.EncodeToString(buffer)
}
