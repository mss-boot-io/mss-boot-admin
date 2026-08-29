package config

import "github.com/mss-boot-io/mss-boot-admin/admin/presentation"

// Presentation owns startup-only safety controls. RecoveryMode is
// intentionally absent from AppConfig and every Admin mutation endpoint so a
// stored presentation profile cannot disable compiled-default recovery.
type Presentation struct {
	RecoveryMode bool                      `yaml:"recoveryMode" json:"recoveryMode"`
	AdoptionMode presentation.AdoptionMode `yaml:"adoptionMode" json:"adoptionMode"`
	ActivePages  []string                  `yaml:"activePages" json:"activePages"`
}
