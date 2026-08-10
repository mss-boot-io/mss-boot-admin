// Package config decodes and normalizes strict, immutable Runtime v2 resource
// configuration. It is intentionally additive to pkg/config: legacy callers
// keep their existing behavior while new composition roots opt into explicit
// provider branches, typed secret references, and pure construction plans.
package config
