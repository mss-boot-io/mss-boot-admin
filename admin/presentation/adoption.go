package presentation

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// AdoptionMode controls whether stored presentation layers are only observed
// or may be returned to a page runtime. It is captured once during startup.
type AdoptionMode string

const (
	AdoptionDisabled AdoptionMode = "disabled"
	AdoptionShadow   AdoptionMode = "shadow"
	AdoptionActive   AdoptionMode = "active"
)

// AdoptionState is the value-free explanation for one effective lookup.
type AdoptionState string

const (
	AdoptionStateRecovery       AdoptionState = "recovery"
	AdoptionStateUnknownPage    AdoptionState = "unknown-page"
	AdoptionStateDisabled       AdoptionState = "disabled"
	AdoptionStateShadow         AdoptionState = "shadow"
	AdoptionStateNotAllowlisted AdoptionState = "not-allowlisted"
	AdoptionStateActive         AdoptionState = "active"
)

// AdoptionDecision is derived only from the immutable startup snapshot.
type AdoptionDecision struct {
	Mode          AdoptionMode
	State         AdoptionState
	RecoveryMode  bool
	Allowlisted   bool
	ResolveLayers bool
	ApplyLayers   bool
}

// AdoptionPolicy is immutable after construction. Its map is package-private
// and every exported collection is returned as a defensive copy.
type AdoptionPolicy struct {
	initialized bool
	mode        AdoptionMode
	recovery    bool
	activePages map[string]struct{}
	pages       []string
}

func NewAdoptionPolicy(
	mode AdoptionMode,
	activePages []string,
	recovery bool,
	registry *Registry,
) (AdoptionPolicy, error) {
	if registry == nil {
		return AdoptionPolicy{}, errors.New("presentation capability registry is required")
	}
	if mode == "" {
		mode = AdoptionDisabled
	}
	switch mode {
	case AdoptionDisabled, AdoptionShadow, AdoptionActive:
	default:
		return AdoptionPolicy{}, fmt.Errorf("unsupported presentation adoption mode %q", mode)
	}

	allowlist := make(map[string]struct{}, len(activePages))
	pages := make([]string, 0, len(activePages))
	for index, configured := range activePages {
		pageKey := strings.TrimSpace(configured)
		if pageKey == "" || pageKey != configured || !identifierPattern.MatchString(pageKey) {
			return AdoptionPolicy{}, fmt.Errorf("invalid presentation activePages[%d] %q", index, configured)
		}
		if _, duplicate := allowlist[pageKey]; duplicate {
			return AdoptionPolicy{}, fmt.Errorf("duplicate presentation active page %q", pageKey)
		}
		if _, registered := registry.Lookup(pageKey); !registered && !recovery {
			return AdoptionPolicy{}, fmt.Errorf("presentation active page %q is not registered", pageKey)
		}
		allowlist[pageKey] = struct{}{}
		pages = append(pages, pageKey)
	}
	sort.Strings(pages)
	return AdoptionPolicy{
		initialized: true,
		mode:        mode,
		recovery:    recovery,
		activePages: allowlist,
		pages:       pages,
	}, nil
}

func MustNewAdoptionPolicy(
	mode AdoptionMode,
	activePages []string,
	recovery bool,
	registry *Registry,
) AdoptionPolicy {
	policy, err := NewAdoptionPolicy(mode, activePages, recovery, registry)
	if err != nil {
		panic(err)
	}
	return policy
}

func (policy AdoptionPolicy) Initialized() bool { return policy.initialized }
func (policy AdoptionPolicy) Mode() AdoptionMode {
	if policy.mode == "" {
		return AdoptionDisabled
	}
	return policy.mode
}
func (policy AdoptionPolicy) RecoveryEnabled() bool { return policy.recovery }

func (policy AdoptionPolicy) ActivePages() []string {
	return append([]string(nil), policy.pages...)
}

func (policy AdoptionPolicy) Decide(pageKey string) AdoptionDecision {
	decision := AdoptionDecision{Mode: policy.Mode()}
	_, decision.Allowlisted = policy.activePages[pageKey]
	if policy.recovery {
		decision.RecoveryMode = true
		decision.State = AdoptionStateRecovery
		return decision
	}
	switch policy.mode {
	case AdoptionShadow:
		decision.State = AdoptionStateShadow
		decision.ResolveLayers = true
	case AdoptionActive:
		if decision.Allowlisted {
			decision.State = AdoptionStateActive
			decision.ResolveLayers = true
			decision.ApplyLayers = true
		} else {
			decision.State = AdoptionStateNotAllowlisted
		}
	default:
		decision.State = AdoptionStateDisabled
	}
	return decision
}
