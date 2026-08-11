package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/provider"
)

func TestRootRegistersProviderEvidenceHelp(t *testing.T) {
	command := NewAgentRootCommand()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"provider", "evidence", "--help"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute provider evidence help: %v", err)
	}
	for _, value := range []string{"--input", "--required", "--format"} {
		if !strings.Contains(stdout.String(), value) {
			t.Fatalf("help does not contain %s:\n%s", value, stdout.String())
		}
	}
}

func TestProviderEvidenceCommandPassesRequiredAndWritesJSON(t *testing.T) {
	rootOverride := repositoryRoot(t)
	var receivedRoot string
	var received provider.Options
	runner := func(_ context.Context, root string, options provider.Options) (provider.Report, error) {
		receivedRoot = root
		received = options
		return provider.Report{
			APIVersion:             provider.APIVersion,
			Kind:                   provider.ValidationKind,
			Source:                 options.Input,
			Version:                "v1.1.0",
			Commit:                 "0123456789abcdef0123456789abcdef01234567",
			RequiredGate:           options.Required,
			Success:                true,
			RequiredCount:          1,
			QualifiedRequiredCount: 1,
		}, nil
	}
	command := newProviderCommandWithRunner(&rootOverride, runner)
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"evidence", "--input", "fixtures/provider.json", "--required", "--format", "json"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute provider evidence: %v", err)
	}
	if receivedRoot != rootOverride || received.Input != "fixtures/provider.json" || !received.Required {
		t.Fatalf("unexpected runner options: root=%q options=%#v", receivedRoot, received)
	}
	var report provider.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode provider report: %v\n%s", err, stdout.String())
	}
	if !report.Success || !report.RequiredGate || report.QualifiedRequiredCount != 1 {
		t.Fatalf("unexpected provider report: %#v", report)
	}
}

func TestProviderEvidenceCommandWritesNegativeReportBeforeReturningError(t *testing.T) {
	rootOverride := repositoryRoot(t)
	runner := func(_ context.Context, _ string, options provider.Options) (provider.Report, error) {
		return provider.Report{
			APIVersion:   provider.APIVersion,
			Kind:         provider.ValidationKind,
			Source:       options.Input,
			Version:      "v1.1.0",
			Commit:       "0123456789abcdef0123456789abcdef01234567",
			RequiredGate: true,
			Success:      false,
			Failures:     []string{"redis/named-resource fixture standalone@7.4.1: run count is zero"},
		}, errors.New("required provider evidence failed")
	}
	command := newProviderCommandWithRunner(&rootOverride, runner)
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SilenceUsage = true
	command.SilenceErrors = true
	command.SetArgs([]string{"evidence", "--required", "--format", "json"})
	if err := command.ExecuteContext(context.Background()); err == nil {
		t.Fatal("negative required evidence must return a non-zero CLI result")
	}
	var report provider.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("negative report was not written before the error: %v\n%s", err, stdout.String())
	}
	if report.Success || len(report.Failures) != 1 {
		t.Fatalf("unexpected negative provider report: %#v", report)
	}
}
