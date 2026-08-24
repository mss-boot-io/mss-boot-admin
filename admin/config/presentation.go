package config

// Presentation owns startup-only safety controls. RecoveryMode is
// intentionally absent from AppConfig and every Admin mutation endpoint so a
// stored presentation profile cannot disable compiled-default recovery.
type Presentation struct {
	RecoveryMode bool `yaml:"recoveryMode" json:"recoveryMode"`
}
