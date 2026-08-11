package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func TestLoadProviderEvidenceAcceptsStrictPinnedReport(t *testing.T) {
	root := t.TempDir()
	document := validDocument()
	writeEvidence(t, root, "evidence/provider.json", document)

	loaded, source, err := Load(root, "evidence/provider.json")
	if err != nil {
		t.Fatalf("load provider evidence: %v", err)
	}
	if source != "evidence/provider.json" {
		t.Fatalf("source = %q, want repository-relative input", source)
	}
	if !reflect.DeepEqual(loaded, document) {
		t.Fatalf("loaded document differs:\n got: %#v\nwant: %#v", loaded, document)
	}
}

func TestLoadProviderEvidenceRejectsInvalidIdentityAndUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		failure string
	}{
		{
			name: "invalid identity",
			content: `{
  "apiVersion":"mss.io/v1alpha1",
  "kind":"ProviderMaturityReport",
  "metadata":{"version":"v1.1.0","commit":"0123456789abcdef0123456789abcdef01234567"},
  "providers":[{"provider":"Redis Scope","capability":"named-resource","maturity":"beta","required":true,"fixture":{"name":"standalone","version":"7.4.1"},"result":{"run":1,"pass":1,"skip":0,"fail":0,"cachedOnly":false}}]
}`,
			failure: "provider must be a lowercase provider identity",
		},
		{
			name: "unknown field",
			content: `{
  "apiVersion":"mss.io/v1alpha1",
  "kind":"ProviderMaturityReport",
  "metadata":{"version":"v1.1.0","commit":"0123456789abcdef0123456789abcdef01234567","branch":"main"},
  "providers":[]
}`,
			failure: "unknown field",
		},
		{
			name: "missing required result field",
			content: `{
  "apiVersion":"mss.io/v1alpha1",
  "kind":"ProviderMaturityReport",
  "metadata":{"version":"v1.1.0","commit":"0123456789abcdef0123456789abcdef01234567"},
  "providers":[{"provider":"redis","capability":"named-resource","maturity":"beta","required":true,"fixture":{"name":"standalone","version":"7.4.1"},"result":{"run":1,"pass":1,"skip":0,"fail":0}}]
}`,
			failure: "providers[0].result.cachedOnly",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "provider.json")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatalf("write evidence: %v", err)
			}
			if _, _, err := Load(root, "provider.json"); err == nil || !strings.Contains(err.Error(), test.failure) {
				t.Fatalf("Load error = %v, want failure containing %q", err, test.failure)
			}
		})
	}
}

func TestEvaluateRequiredProviderEvidenceAcceptsFreshQualifiedFixtures(t *testing.T) {
	report := Evaluate(validDocument(), ".mss/reports/provider-maturity.json", true)
	if !report.Success || report.RequiredCount != 1 || report.QualifiedRequiredCount != 1 {
		t.Fatalf("unexpected required report: %#v", report)
	}
	if len(report.Providers) != 1 || !report.Providers[0].Qualified || len(report.Failures) != 0 {
		t.Fatalf("qualified evidence was rejected: %#v", report)
	}
}

func TestEvaluateRequiredProviderEvidenceRejectsZeroHitSkipAndCachedOnly(t *testing.T) {
	tests := []struct {
		name    string
		result  Result
		failure string
	}{
		{
			name:    "zero hit",
			result:  Result{},
			failure: "run count is zero",
		},
		{
			name:    "skip",
			result:  Result{Run: 1, Skip: 1},
			failure: "skip count is 1",
		},
		{
			name:    "cached only",
			result:  Result{Run: 1, Pass: 1, CachedOnly: true},
			failure: "evidence is cached-only",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := validDocument()
			document.Providers[0].Result = test.result
			report := Evaluate(document, ".mss/reports/provider-maturity.json", true)
			if report.Success {
				t.Fatalf("negative evidence reported success: %#v", report)
			}
			if !containsFailure(report.Failures, test.failure) {
				t.Fatalf("failures = %#v, want %q", report.Failures, test.failure)
			}
			root := t.TempDir()
			writeEvidence(t, root, "provider.json", document)
			runReport, err := Run(root, Options{Input: "provider.json", Required: true})
			if err == nil || runReport.Success {
				t.Fatalf("Run must return negative required evidence: report=%#v err=%v", runReport, err)
			}
		})
	}
}

func TestEvaluateOptionalProviderEvidenceDoesNotBlockRequiredGate(t *testing.T) {
	document := validDocument()
	document.Providers = append(document.Providers, Evidence{
		Provider:   "rustfs",
		Capability: "object-store",
		Maturity:   "blocked",
		Required:   false,
		Fixture:    Fixture{Name: "optional-s3", Version: "1.0.0"},
		Result:     Result{},
	})
	report := Evaluate(document, ".mss/reports/provider-maturity.json", true)
	if !report.Success || report.RequiredCount != 1 || report.QualifiedRequiredCount != 1 {
		t.Fatalf("optional provider blocked required gate: %#v", report)
	}
	if len(report.Providers) != 2 || report.Providers[1].Qualified {
		t.Fatalf("optional negative evidence must remain visible and unqualified: %#v", report.Providers)
	}
}

func TestProviderEvidenceReportJSONIsDeterministic(t *testing.T) {
	document := validDocument()
	document.Providers = append([]Evidence{
		{
			Provider:   "memory",
			Capability: "event-bus",
			Maturity:   "beta",
			Required:   true,
			Fixture:    Fixture{Name: "in-process", Version: "go1.26.5"},
			Result:     Result{Run: 2, Pass: 2},
		},
	}, document.Providers...)

	first := Evaluate(document, "evidence.json", true)
	second := Evaluate(document, "evidence.json", true)
	firstJSON, err := first.JSON()
	if err != nil {
		t.Fatalf("marshal first report: %v", err)
	}
	secondJSON, err := second.JSON()
	if err != nil {
		t.Fatalf("marshal second report: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("report JSON is not deterministic:\n%s\n---\n%s", firstJSON, secondJSON)
	}
	if got := first.Providers[0].Provider; got != "memory" {
		t.Fatalf("first provider = %q, want stable provider sort", got)
	}
}

func validDocument() Document {
	return Document{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Version: "v1.1.0",
			Commit:  testCommit,
		},
		Providers: []Evidence{
			{
				Provider:   "redis",
				Capability: "named-resource",
				Maturity:   "beta",
				Required:   true,
				Fixture:    Fixture{Name: "standalone", Version: "7.4.1"},
				Result:     Result{Run: 1, Pass: 1},
			},
		},
	}
}

func writeEvidence(t *testing.T, root, relative string, document Document) {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create evidence directory: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
}

func containsFailure(failures []string, value string) bool {
	for _, failure := range failures {
		if strings.Contains(failure, value) {
			return true
		}
	}
	return false
}
