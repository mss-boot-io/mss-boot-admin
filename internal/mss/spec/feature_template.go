package spec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenderFeatureTemplate returns a semantically valid Feature starter contract.
func RenderFeatureTemplate(name, displayName, owner string) ([]byte, error) {
	name = normalizeIdentifier(name)
	if !identifierPattern.MatchString(name) {
		return nil, fmt.Errorf("feature name %q must be lower-case kebab-case", name)
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = titleWords(name)
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		owner = "product-engineering"
	}
	feature := FeatureSpec{
		APIVersion: "mss.io/v1alpha1",
		Kind:       "Feature",
		Metadata: FeatureMetadata{
			Name:        name,
			DisplayName: displayName,
			Description: "Describe the user-visible and engineering value of this feature.",
			Owner:       owner,
		},
		Spec: FeatureBody{
			Problem:  "Describe the concrete user or engineering problem, current impact, and why it should be solved now.",
			Goals:    []string{"Deliver the required behavior with explicit authorization and verifiable evidence."},
			NonGoals: []string{"Do not expand unrelated platform capabilities as part of this feature."},
			Actors: []FeatureActor{
				{ID: "administrator", DisplayName: "Administrator", Description: "Administers this feature according to backend permissions."},
			},
			Modules: []FeatureModule{
				{Name: name, Operation: "create", SpecPath: ".mss/modules/" + name + ".yaml", Description: "Primary vertical management module for the feature."},
			},
			Requirements: []FeatureRequirement{
				{
					ID:          name + "-manage",
					Title:       "Manage " + displayName,
					Description: "An authorized administrator can create, read, update, and delete valid records for this feature.",
					Priority:    "must",
					Actor:       "administrator",
					Module:      name,
					Permission:  name + ":manage",
					Rules:       []string{"Every mutation is authorized on the backend and invalid input is rejected."},
				},
			},
			Constraints: []FeatureConstraint{
				{ID: "backend-authorization", Type: "security", Statement: "Frontend visibility is not authorization; every protected operation is enforced on the backend."},
				{ID: "migration-compatibility", Type: "compatibility", Statement: "Database changes are additive or include an explicit data migration and rollback strategy."},
			},
			Acceptance: []AcceptanceCriterion{
				{
					ID:          name + "-contract",
					Requirement: name + "-manage",
					Statement:   "The feature contract, generated module, authorization behavior, and changed-file verification pass.",
					Level:       "integration",
					Required:    true,
					Evidence: []AcceptanceEvidence{
						{Type: "command", Value: "go run ./cmd/mss verify --changed"},
					},
				},
			},
			Risks: []FeatureRisk{
				{ID: "delivery-drift", Description: "Implementation, permissions, tests, and documentation may drift from the approved feature contract.", Severity: "high", Mitigation: "Keep the Feature and AdminModule specifications in the same change and enforce contract and verifier checks in CI."},
			},
			Validation: FeatureValidation{
				Changed: "go run ./cmd/mss verify --changed",
				All:     "go run ./cmd/mss verify --all",
			},
			Rollout: FeatureRollout{
				Strategy:  "phased",
				Migration: "Apply and validate additive migrations in a non-production environment before enabling the feature.",
				Rollback:  "Disable exposure, revert the feature commit, and execute only the documented safe migration rollback.",
			},
		},
	}
	feature.Normalize()
	if err := feature.Validate(); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(feature)
	if err != nil {
		return nil, err
	}
	return data, nil
}
