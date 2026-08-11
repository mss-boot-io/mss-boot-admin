package config

import (
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	secretFingerprintIterations = 16_384
	secretFingerprintSize       = 12
	secretFingerprintSalt       = "mss-boot/runtime/config/secret-ref-fingerprint/v1"
)

// SecretSource identifies a supported secret source without exposing its
// locator. The first runtime-config checkpoint intentionally supports only
// process environment references.
type SecretSource string

const SecretSourceEnv SecretSource = "env"

// SecretRef is a validated typed reference, not secret material. Its fields
// are private and all ordinary formatting/marshalling is redacted. Reference
// returns the raw locator only for resolver implementations and must never be
// included in diagnostics.
type SecretRef struct {
	source      SecretSource
	locator     string
	fingerprint string
}

// ParseSecretRef accepts env://NAME and rejects raw values or malformed names.
func ParseSecretRef(value string) (SecretRef, error) {
	const prefix = "env://"
	if !strings.HasPrefix(value, prefix) {
		return SecretRef{}, invalid("secretRef", "must use a supported typed reference")
	}
	name := strings.TrimPrefix(value, prefix)
	if !validEnvironmentName(name) {
		return SecretRef{}, invalid("secretRef", "contains an invalid environment reference")
	}
	fingerprint, err := secretReferenceFingerprint(SecretSourceEnv, name)
	if err != nil {
		return SecretRef{}, invalid("secretRef", "fingerprint derivation failed")
	}
	return SecretRef{
		source:      SecretSourceEnv,
		locator:     name,
		fingerprint: fingerprint,
	}, nil
}

func secretReferenceFingerprint(source SecretSource, locator string) (string, error) {
	input := string(source) + "\x00" + locator
	digest, err := pbkdf2.Key(sha256.New, input, []byte(secretFingerprintSalt), secretFingerprintIterations, secretFingerprintSize)
	if err != nil {
		return "", err
	}
	return "pbkdf2-sha256:" + hex.EncodeToString(digest), nil
}

func validEnvironmentName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	for index := range len(name) {
		value := name[index]
		if value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || index > 0 && value >= '0' && value <= '9' {
			continue
		}
		return false
	}
	return true
}

// Source returns the non-secret source kind.
func (r SecretRef) Source() SecretSource {
	return r.source
}

// Fingerprint returns a non-reversible identifier suitable for rotation
// diagnostics. It never contains the source locator.
func (r SecretRef) Fingerprint() string {
	return r.fingerprint
}

// Reference returns the canonical source reference for resolver
// implementations. Callers must treat it as sensitive configuration metadata.
func (r SecretRef) Reference() string {
	if r.source == "" || r.locator == "" {
		return ""
	}
	return string(r.source) + "://" + r.locator
}

func (r SecretRef) valid() bool {
	if r.source != SecretSourceEnv || r.locator == "" || r.fingerprint == "" {
		return false
	}
	parsed, err := ParseSecretRef(r.Reference())
	return err == nil && parsed == r
}

func (r SecretRef) String() string {
	if !r.valid() {
		return "SecretRef<invalid>"
	}
	return fmt.Sprintf("SecretRef{source:%q fingerprint:%q}", r.source, r.fingerprint)
}

func (r SecretRef) GoString() string {
	return r.String()
}

func (r SecretRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Source      SecretSource `json:"source"`
		Fingerprint string       `json:"fingerprint"`
	}{Source: r.source, Fingerprint: r.fingerprint})
}

func (r *SecretRef) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return invalid("secretRef", "must be a typed reference string")
	}
	parsed, err := ParseSecretRef(value)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

func (r SecretRef) MarshalYAML() (any, error) {
	return map[string]string{
		"source":      string(r.source),
		"fingerprint": r.fingerprint,
	}, nil
}

func (r *SecretRef) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return invalid("secretRef", "must be a typed reference string")
	}
	parsed, err := ParseSecretRef(node.Value)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

// SecretResolver resolves a typed reference. Implementations must not include
// resolved values in returned errors; Normalize also sanitizes any error it
// receives before returning it.
type SecretResolver interface {
	Resolve(context.Context, SecretRef) (string, error)
}

// SecretResolverFunc adapts a function to SecretResolver.
type SecretResolverFunc func(context.Context, SecretRef) (string, error)

func (f SecretResolverFunc) Resolve(ctx context.Context, ref SecretRef) (string, error) {
	if f == nil {
		return "", &SecretResolutionError{
			Path:        "secretRef",
			Source:      ref.source,
			Fingerprint: ref.fingerprint,
			Reason:      "resolver is not configured",
		}
	}
	return f(ctx, ref)
}

// EnvSecretResolver resolves env://NAME references.
type EnvSecretResolver struct{}

func (EnvSecretResolver) Resolve(ctx context.Context, ref SecretRef) (string, error) {
	if ctx == nil {
		return "", invalid("secretRef", "context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !ref.valid() || ref.source != SecretSourceEnv {
		return "", invalid("secretRef", "uses an unsupported source")
	}
	value, ok := os.LookupEnv(ref.locator)
	if !ok || value == "" {
		return "", &SecretResolutionError{
			Path:        "secretRef",
			Source:      ref.source,
			Fingerprint: ref.fingerprint,
			Reason:      "referenced value is missing",
		}
	}
	return value, nil
}

const maxResolvedSecretBytes = 1 << 20

func resolveSecret(ctx context.Context, resolver SecretResolver, path string, ref SecretRef) (ResolvedSecret, error) {
	if err := ctx.Err(); err != nil {
		return ResolvedSecret{}, err
	}
	value, err := resolver.Resolve(ctx, ref)
	if err != nil {
		// Preserve cancellation classification without retaining a resolver's
		// potentially sensitive wrapper text.
		if errors.Is(err, context.Canceled) {
			return ResolvedSecret{}, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return ResolvedSecret{}, context.DeadlineExceeded
		}
		return ResolvedSecret{}, &SecretResolutionError{
			Path:        path,
			Source:      ref.source,
			Fingerprint: ref.fingerprint,
			Reason:      "resolver failed",
		}
	}
	if value == "" || len(value) > maxResolvedSecretBytes {
		return ResolvedSecret{}, &SecretResolutionError{
			Path:        path,
			Source:      ref.source,
			Fingerprint: ref.fingerprint,
			Reason:      "resolved value is empty or exceeds the size limit",
		}
	}
	return ResolvedSecret{value: value, source: ref.source, fingerprint: ref.fingerprint}, nil
}

// ResolvedSecret retains secret material for a provider builder while making
// ordinary formatting and serialization safe. Reveal is an explicit trust
// boundary and returns an immutable Go string.
type ResolvedSecret struct {
	value       string
	source      SecretSource
	fingerprint string
}

func (s ResolvedSecret) Source() SecretSource {
	return s.source
}

func (s ResolvedSecret) Fingerprint() string {
	return s.fingerprint
}

func (s ResolvedSecret) Reveal() string {
	return s.value
}

func (s ResolvedSecret) String() string {
	if s.source == "" {
		return "ResolvedSecret<none>"
	}
	return fmt.Sprintf("ResolvedSecret{source:%q fingerprint:%q}", s.source, s.fingerprint)
}

func (s ResolvedSecret) GoString() string {
	return s.String()
}

func (s ResolvedSecret) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Source      SecretSource `json:"source"`
		Fingerprint string       `json:"fingerprint"`
	}{Source: s.source, Fingerprint: s.fingerprint})
}

func (s ResolvedSecret) MarshalYAML() (any, error) {
	return map[string]string{
		"source":      string(s.source),
		"fingerprint": s.fingerprint,
	}, nil
}
