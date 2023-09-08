package config

import "time"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/12 23:22:37
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/12 23:22:37
 */

type Auth struct {
	Realm       string        `yaml:"realm" json:"realm"`
	Key         string        `yaml:"key" json:"key"`
	IdentityKey string        `yaml:"identityKey" json:"identityKey"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout"`
	MaxRefresh  time.Duration `yaml:"maxRefresh" json:"maxRefresh"`
}
