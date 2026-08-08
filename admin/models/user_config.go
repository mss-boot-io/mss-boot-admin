package models

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/2 00:21:01
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:21:01
 */

type UserConfig struct {
	ModelGormTenant
	// UserID 用户id
	UserID string `json:"userID" gorm:"size:64;index;default:'';not null;comment:用户id" binding:"required"`
	// Name 名称
	Name string `json:"name" gorm:"size:128;index;default:'';not null;comment:名称" binding:"required"`
	// Group 分组
	Group string `json:"group" gorm:"size:128;default:'';not null;comment:分组" binding:"required"`
	// Value 值
	Value string `json:"value" gorm:"size:255;default:'';not null;comment:值"`
}

var (
	ErrUserConfigIdentityMismatch = errors.New("user config key does not exactly match stored database identity")
	ErrUserConfigRevisionChanged  = errors.New("user config revision changed concurrently")
)

func (*UserConfig) TableName() string {
	return "mss_boot_user_configs"
}

func (e *UserConfig) SetUserConfig(ctx *gin.Context, userID, key string, value string) error {
	if key == "" {
		return nil
	}
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("%w: user id is empty", ErrUserConfigIdentityMismatch)
	}

	var group string
	keys := strings.Split(key, ".")
	if len(keys) > 1 {
		group = keys[0]
		key = strings.Join(keys[1:], ".")
	}
	if key == "" {
		return fmt.Errorf("%w: name is empty", ErrUserConfigIdentityMismatch)
	}
	canonicalGroup := strings.ToLower(strings.TrimSpace(norm.NFKC.String(group)))
	if canonicalGroup == ConfigRevisionResourceTheme && group != ConfigRevisionResourceTheme {
		return fmt.Errorf("%w: theme group must use canonical casing", ErrUserConfigIdentityMismatch)
	}

	db := center.GetDB(ctx, e).Clauses(dbresolver.Write)
	return db.Transaction(func(tx *gorm.DB) error {
		profileRevision, err := lockUserConfigRevision(tx, userID, ConfigRevisionResourceUserProfile)
		if err != nil {
			return err
		}
		var themeRevision int64
		if group == ConfigRevisionResourceTheme {
			themeRevision, err = lockUserConfigRevision(tx, userID, ConfigRevisionResourceTheme)
			if err != nil {
				return err
			}
		}
		if err = upsertExactUserConfig(tx, userID, group, key, value); err != nil {
			return err
		}
		if _, err = advanceUserConfigRevision(tx, userID, ConfigRevisionResourceUserProfile, profileRevision); err != nil {
			return err
		}
		if group == ConfigRevisionResourceTheme {
			if _, err = advanceUserConfigRevision(tx, userID, ConfigRevisionResourceTheme, themeRevision); err != nil {
				return err
			}
		}
		return nil
	})
}

func lockUserConfigRevision(tx *gorm.DB, userID, resource string) (int64, error) {
	row := &ConfigRevision{
		Scope: ConfigRevisionScopeUser, OwnerID: userID, Resource: resource, UpdatedAt: time.Now(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return 0, err
	}
	row = &ConfigRevision{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		ConfigRevisionScopeUser,
		userID,
		resource,
	).Take(row).Error
	if err != nil {
		return 0, err
	}
	if row.Scope != ConfigRevisionScopeUser || row.OwnerID != userID || row.Resource != resource {
		return 0, fmt.Errorf(
			"%w: requested scope=%q resource=%q, stored scope=%q resource=%q, ownerMatch=%t",
			ErrUserConfigIdentityMismatch,
			ConfigRevisionScopeUser,
			resource,
			row.Scope,
			row.Resource,
			row.OwnerID == userID,
		)
	}
	if row.Revision < 0 {
		return 0, fmt.Errorf("user config revision is negative for resource %q", resource)
	}
	return row.Revision, nil
}

func advanceUserConfigRevision(tx *gorm.DB, userID, resource string, current int64) (int64, error) {
	if current < 0 || current == int64(^uint64(0)>>1) {
		return 0, fmt.Errorf("user config revision cannot advance for resource %q", resource)
	}
	next := current + 1
	result := tx.Model(&ConfigRevision{}).
		Where(
			"scope = ? AND owner_id = ? AND resource = ? AND revision = ?",
			ConfigRevisionScopeUser,
			userID,
			resource,
			current,
		).
		Updates(map[string]any{"revision": next, "updated_at": time.Now()})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, ErrUserConfigRevisionChanged
	}
	return next, nil
}

func upsertExactUserConfig(tx *gorm.DB, userID, group, name, value string) error {
	candidates, err := exactUserConfigCandidates(tx, userID, group, name)
	if err != nil {
		return err
	}
	if len(candidates) > 1 {
		return fmt.Errorf(
			"%w: group=%q name=%q matched %d rows",
			ErrUserConfigIdentityMismatch,
			group,
			name,
			len(candidates),
		)
	}
	now := time.Now()
	if len(candidates) == 1 {
		candidate := candidates[0]
		if candidate.UserID != userID || candidate.Group != group || candidate.Name != name {
			return fmt.Errorf(
				"%w: requested group=%q name=%q, stored group=%q name=%q, ownerMatch=%t",
				ErrUserConfigIdentityMismatch,
				group,
				name,
				candidate.Group,
				candidate.Name,
				candidate.UserID == userID,
			)
		}
		result := tx.Unscoped().Model(&UserConfig{}).
			Where("id = ?", candidate.ID).
			Updates(map[string]any{
				"user_id":    userID,
				"group":      group,
				"name":       name,
				"value":      value,
				"updated_at": now,
				"deleted_at": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update user config %q/%q: row disappeared", group, name)
		}
		return nil
	}
	record := &UserConfig{UserID: userID, Group: group, Name: name, Value: value}
	record.UpdatedAt = now
	return tx.Create(record).Error
}

func exactUserConfigCandidates(tx *gorm.DB, userID, group, name string) ([]UserConfig, error) {
	newQuery := func() *gorm.DB { return tx.Session(&gorm.Session{NewDB: true}).Unscoped() }
	var exact []UserConfig
	if err := newQuery().
		Where("user_id = ?", userID).
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: group}).
		Where("name = ?", name).
		Find(&exact).Error; err != nil {
		return nil, err
	}
	fallback := newQuery()
	quotedUserID := fallback.Statement.Quote("user_id")
	quotedGroup := fallback.Statement.Quote("group")
	quotedName := fallback.Statement.Quote("name")
	var trailingAliases []UserConfig
	if err := fallback.Where(
		fmt.Sprintf("RTRIM(%s) = ? AND RTRIM(%s) = ? AND RTRIM(%s) = ?", quotedUserID, quotedGroup, quotedName),
		userID,
		group,
		name,
	).Find(&trailingAliases).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]UserConfig, len(exact)+len(trailingAliases))
	for _, row := range exact {
		byID[row.ID] = row
	}
	for _, row := range trailingAliases {
		byID[row.ID] = row
	}
	result := make([]UserConfig, 0, len(byID))
	for _, row := range byID {
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func getUserConfig(ctx *gin.Context, userID, key string) (*UserConfig, error) {
	c := &UserConfig{}
	if key == "" {
		return nil, fmt.Errorf("key is empty")
	}

	var group string
	keys := strings.Split(key, ".")
	if len(keys) > 1 {
		group = keys[0]
		key = strings.Join(keys[1:], ".")
	}
	err := center.GetDB(ctx, c).
		Where("user_id = ?", userID).
		Where("name = ?", key).
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: group}).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (e *UserConfig) GetUserConfig(ctx *gin.Context, userID, key string) (string, bool) {
	c, err := getUserConfig(ctx, userID, key)
	if err != nil {
		return "", false
	}
	return c.Value, true
}
