package config

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidConfiguration classifies malformed or contradictory runtime
	// configuration. Returned errors deliberately omit the rejected value.
	ErrInvalidConfiguration = errors.New("invalid runtime configuration")
	// ErrSecretUnavailable classifies a syntactically valid SecretRef that could
	// not be resolved to non-empty secret material.
	ErrSecretUnavailable = errors.New("runtime secret unavailable")
)

// ConfigurationError identifies the invalid field without retaining its
// value. Path and Reason must describe structure only; callers can therefore
// safely include this error in diagnostics.
type ConfigurationError struct {
	Path   string
	Reason string
}

func (e *ConfigurationError) Error() string {
	if e == nil {
		return ErrInvalidConfiguration.Error()
	}
	return fmt.Sprintf("runtime config %s: %s", e.Path, e.Reason)
}

func (e *ConfigurationError) Unwrap() error {
	return ErrInvalidConfiguration
}

func invalid(path, reason string) error {
	return &ConfigurationError{Path: path, Reason: reason}
}

// SecretResolutionError contains only a source kind and a non-reversible
// reference fingerprint. Neither the reference locator, resolved value, nor a
// resolver-provided error is retained.
type SecretResolutionError struct {
	Path        string
	Source      SecretSource
	Fingerprint string
	Reason      string
}

func (e *SecretResolutionError) Error() string {
	if e == nil {
		return ErrSecretUnavailable.Error()
	}
	return fmt.Sprintf(
		"runtime config %s: secret unavailable (source=%s fingerprint=%s): %s",
		e.Path,
		e.Source,
		e.Fingerprint,
		e.Reason,
	)
}

func (e *SecretResolutionError) Unwrap() error {
	return ErrSecretUnavailable
}
