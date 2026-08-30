package presentation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdoptionPolicyActivePagesSerializesEmptyCollectionAsArray(t *testing.T) {
	registry := MustNewRegistry(validCapability(t))
	policy, err := NewAdoptionPolicy(AdoptionDisabled, nil, false, registry)
	require.NoError(t, err)

	activePages := policy.ActivePages()
	require.NotNil(t, activePages)
	require.Empty(t, activePages)

	raw, err := json.Marshal(struct {
		ActivePages []string `json:"activePages"`
	}{ActivePages: activePages})
	require.NoError(t, err)
	require.JSONEq(t, `{"activePages":[]}`, string(raw))
}

func TestAdoptionPolicySnapshotsModesAllowlistAndRecovery(t *testing.T) {
	capability := validCapability(t)
	registry := MustNewRegistry(capability)

	disabled, err := NewAdoptionPolicy("", nil, false, registry)
	require.NoError(t, err)
	require.Equal(t, AdoptionDecision{
		Mode: AdoptionDisabled, State: AdoptionStateDisabled,
	}, disabled.Decide(capability.PageKey))

	shadow, err := NewAdoptionPolicy(AdoptionShadow, nil, false, registry)
	require.NoError(t, err)
	require.Equal(t, AdoptionDecision{
		Mode: AdoptionShadow, State: AdoptionStateShadow, ResolveLayers: true,
	}, shadow.Decide(capability.PageKey))

	active, err := NewAdoptionPolicy(AdoptionActive, []string{capability.PageKey}, false, registry)
	require.NoError(t, err)
	require.Equal(t, AdoptionDecision{
		Mode: AdoptionActive, State: AdoptionStateActive, Allowlisted: true,
		ResolveLayers: true, ApplyLayers: true,
	}, active.Decide(capability.PageKey))
	require.Equal(t, AdoptionStateNotAllowlisted, active.Decide("other.list").State)

	recovery, err := NewAdoptionPolicy(AdoptionActive, []string{"future.list"}, true, registry)
	require.NoError(t, err)
	require.Equal(t, AdoptionDecision{
		Mode: AdoptionActive, State: AdoptionStateRecovery, RecoveryMode: true,
	}, recovery.Decide(capability.PageKey))
}

func TestAdoptionPolicyRejectsInvalidStartupConfiguration(t *testing.T) {
	registry := MustNewRegistry(validCapability(t))
	tests := []struct {
		name  string
		mode  AdoptionMode
		pages []string
	}{
		{name: "mode", mode: "observe"},
		{name: "unknown page", mode: AdoptionActive, pages: []string{"missing.list"}},
		{name: "duplicate page", mode: AdoptionActive, pages: []string{"orders.list", "orders.list"}},
		{name: "non exact page", mode: AdoptionActive, pages: []string{" orders.list"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewAdoptionPolicy(test.mode, test.pages, false, registry)
			require.Error(t, err)
		})
	}
}
