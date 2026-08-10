package blueprint

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// JSON returns stable indented JSON for agents and CI.
func (p Plan) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// Text returns a compact human-readable application generation plan.
func (p Plan) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "mss new app: %s\n", p.Application.Name)
	fmt.Fprintf(&builder, "display name: %s\n", p.Application.DisplayName)
	fmt.Fprintf(&builder, "module: %s\n", p.Application.Module)
	fmt.Fprintf(&builder, "repository: %s\n", p.Application.Repository)
	fmt.Fprintf(&builder, "blueprint: %s@%s\n", p.Blueprint, p.BlueprintVersion)
	fmt.Fprintf(&builder, "blueprint digest: %s\n", p.Identities.Blueprint.SHA256)
	fmt.Fprintf(&builder, "foundation: %s@%s commit %s\n", p.Identities.Foundation.Repository, p.Identities.Foundation.Version, p.FoundationCommit)
	fmt.Fprintf(&builder, "generator: %s@%s", p.Identities.Generator.Tool, p.Identities.Generator.Version)
	if p.Identities.Generator.Commit != "" {
		fmt.Fprintf(&builder, " commit %s", p.Identities.Generator.Commit)
	}
	builder.WriteByte('\n')
	fmt.Fprintf(&builder, "snapshot digest: %s\n", p.Identities.Snapshot.SHA256)
	fmt.Fprintf(&builder, "destination: %s\n", p.Destination)
	fmt.Fprintf(&builder, "dry run: %t\n", p.DryRun)
	fmt.Fprintf(&builder, "success: %t\n", p.Success)
	fmt.Fprintf(&builder, "files: %d\n", p.TotalFiles)
	fmt.Fprintf(&builder, "bytes: %d\n\n", p.TotalBytes)

	counts := make(map[Action]int)
	for _, change := range p.Changes {
		counts[change.Action]++
	}
	actions := make([]string, 0, len(counts))
	for action := range counts {
		actions = append(actions, string(action))
	}
	sort.Strings(actions)
	for _, action := range actions {
		fmt.Fprintf(&builder, "%s: %d\n", action, counts[Action(action)])
	}
	for _, change := range p.Changes {
		if change.Action == ActionUnchanged {
			continue
		}
		fmt.Fprintf(&builder, "- [%s] %s", change.Action, change.Path)
		if change.Detail != "" {
			fmt.Fprintf(&builder, ": %s", change.Detail)
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}
