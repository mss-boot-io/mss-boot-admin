package cache

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxIdentityBytes  = 1024
	maxNamespaceBytes = 63
	maxAllowedPayload = 16 << 20
)

var (
	ErrInvalidPolicy = errors.New("invalid derived cache policy")
	ErrInvalidTarget = errors.New("invalid derived cache target")
	ErrInvalidResult = errors.New("invalid derived cache result")
	ErrClosed        = errors.New("derived cache is closed")
)

// Authority identifies the source that remains correct when the cache is
// empty or unavailable. Runtime v2 intentionally supports only database
// authority in this package.
type Authority string

const AuthorityDatabase Authority = "database"

// FailureMode states how provider failures affect reads and invalidations.
// BypassAuthority never returns a provider failure in place of an authority
// result and never turns a completed database write into an operation failure.
type FailureMode string

const FailureModeBypassAuthority FailureMode = "bypass-authority"

// Reconstruction states how a missing or unusable entry is rebuilt.
type Reconstruction string

const ReconstructionLoader Reconstruction = "loader"

// Policy is immutable after NewDerived or NewQueryCache returns.
type Policy struct {
	Authority       Authority
	Namespace       string
	TTL             time.Duration
	MaxPayloadBytes int
	FailureMode     FailureMode
	Reconstruction  Reconstruction
}

func (p Policy) validate() error {
	switch {
	case p.Authority != AuthorityDatabase:
		return validationError(ErrInvalidPolicy, "authority", "must be database")
	case !validNamespace(p.Namespace):
		return validationError(ErrInvalidPolicy, "namespace", "must be canonical")
	case p.TTL <= 0:
		return validationError(ErrInvalidPolicy, "ttl", "must be positive")
	case p.MaxPayloadBytes <= 0 || p.MaxPayloadBytes > maxAllowedPayload:
		return validationError(ErrInvalidPolicy, "maxPayloadBytes", "is outside the supported bound")
	case p.FailureMode != FailureModeBypassAuthority:
		return validationError(ErrInvalidPolicy, "failureMode", "must bypass to authority")
	case p.Reconstruction != ReconstructionLoader:
		return validationError(ErrInvalidPolicy, "reconstruction", "must use the loader")
	default:
		return nil
	}
}

// Dataset is the invalidation unit. Datasource is a stable logical identity
// shared by every application instance, never a DSN. Table is the logical
// authority table or equivalent revision domain.
type Dataset struct {
	Datasource string
	Table      string
}

// Target identifies one opt-in cached query. QueryIdentity must be a stable,
// non-sensitive identity covering normalized query inputs, preload graph, and
// scan/result shape, not raw SQL or credentials. All fields are digested
// before entering Redis keys.
type Target struct {
	Datasource    string
	Table         string
	QueryIdentity string
}

func (t Target) dataset() Dataset {
	return Dataset{Datasource: t.Datasource, Table: t.Table}
}

func (d Dataset) validate() error {
	if !validIdentity(d.Datasource) {
		return validationError(ErrInvalidTarget, "datasource", "must be a stable logical identity")
	}
	if !validIdentity(d.Table) {
		return validationError(ErrInvalidTarget, "table", "must be a stable logical identity")
	}
	return nil
}

func (t Target) validate() error {
	if err := t.dataset().validate(); err != nil {
		return err
	}
	if !validIdentity(t.QueryIdentity) {
		return validationError(ErrInvalidTarget, "queryIdentity", "must be a stable non-sensitive identity")
	}
	return nil
}

func validIdentity(value string) bool {
	if value == "" || len(value) > maxIdentityBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validNamespace(value string) bool {
	if value == "" || len(value) > maxNamespaceBytes || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && (character == '-' || character == '.') {
			continue
		}
		return false
	}
	return value[len(value)-1] != '-' && value[len(value)-1] != '.' && !strings.Contains(value, "..")
}

type ValidationError struct {
	class  error
	path   string
	reason string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "invalid derived cache value"
	}
	return fmt.Sprintf("derived cache %s: %s", e.path, e.reason)
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.class
}

func validationError(class error, path, reason string) error {
	return &ValidationError{class: class, path: path, reason: reason}
}
