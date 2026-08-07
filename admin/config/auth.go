package config

import (
	"errors"
	"strings"
	"time"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/12 23:22:37
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/12 23:22:37
 */

type Auth struct {
	Realm          string        `yaml:"realm" json:"realm"`
	Key            string        `yaml:"key" json:"key"`
	IdentityKey    string        `yaml:"identityKey" json:"identityKey"`
	Timeout        time.Duration `yaml:"timeout" json:"timeout"`
	MaxRefresh     time.Duration `yaml:"maxRefresh" json:"maxRefresh"`
	SessionEnabled bool          `yaml:"sessionEnabled" json:"sessionEnabled"`
}

const insecureDefaultAuthKey = "mss-boot-admin-secret"

func validateProductionAuthKey(mode Mode, key string) error {
	if mode != ModeProd {
		return nil
	}
	normalized := strings.TrimSpace(key)
	if normalized == "" || normalized == insecureDefaultAuthKey {
		return errors.New("production auth.key must override the insecure development default")
	}
	if len(normalized) < 32 {
		return errors.New("production auth.key must contain at least 32 bytes of entropy-bearing material")
	}
	return nil
}
