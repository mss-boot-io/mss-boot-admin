package cache

import (
	"context"
	"errors"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/8/13 15:33:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/8/13 15:33:16
 */

var ErrLegacyVerifyCodeDisabled = errors.New("legacy verification-code store is disabled")

// NewVerifyCode retains the old construction symbol for source compatibility.
// Deprecated: the email-only GET/SET/DEL protocol is unsafe and permanently
// disabled; use the provisional purpose-scoped challenge service instead.
func NewVerifyCode(cache storage.AdapterCache) *VerifyCode {
	return &VerifyCode{Cache: cache}
}

type VerifyCode struct {
	Cache storage.AdapterCache
}

func (v *VerifyCode) GenerateCode(ctx context.Context, key string, expire time.Duration) (string, error) {
	return "", ErrLegacyVerifyCodeDisabled
}

func (v *VerifyCode) VerifyCode(ctx context.Context, key, code string) (bool, error) {
	return false, ErrLegacyVerifyCodeDisabled
}
