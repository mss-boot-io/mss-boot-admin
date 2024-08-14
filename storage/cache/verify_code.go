package cache

import (
	"context"
	"fmt"
	"github.com/mss-boot-io/mss-boot-admin/storage"
	"github.com/spf13/cast"
	"math/rand"
	"time"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/8/13 15:33:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/8/13 15:33:16
 */

func generateCode6() int {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return rand.Intn(900000) + 100000
}

// NewVerifyCode create a new verify code
func NewVerifyCode(cache storage.AdapterCache) *VerifyCode {
	return &VerifyCode{Cache: cache}
}

type VerifyCode struct {
	Cache storage.AdapterCache
}

func (v *VerifyCode) GenerateCode(ctx context.Context, key string, expire time.Duration) (string, error) {
	code := generateCode6()
	err := v.Cache.Set(ctx, fmt.Sprintf("verify-code-%s", key), code, expire)
	if err != nil {
		return "", err
	}
	return cast.ToString(code), nil
}

func (v *VerifyCode) VerifyCode(ctx context.Context, key, code string) (bool, error) {
	s, err := v.Cache.Get(ctx, fmt.Sprintf("verify-code-%s", key))
	if err != nil {
		return false, err
	}
	if s == "" {
		return false, nil
	}
	_ = v.Cache.Del(ctx, fmt.Sprintf("verify-code-%s", key))
	return s == code, nil
}
