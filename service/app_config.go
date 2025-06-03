package service

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/spf13/cast"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 22:01:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 22:01:11
 */

type AppConfig struct{}

func (e *AppConfig) Profile(ctx *gin.Context, auth bool) (map[string]gin.H, error) {
	tenant, err := center.GetTenant().GetTenant(ctx)
	if err != nil {
		slog.Error("get tenant error", "err", err)
		return nil, err
	}
	result := make(map[string]gin.H)
	if center.GetCache() != nil {
		groups := make([]string, 0)
		err = center.GetCache().SMembers(ctx, fmt.Sprintf("%s:app-configs", tenant.GetID())).ScanSlice(&groups)
		if err == nil {
			for i := range groups {
				configMap := make(map[string]string)
				configMap, err = center.GetCache().HGetAll(ctx, fmt.Sprintf("%v:app-configs:%s", tenant.GetID(), groups[i])).Result()
				if err != nil {
					slog.Error("get app config group error", "group", groups[i], "err", err)
					break
				}
				for k, v := range configMap {
					if result[groups[i]] == nil {
						result[groups[i]] = make(gin.H)
					}
					result[groups[i]][k] = transferValue(v)
				}
			}
			if err == nil && len(result) > 0 {
				return result, nil
			}
			result = make(map[string]gin.H) // Reset result if cache retrieval fails
		}
	}
	list := make([]*models.AppConfig, 0)
	query := center.GetTenant().GetDB(ctx, &models.AppConfig{})
	if !auth {
		query = query.Where("auth = ?", false)
	}
	err = query.Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i := range list {
		if result[list[i].Group] == nil {
			result[list[i].Group] = make(gin.H)
		}
		result[list[i].Group][list[i].Name] = transferValue(list[i].Value)
	}
	if center.GetCache() != nil {
		for group := range result {
			if len(result[group]) == 0 {
				continue
			}
			data := make(map[string]string)
			for k, v := range result[group] {
				data[k] = cast.ToString(v)
			}
			// Set cache for each group
			err = center.GetCache().HSet(ctx, fmt.Sprintf("%v:app-configs:%s", tenant.GetID(), group), data).Err()
			if err != nil {
				slog.Error("set app config group error", "group", group, "err", err)
				continue
			}
			err = center.GetCache().SAdd(ctx, fmt.Sprintf("%v:app-configs", tenant.GetID()), group).Err()
			if err != nil {
				slog.Error("set app config group error", "group", group, "err", err)
				continue
			}
		}
	}
	return result, nil
}

func transferValue(value string) any {
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		return value
	}
}

func (e *AppConfig) Group(ctx *gin.Context, group string) (map[string]any, error) {
	list := make([]*models.AppConfig, 0)
	err := center.GetTenant().GetDB(ctx, &models.AppConfig{}).
		Where(&models.AppConfig{Group: group}).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	for i := range list {
		result[list[i].Name] = list[i].Value
	}
	return result, nil
}

func (e *AppConfig) CreateOrUpdate(ctx *gin.Context, group string, data map[string]any) error {
	var err error
	if center.GetCache() != nil {
		// Clear cache for the group
		err = center.GetCache().Del(ctx, fmt.Sprintf("%v:app-configs:%s", center.GetTenant().GetID(), group)).Err()
		if err != nil {
			return err
		}
	}
	for k, v := range data {
		err = center.GetAppConfig().SetAppConfig(ctx, fmt.Sprintf("%s:%s", group, k), isAuth(cast.ToString(v)), cast.ToString(v))
		if err != nil {
			return err
		}
	}
	return nil
}

func isAuth(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "auth") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password") ||
		strings.Contains(key, "pwd") ||
		strings.Contains(key, "token")
}
