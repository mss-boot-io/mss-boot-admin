// Package provider validates provider-specific maturity evidence without
// constructing or contacting the provider itself.
package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	APIVersion     = "mss.io/v1alpha1"
	Kind           = "ProviderMaturityReport"
	ValidationKind = "ProviderEvidenceValidation"
	DefaultInput   = ".mss/reports/provider-maturity.json"
)

var (
	commitPattern   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	identityPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)
	versionPattern  = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

// Options selects the checked-in or generated evidence document and whether
// release-required evidence is enforced.
type Options struct {
	Input    string
	Required bool
}

// Document is the strict provider evidence input described by the checked-in
// JSON schema.
type Document struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   Metadata   `json:"metadata"`
	Providers  []Evidence `json:"providers"`
}

type Metadata struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// Evidence binds one provider to one capability and one pinned fixture. This
// prevents evidence for a provider name from promoting unrelated capabilities.
type Evidence struct {
	Provider   string  `json:"provider"`
	Capability string  `json:"capability"`
	Maturity   string  `json:"maturity"`
	Required   bool    `json:"required"`
	Fixture    Fixture `json:"fixture"`
	Result     Result  `json:"result"`
}

type Fixture struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Result contains exact, uncached execution counts supplied by the evidence
// producer. This command validates evidence; it never starts a provider.
type Result struct {
	Run        int  `json:"run"`
	Pass       int  `json:"pass"`
	Skip       int  `json:"skip"`
	Fail       int  `json:"fail"`
	CachedOnly bool `json:"cachedOnly"`
}

// ProviderResult is the normalized, deterministic report row.
type ProviderResult struct {
	Provider   string   `json:"provider"`
	Capability string   `json:"capability"`
	Maturity   string   `json:"maturity"`
	Required   bool     `json:"required"`
	Fixture    Fixture  `json:"fixture"`
	Result     Result   `json:"result"`
	Qualified  bool     `json:"qualified"`
	Failures   []string `json:"failures,omitempty"`
}

// Report is the normalized validation envelope emitted by the command. The
// source ProviderMaturityReport remains the schema-governed evidence artifact;
// this distinct kind avoids presenting derived gate fields as source evidence.
// Timestamps are omitted so identical evidence produces identical output.
type Report struct {
	APIVersion             string           `json:"apiVersion"`
	Kind                   string           `json:"kind"`
	Source                 string           `json:"source"`
	Version                string           `json:"version"`
	Commit                 string           `json:"commit"`
	RequiredGate           bool             `json:"requiredGate"`
	Success                bool             `json:"success"`
	RequiredCount          int              `json:"requiredCount"`
	QualifiedRequiredCount int              `json:"qualifiedRequiredCount"`
	Providers              []ProviderResult `json:"providers"`
	Failures               []string         `json:"failures,omitempty"`
}

// Run loads, validates, normalizes, and evaluates an evidence document.
func Run(root string, options Options) (Report, error) {
	document, source, err := Load(root, options.Input)
	if err != nil {
		if options.Input == "" {
			options.Input = DefaultInput
		}
		return Report{
			APIVersion:   APIVersion,
			Kind:         ValidationKind,
			Source:       filepath.ToSlash(options.Input),
			RequiredGate: options.Required,
			Success:      false,
			Providers:    []ProviderResult{},
			Failures:     []string{err.Error()},
		}, err
	}
	report := Evaluate(document, source, options.Required)
	if options.Required && !report.Success {
		return report, errors.New("required provider evidence failed")
	}
	return report, nil
}

// Load reads one repository-confined JSON document with strict unknown-field
// rejection and semantic validation.
func Load(root, input string) (Document, string, error) {
	path, source, err := resolveInput(root, input)
	if err != nil {
		return Document{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, "", fmt.Errorf("read provider evidence %s: %w", source, err)
	}

	document := Document{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Document{}, "", fmt.Errorf("decode provider evidence %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Document{}, "", fmt.Errorf("decode provider evidence %s: multiple JSON values are not supported", source)
		}
		return Document{}, "", fmt.Errorf("decode provider evidence %s: %w", source, err)
	}
	if err := validateRequiredFields(data); err != nil {
		return Document{}, "", fmt.Errorf("validate provider evidence %s: %w", source, err)
	}
	if err := document.Validate(); err != nil {
		return Document{}, "", fmt.Errorf("validate provider evidence %s: %w", source, err)
	}
	return document, source, nil
}

// Validate enforces semantics that JSON Schema cannot express, including
// provider/capability/fixture identity uniqueness and result count consistency.
func (document Document) Validate() error {
	var problems []string
	if document.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if document.Kind != Kind {
		problems = append(problems, fmt.Sprintf("kind must be %q", Kind))
	}
	if !versionPattern.MatchString(document.Metadata.Version) {
		problems = append(problems, "metadata.version must be a v-prefixed semantic version")
	}
	if !commitPattern.MatchString(document.Metadata.Commit) {
		problems = append(problems, "metadata.commit must be a full lowercase 40-character Git SHA")
	}

	seen := make(map[string]int, len(document.Providers))
	for index, evidence := range document.Providers {
		prefix := fmt.Sprintf("providers[%d]", index)
		validateIdentity(&problems, prefix+".provider", evidence.Provider)
		validateIdentity(&problems, prefix+".capability", evidence.Capability)
		validateIdentity(&problems, prefix+".fixture.name", evidence.Fixture.Name)
		if strings.TrimSpace(evidence.Fixture.Version) == "" || strings.ContainsAny(evidence.Fixture.Version, " \t\r\n") {
			problems = append(problems, prefix+".fixture.version must be a pinned non-empty value without whitespace")
		}
		switch evidence.Maturity {
		case "blocked", "experimental", "beta", "stable":
		default:
			problems = append(problems, prefix+".maturity must be blocked, experimental, beta, or stable")
		}
		if evidence.Result.Run < 0 || evidence.Result.Pass < 0 || evidence.Result.Skip < 0 || evidence.Result.Fail < 0 {
			problems = append(problems, prefix+".result counts must be non-negative")
		}
		if evidence.Result.Pass > evidence.Result.Run {
			problems = append(problems, prefix+".result.pass cannot exceed result.run")
		}
		if evidence.Result.Pass+evidence.Result.Skip+evidence.Result.Fail != evidence.Result.Run {
			problems = append(problems, prefix+".result pass, skip, and fail counts must sum to result.run")
		}
		identity := strings.Join([]string{evidence.Provider, evidence.Capability, evidence.Fixture.Name, evidence.Fixture.Version}, "\x00")
		if previous, ok := seen[identity]; ok {
			problems = append(problems, fmt.Sprintf("%s duplicates providers[%d] identity", prefix, previous))
		} else {
			seen[identity] = index
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func validateIdentity(problems *[]string, path, value string) {
	if !identityPattern.MatchString(value) {
		*problems = append(*problems, path+" must be a lowercase provider identity")
	}
}

type requiredDocument struct {
	APIVersion *string             `json:"apiVersion"`
	Kind       *string             `json:"kind"`
	Metadata   *requiredMetadata   `json:"metadata"`
	Providers  *[]requiredEvidence `json:"providers"`
}

type requiredMetadata struct {
	Version *string `json:"version"`
	Commit  *string `json:"commit"`
}

type requiredEvidence struct {
	Provider   *string          `json:"provider"`
	Capability *string          `json:"capability"`
	Maturity   *string          `json:"maturity"`
	Required   *bool            `json:"required"`
	Fixture    *requiredFixture `json:"fixture"`
	Result     *requiredResult  `json:"result"`
}

type requiredFixture struct {
	Name    *string `json:"name"`
	Version *string `json:"version"`
}

type requiredResult struct {
	Run        *int  `json:"run"`
	Pass       *int  `json:"pass"`
	Skip       *int  `json:"skip"`
	Fail       *int  `json:"fail"`
	CachedOnly *bool `json:"cachedOnly"`
}

func validateRequiredFields(data []byte) error {
	var required requiredDocument
	if err := json.Unmarshal(data, &required); err != nil {
		return err
	}
	var missing []string
	requireField(&missing, "apiVersion", required.APIVersion != nil)
	requireField(&missing, "kind", required.Kind != nil)
	requireField(&missing, "metadata", required.Metadata != nil)
	if required.Metadata != nil {
		requireField(&missing, "metadata.version", required.Metadata.Version != nil)
		requireField(&missing, "metadata.commit", required.Metadata.Commit != nil)
	}
	requireField(&missing, "providers", required.Providers != nil)
	if required.Providers != nil {
		for index, evidence := range *required.Providers {
			prefix := fmt.Sprintf("providers[%d]", index)
			requireField(&missing, prefix+".provider", evidence.Provider != nil)
			requireField(&missing, prefix+".capability", evidence.Capability != nil)
			requireField(&missing, prefix+".maturity", evidence.Maturity != nil)
			requireField(&missing, prefix+".required", evidence.Required != nil)
			requireField(&missing, prefix+".fixture", evidence.Fixture != nil)
			if evidence.Fixture != nil {
				requireField(&missing, prefix+".fixture.name", evidence.Fixture.Name != nil)
				requireField(&missing, prefix+".fixture.version", evidence.Fixture.Version != nil)
			}
			requireField(&missing, prefix+".result", evidence.Result != nil)
			if evidence.Result != nil {
				requireField(&missing, prefix+".result.run", evidence.Result.Run != nil)
				requireField(&missing, prefix+".result.pass", evidence.Result.Pass != nil)
				requireField(&missing, prefix+".result.skip", evidence.Result.Skip != nil)
				requireField(&missing, prefix+".result.fail", evidence.Result.Fail != nil)
				requireField(&missing, prefix+".result.cachedOnly", evidence.Result.CachedOnly != nil)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

func requireField(missing *[]string, path string, present bool) {
	if !present {
		*missing = append(*missing, path)
	}
}

// Evaluate creates a stable report and optionally applies the release-required
// gate. Optional rows are visible but never block the required gate.
func Evaluate(document Document, source string, requiredGate bool) Report {
	providers := append([]Evidence(nil), document.Providers...)
	sort.Slice(providers, func(i, j int) bool {
		left := providers[i]
		right := providers[j]
		if left.Provider != right.Provider {
			return left.Provider < right.Provider
		}
		if left.Capability != right.Capability {
			return left.Capability < right.Capability
		}
		if left.Fixture.Name != right.Fixture.Name {
			return left.Fixture.Name < right.Fixture.Name
		}
		return left.Fixture.Version < right.Fixture.Version
	})

	report := Report{
		APIVersion:   APIVersion,
		Kind:         ValidationKind,
		Source:       source,
		Version:      document.Metadata.Version,
		Commit:       document.Metadata.Commit,
		RequiredGate: requiredGate,
		Success:      true,
		Providers:    make([]ProviderResult, 0, len(providers)),
	}
	for _, evidence := range providers {
		failures := qualificationFailures(evidence.Result)
		row := ProviderResult{
			Provider:   evidence.Provider,
			Capability: evidence.Capability,
			Maturity:   evidence.Maturity,
			Required:   evidence.Required,
			Fixture:    evidence.Fixture,
			Result:     evidence.Result,
			Qualified:  len(failures) == 0,
			Failures:   failures,
		}
		report.Providers = append(report.Providers, row)
		if !evidence.Required {
			continue
		}
		report.RequiredCount++
		if row.Qualified {
			report.QualifiedRequiredCount++
			continue
		}
		if requiredGate {
			for _, failure := range failures {
				report.Failures = append(report.Failures, fmt.Sprintf("%s/%s fixture %s@%s: %s", evidence.Provider, evidence.Capability, evidence.Fixture.Name, evidence.Fixture.Version, failure))
			}
		}
	}
	if requiredGate && report.RequiredCount == 0 {
		report.Failures = append(report.Failures, "required provider selection is empty")
	}
	if requiredGate && len(report.Failures) > 0 {
		report.Success = false
	}
	return report
}

func qualificationFailures(result Result) []string {
	var failures []string
	if result.Run == 0 {
		failures = append(failures, "run count is zero")
	}
	if result.Skip > 0 {
		failures = append(failures, fmt.Sprintf("skip count is %d", result.Skip))
	}
	if result.Fail > 0 {
		failures = append(failures, fmt.Sprintf("fail count is %d", result.Fail))
	}
	if result.Pass != result.Run {
		failures = append(failures, fmt.Sprintf("pass count is %d, want run count %d", result.Pass, result.Run))
	}
	if result.CachedOnly {
		failures = append(failures, "evidence is cached-only")
	}
	return failures
}

func resolveInput(root, input string) (string, string, error) {
	if strings.TrimSpace(root) == "" {
		return "", "", errors.New("repository root is required")
	}
	if input == "" {
		input = DefaultInput
	}
	if filepath.IsAbs(input) || strings.Contains(input, `\`) {
		return "", "", errors.New("provider evidence input must be a slash-separated repository-relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(input))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("provider evidence input must identify a confined repository file")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	candidate := filepath.Join(resolvedRoot, clean)
	if _, err := os.Lstat(candidate); err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("provider evidence %s does not exist", filepath.ToSlash(clean))
		}
		return "", "", fmt.Errorf("provider evidence %s is unavailable", filepath.ToSlash(clean))
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", "", fmt.Errorf("provider evidence %s cannot be resolved", filepath.ToSlash(clean))
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", "", errors.New("provider evidence input resolves outside the repository")
	}
	return resolvedCandidate, filepath.ToSlash(clean), nil
}

func (report Report) JSON() ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func (report Report) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "provider evidence: success=%t required=%d qualified=%d\n", report.Success, report.RequiredCount, report.QualifiedRequiredCount)
	fmt.Fprintf(&builder, "source: %s\nversion: %s\ncommit: %s\n", report.Source, report.Version, report.Commit)
	for _, row := range report.Providers {
		fmt.Fprintf(&builder, "- %s/%s %s@%s maturity=%s required=%t qualified=%t\n", row.Provider, row.Capability, row.Fixture.Name, row.Fixture.Version, row.Maturity, row.Required, row.Qualified)
	}
	for _, failure := range report.Failures {
		fmt.Fprintf(&builder, "FAIL %s\n", failure)
	}
	return builder.String()
}
