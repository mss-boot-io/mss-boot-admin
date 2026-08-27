package app

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	evalcmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/eval"
)

func TestEvalRunContributorRegistryFlagIsHiddenAndForwarded(t *testing.T) {
	rootOverride := repositoryRoot(t)
	for _, test := range []struct {
		name     string
		args     []string
		caseIDs  []string
		registry string
	}{
		{
			name:    "normal path keeps public resolver default",
			args:    []string{"mcp-project-tools", "--format", "json"},
			caseIDs: []string{"mcp-project-tools"},
		},
		{
			name:     "candidate path forwards explicit loopback registry",
			args:     []string{"--all", "--format", "json", "--contributor-npm-registry", "http://127.0.0.1:4873"},
			registry: "http://127.0.0.1:4873",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var capturedRoot string
			var captured evalcmd.RunOptions
			runner := func(_ context.Context, root string, options evalcmd.RunOptions) (evalcmd.Report, error) {
				capturedRoot = root
				captured = options
				return evalcmd.Report{Project: "fixture", Success: true}, nil
			}
			command := newEvalRunCommandWithRunner(&rootOverride, runner)
			flag := command.Flags().Lookup("contributor-npm-registry")
			if flag == nil || !flag.Hidden || flag.DefValue != "" {
				t.Fatalf("contributor registry flag = %#v", flag)
			}
			command.SetArgs(test.args)
			command.SetOut(&bytes.Buffer{})
			command.SetErr(&bytes.Buffer{})
			if err := command.Execute(); err != nil {
				t.Fatalf("execute eval run: %v", err)
			}
			if capturedRoot != rootOverride {
				t.Fatalf("runner root = %q, want %q", capturedRoot, rootOverride)
			}
			if !reflect.DeepEqual(captured.CaseIDs, test.caseIDs) {
				t.Fatalf("runner cases = %#v, want %#v", captured.CaseIDs, test.caseIDs)
			}
			if captured.ContributorFrontendRegistryURL != test.registry {
				t.Fatalf("runner registry = %q, want %q", captured.ContributorFrontendRegistryURL, test.registry)
			}
		})
	}
}
