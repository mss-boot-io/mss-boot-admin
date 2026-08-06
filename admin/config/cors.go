package config

import "time"

// CORS defines the browser origins that may call the Admin API. Credentialed
// requests require exact origins; wildcard origins are rejected by the router.
type CORS struct {
	AllowOrigins  []string      `yaml:"allowOrigins" json:"allowOrigins"`
	AllowMethods  []string      `yaml:"allowMethods" json:"allowMethods"`
	AllowHeaders  []string      `yaml:"allowHeaders" json:"allowHeaders"`
	ExposeHeaders []string      `yaml:"exposeHeaders" json:"exposeHeaders"`
	MaxAge        time.Duration `yaml:"maxAge" json:"maxAge"`
}
