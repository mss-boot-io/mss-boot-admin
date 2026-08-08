package service

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/spf13/cast"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/2 00:42:39
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:42:39
 */

type UserConfig struct{}

var ErrUserConfigKeyCaseMismatch = errors.New("user config key casing does not match stored database key")

type UserConfigKeyCaseMismatchError struct {
	UserID       string
	Group        string
	Name         string
	StoredUserID string
	StoredGroup  string
	StoredName   string
}

func (e *UserConfigKeyCaseMismatchError) Error() string {
	return fmt.Sprintf(
		"%s: got %s/%s/%s, stored %s/%s/%s",
		ErrUserConfigKeyCaseMismatch,
		e.UserID,
		e.Group,
		e.Name,
		e.StoredUserID,
		e.StoredGroup,
		e.StoredName,
	)
}

func (e *UserConfigKeyCaseMismatchError) Unwrap() error { return ErrUserConfigKeyCaseMismatch }

func (e *UserConfig) Profile(ctx *gin.Context, userID string) (map[string]gin.H, error) {
	list := make([]*models.UserConfig, 0)
	err := center.GetDB(ctx, &models.UserConfig{}).
		Where("user_id = ?", userID).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]gin.H)
	for i := range list {
		if list[i].UserID != userID {
			continue
		}
		if list[i].Group == ThemeConfigGroup {
			continue
		}
		if _, ok := result[list[i].Group]; !ok {
			result[list[i].Group] = make(gin.H)
		}
		result[list[i].Group][list[i].Name] = list[i].Value
	}
	theme, err := (&Theme{}).UserResource(ctx, userID)
	if err != nil {
		return nil, err
	}
	result[ThemeConfigGroup] = gin.H(themeResourceMap(theme))
	return result, nil
}

func (e *UserConfig) ThemeResource(ctx *gin.Context, userID string) (*dto.ThemeResource, error) {
	return (&Theme{}).UserResource(ctx, userID)
}

func (e *UserConfig) LegacyThemeGroup(
	ctx *gin.Context,
	userID string,
	resource *dto.ThemeResource,
) (map[string]any, error) {
	if resource == nil {
		var err error
		resource, err = (&Theme{}).UserResource(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	return themeOverridesMap(&resource.ThemeOverrides), nil
}

func (e *UserConfig) UpdateTheme(
	ctx *gin.Context,
	userID string,
	data map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	return (&Theme{}).PatchUserResource(ctx, userID, data, expectedRevision)
}

func (e *UserConfig) ResetTheme(
	ctx *gin.Context,
	userID string,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	return (&Theme{}).ResetUserResource(ctx, userID, expectedRevision)
}

func (e *UserConfig) Group(ctx *gin.Context, userID, group string) (map[string]any, error) {
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return nil, err
	}
	if group == ThemeConfigGroup {
		return e.LegacyThemeGroup(ctx, userID, nil)
	}
	list := make([]*models.UserConfig, 0)
	err := center.GetDB(ctx, &models.UserConfig{}).
		Where(models.UserConfig{UserID: userID, Group: group}).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	for i := range list {
		if list[i].UserID != userID {
			continue
		}
		result[list[i].Name] = list[i].Value
	}
	return result, nil
}

func (e *UserConfig) CreateOrUpdate(ctx *gin.Context, userID, group string, data map[string]any) error {
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return err
	}
	if group == ThemeConfigGroup {
		return (&Theme{}).PatchUser(ctx, userID, data)
	}
	keys := make([]string, 0, len(data))
	for name := range data {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		if err := (&models.UserConfig{}).SetUserConfig(
			ctx,
			userID,
			fmt.Sprintf("%s.%s", group, name),
			cast.ToString(data[name]),
		); err != nil {
			return err
		}
	}
	return nil
}

func (e *UserConfig) Reset(ctx *gin.Context, userID, group string) error {
	if err := rejectNonCanonicalThemeGroup(group); err != nil {
		return err
	}
	if group != ThemeConfigGroup {
		return ErrThemeGroupOnly
	}
	return (&Theme{}).ResetUser(ctx, userID)
}
