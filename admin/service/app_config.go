package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 22:01:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 22:01:11
 */

type AppConfig struct{}

const (
	// The shared Redis hash tag keeps the atomic two-key scripts compatible
	// with Redis Cluster as well as single-node and sentinel deployments.
	appConfigPublicProfileCacheKey        = "app-configs:{profile:public}:payload"
	appConfigPublicProfileGenerationKey   = "app-configs:{profile:public}:generation"
	appConfigPublicProfileMaxReadAttempts = 3

	cachePublicProfileIfCurrentScript = `
local generation = redis.call("GET", KEYS[1])
if not generation then
  generation = "0"
end
if generation ~= ARGV[1] then
  return 0
end
redis.call("SET", KEYS[2], ARGV[2])
return 1
`
	advancePublicProfileGenerationScript = `
local generation = redis.call("INCR", KEYS[1])
redis.call("DEL", KEYS[2])
return generation
`
)

type appConfigPublicKey struct {
	Group string
	Name  string
}

// publicAppConfigKeys is the browser bootstrap contract. Additions require a
// review of the frontend consumer and the client-configuration security
// contract; every key not listed here is private by default.
var publicAppConfigKeys = []appConfigPublicKey{
	{Group: "base", Name: "websiteName"},
	{Group: "base", Name: "websiteDescription"},
	{Group: "base", Name: "websiteLogo"},
	{Group: "base", Name: "websiteRecordNumber"},
	{Group: "base", Name: "websiteCopyRight"},
	{Group: "security", Name: "registerEnabled"},
	{Group: "security", Name: "phoneEnabled"},
	{Group: "security", Name: "emailEnabled"},
	{Group: "security", Name: "githubEnabled"},
	{Group: "security", Name: "larkEnabled"},
	{Group: "theme", Name: "navTheme"},
	{Group: "theme", Name: "layout"},
	{Group: "theme", Name: "contentWidth"},
	{Group: "theme", Name: "fixedHeader"},
	{Group: "theme", Name: "fixSiderbar"},
	{Group: "theme", Name: "colorWeak"},
	{Group: "theme", Name: "pwa"},
	{Group: "theme", Name: "colorPrimary"},
}

var publicAppConfigKeySet = func() map[appConfigPublicKey]struct{} {
	result := make(map[appConfigPublicKey]struct{}, len(publicAppConfigKeys))
	for _, key := range publicAppConfigKeys {
		result[key] = struct{}{}
	}
	return result
}()

type publicAppConfigCacheEnvelope struct {
	Generation int64            `json:"generation"`
	Profile    map[string]gin.H `json:"profile"`
}

// Profile returns the public application bootstrap projection. The auth
// argument is retained for source compatibility, but caller identity must
// never widen this endpoint to include authenticated-only or secret values.
func (e *AppConfig) Profile(ctx *gin.Context, _ bool) (map[string]gin.H, error) {
	for attempt := 0; attempt < appConfigPublicProfileMaxReadAttempts; attempt++ {
		generation, cacheAvailable := getPublicProfileGeneration(ctx)
		if cacheAvailable {
			if cached, ok := getCachedPublicProfile(ctx, generation); ok {
				return cached, nil
			}
		}

		result, err := loadPublicProfile(ctx)
		if err != nil {
			return nil, err
		}
		if !cacheAvailable {
			return result, nil
		}

		stored, err := cachePublicProfile(ctx, generation, result)
		if err != nil {
			slog.Warn("set public app config profile cache", "err", err)
			return result, nil
		}
		if stored {
			return result, nil
		}
	}

	return nil, errors.New("app config profile changed during read")
}

func getPublicProfileGeneration(ctx *gin.Context) (int64, bool) {
	cache := center.GetCache()
	if cache == nil {
		return 0, false
	}
	value, err := cache.Get(ctx, appConfigPublicProfileGenerationKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, true
		}
		slog.Warn("get public app config profile cache generation", "err", err)
		return 0, false
	}
	generation, err := strconv.ParseInt(value, 10, 64)
	if err != nil || generation < 0 {
		slog.Warn("decode public app config profile cache generation", "value", value, "err", err)
		return 0, false
	}
	return generation, true
}

func loadPublicProfile(ctx *gin.Context) (map[string]gin.H, error) {
	list := make([]*models.AppConfig, 0, len(publicAppConfigKeys))
	query := center.GetDB(ctx, &models.AppConfig{}).Where("1 = 0")
	for _, key := range publicAppConfigKeys {
		query = query.Or(&models.AppConfig{Group: key.Group, Name: key.Name})
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}

	result := make(map[string]gin.H)
	for i := range list {
		if !isPublicAppConfigKey(list[i].Group, list[i].Name) {
			continue
		}
		if result[list[i].Group] == nil {
			result[list[i].Group] = make(gin.H)
		}
		result[list[i].Group][list[i].Name] = transferValue(list[i].Value)
	}
	return result, nil
}

func getCachedPublicProfile(ctx *gin.Context, generation int64) (map[string]gin.H, bool) {
	cache := center.GetCache()
	if cache == nil {
		return nil, false
	}
	payload, err := cache.Get(ctx, appConfigPublicProfileCacheKey).Bytes()
	if err != nil {
		return nil, false
	}
	envelope := &publicAppConfigCacheEnvelope{}
	if err = json.Unmarshal(payload, envelope); err != nil {
		slog.Warn("decode public app config profile cache", "err", err)
		return nil, false
	}
	if envelope.Generation != generation || envelope.Profile == nil {
		return nil, false
	}
	return publicProfileProjection(envelope.Profile), true
}

func cachePublicProfile(ctx *gin.Context, generation int64, profile map[string]gin.H) (bool, error) {
	cache := center.GetCache()
	if cache == nil {
		return false, nil
	}
	payload, err := json.Marshal(&publicAppConfigCacheEnvelope{
		Generation: generation,
		Profile:    publicProfileProjection(profile),
	})
	if err != nil {
		return false, err
	}
	result, err := cache.Eval(
		ctx,
		cachePublicProfileIfCurrentScript,
		[]string{appConfigPublicProfileGenerationKey, appConfigPublicProfileCacheKey},
		strconv.FormatInt(generation, 10),
		payload,
	).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func publicProfileProjection(profile map[string]gin.H) map[string]gin.H {
	result := make(map[string]gin.H, len(profile))
	for group, values := range profile {
		for name, value := range values {
			if !isPublicAppConfigKey(group, name) {
				continue
			}
			if result[group] == nil {
				result[group] = make(gin.H)
			}
			result[group][name] = value
		}
	}
	return result
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
	err := center.GetDB(ctx, &models.AppConfig{}).
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

func (e *AppConfig) CreateOrUpdate(ctx *gin.Context, group string, data map[string]any) (err error) {
	// Invalidate after the writes so a concurrent cache fill with the old DB
	// values is also removed. A failed partial update must invalidate as well.
	defer func() {
		if cacheErr := invalidatePublicProfile(ctx); cacheErr != nil {
			err = errors.Join(err, cacheErr)
		}
	}()

	for k, v := range data {
		err = center.GetAppConfig().SetAppConfig(
			ctx,
			fmt.Sprintf("%s:%s", group, k),
			!isPublicAppConfigKey(group, k),
			cast.ToString(v),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func invalidatePublicProfile(ctx *gin.Context) error {
	cache := center.GetCache()
	if cache == nil {
		return nil
	}
	return cache.Eval(
		ctx,
		advancePublicProfileGenerationScript,
		[]string{appConfigPublicProfileGenerationKey, appConfigPublicProfileCacheKey},
	).Err()
}

func isPublicAppConfigKey(group, name string) bool {
	_, ok := publicAppConfigKeySet[appConfigPublicKey{Group: group, Name: name}]
	return ok
}
