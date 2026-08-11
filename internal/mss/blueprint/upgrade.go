package blueprint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ActionUpdate   Action = "update"
	ActionDelete   Action = "delete"
	ActionPreserve Action = "preserve"
)

// UpgradeOptions controls three-way comparison against a newer foundation checkout.
type UpgradeOptions struct {
	ApplicationRoot string
	FoundationRoot  string
	ManifestPath    string
	Blueprint       string
	Application     Application
	Write           bool
}

// UpgradeChange records base, current, and desired state for one managed file.
type UpgradeChange struct {
	Path          string      `json:"path"`
	Action        Action      `json:"action"`
	Mode          fs.FileMode `json:"mode,omitempty"`
	BaseSHA256    string      `json:"baseSha256,omitempty"`
	CurrentSHA256 string      `json:"currentSha256,omitempty"`
	DesiredSHA256 string      `json:"desiredSha256,omitempty"`
	Detail        string      `json:"detail,omitempty"`
}

// UpgradePlan is safe to review before any downstream files change.
type UpgradePlan struct {
	Application          Application     `json:"application"`
	ApplicationRoot      string          `json:"applicationRoot"`
	FoundationRoot       string          `json:"foundationRoot"`
	Blueprint            string          `json:"blueprint"`
	FromBlueprintVersion string          `json:"fromBlueprintVersion"`
	ToBlueprintVersion   string          `json:"toBlueprintVersion"`
	FromFoundationCommit string          `json:"fromFoundationCommit"`
	ToFoundationCommit   string          `json:"toFoundationCommit"`
	FromIdentities       *IdentitySet    `json:"fromIdentities,omitempty"`
	ToIdentities         IdentitySet     `json:"toIdentities"`
	LegacyInput          bool            `json:"legacyInput,omitempty"`
	DryRun               bool            `json:"dryRun"`
	Success              bool            `json:"success"`
	Changes              []UpgradeChange `json:"changes"`
}

// Upgrade plans or applies a three-way foundation upgrade. User-created files
// outside the managed manifest are preserved and ignored.
func Upgrade(ctx context.Context, options UpgradeOptions) (UpgradePlan, error) {
	applicationRoot, err := filepath.Abs(options.ApplicationRoot)
	if err != nil {
		return UpgradePlan{}, fmt.Errorf("resolve application root: %w", err)
	}
	foundationRoot, err := filepath.Abs(options.FoundationRoot)
	if err != nil {
		return UpgradePlan{}, fmt.Errorf("resolve foundation root: %w", err)
	}
	if options.ManifestPath == "" {
		options.ManifestPath = ".mss/blueprint-manifest.json"
	}
	oldManifest, legacyManifest, err := readManifestForUpgrade(applicationRoot, options.ManifestPath)
	if err != nil {
		return UpgradePlan{}, fmt.Errorf("read downstream foundation baseline: %w", err)
	}
	if options.Blueprint == "" {
		options.Blueprint = oldManifest.Metadata.Blueprint
	}
	if options.Blueprint != oldManifest.Metadata.Blueprint {
		return UpgradePlan{}, fmt.Errorf("blueprint switch from %s to %s requires an explicit migration recipe", oldManifest.Metadata.Blueprint, options.Blueprint)
	}
	if options.Application.Name == "" {
		options.Application.Name = oldManifest.Metadata.Project
	}
	if options.Application.Module == "" {
		options.Application.Module = oldManifest.Metadata.Module
	}
	if options.Application.Repository == "" {
		options.Application.Repository = oldManifest.Metadata.Repository
	}
	options.Application = normalizeApplication(options.Application)
	if err := ValidateApplication(options.Application); err != nil {
		return UpgradePlan{}, err
	}
	if options.Application.Name != oldManifest.Metadata.Project || options.Application.Module != oldManifest.Metadata.Module {
		return UpgradePlan{}, errors.New("application identity does not match the existing blueprint manifest")
	}

	newBlueprint, err := Load(foundationRoot, options.Blueprint)
	if err != nil {
		return UpgradePlan{}, err
	}
	desired, newManifest, err := BuildDesired(ctx, foundationRoot, newBlueprint, options.Application)
	if err != nil {
		return UpgradePlan{}, err
	}
	if normalizedPath(options.ManifestPath) != normalizedPath(newBlueprint.Spec.ManifestPath) {
		return UpgradePlan{}, fmt.Errorf("manifest path switch from %s to %s requires an explicit migration recipe", options.ManifestPath, newBlueprint.Spec.ManifestPath)
	}
	if !legacyManifest && (normalizedPath(oldManifest.Records.LockPath) != normalizedPath(newBlueprint.Spec.LockPath) ||
		normalizedPath(oldManifest.Records.ManifestPath) != normalizedPath(newBlueprint.Spec.ManifestPath)) {
		return UpgradePlan{}, errors.New("snapshot record path changes require an explicit migration recipe")
	}

	plan, err := buildUpgradePlan(applicationRoot, foundationRoot, options.ManifestPath, newBlueprint.Spec.LockPath, oldManifest, newBlueprint, newManifest, desired, !options.Write, options.Application)
	if err != nil {
		return plan, err
	}
	if !options.Write {
		return plan, nil
	}
	if !plan.Success {
		return plan, errors.New("foundation upgrade contains conflicts; no files were changed")
	}
	records := SnapshotRecordPaths{LockPath: normalizedPath(newBlueprint.Spec.LockPath), ManifestPath: normalizedPath(newBlueprint.Spec.ManifestPath)}
	if err := applyUpgrade(ctx, applicationRoot, plan, desired, records); err != nil {
		return plan, err
	}
	plan.DryRun = false
	return plan, nil
}

func buildUpgradePlan(
	applicationRoot string,
	foundationRoot string,
	manifestPath string,
	lockPath string,
	oldManifest Manifest,
	newBlueprint *Document,
	newManifest Manifest,
	desired map[string]desiredFile,
	dryRun bool,
	application Application,
) (UpgradePlan, error) {
	plan := UpgradePlan{
		Application:          application,
		ApplicationRoot:      applicationRoot,
		FoundationRoot:       foundationRoot,
		Blueprint:            newBlueprint.Metadata.Name,
		FromBlueprintVersion: oldManifest.Metadata.BlueprintVersion,
		ToBlueprintVersion:   newBlueprint.Metadata.Version,
		FromFoundationCommit: oldManifest.Metadata.FoundationCommit,
		ToFoundationCommit:   newManifest.Metadata.FoundationCommit,
		ToIdentities:         newManifest.Identities,
		LegacyInput:          oldManifest.APIVersion == legacyAPIVersion,
		DryRun:               dryRun,
		Success:              true,
	}
	if oldManifest.APIVersion == snapshotAPIVersion {
		from := oldManifest.Identities
		plan.FromIdentities = &from
	}

	managed := make(map[string]bool, len(oldManifest.Files)+len(desired))
	for relative := range oldManifest.Files {
		managed[relative] = true
	}
	for relative := range desired {
		managed[relative] = true
	}
	managed[manifestPath] = true
	managed[lockPath] = true
	paths := make([]string, 0, len(managed))
	for relative := range managed {
		if !safeRelativePath(relative) {
			return plan, fmt.Errorf("managed upgrade path is unsafe: %s", relative)
		}
		paths = append(paths, relative)
	}
	sort.Strings(paths)

	for _, relative := range paths {
		base, hadBase := oldManifest.Files[relative]
		wanted, hasDesired := desired[relative]
		current, currentExists, err := readManagedFile(applicationRoot, relative)
		if err != nil {
			return plan, err
		}
		currentHash := ""
		if currentExists {
			currentHash = digest(current)
		}
		desiredHash := ""
		if hasDesired {
			desiredHash = digest(wanted.Data)
		}
		change := UpgradeChange{
			Path:          relative,
			Mode:          wanted.Mode,
			CurrentSHA256: currentHash,
			DesiredSHA256: desiredHash,
		}
		if hadBase {
			change.BaseSHA256 = base.SHA256
		}

		switch {
		case relative == manifestPath || relative == lockPath:
			if currentExists && currentHash == desiredHash {
				change.Action = ActionUnchanged
			} else {
				change.Action = ActionUpdate
				change.Detail = "commit the verified snapshot record after a conflict-free upgrade"
			}

		case hadBase && hasDesired:
			switch {
			case !currentExists:
				change.Action = ActionConflict
				change.Detail = "managed file was deleted locally while the foundation still requires it"
			case currentHash == base.SHA256 && desiredHash == base.SHA256:
				change.Action = ActionUnchanged
			case currentHash == base.SHA256:
				change.Action = ActionUpdate
				change.Detail = "foundation changed and downstream file is unmodified"
			case currentHash == desiredHash:
				change.Action = ActionUnchanged
				change.Detail = "downstream already matches the new foundation"
			case desiredHash == base.SHA256:
				change.Action = ActionPreserve
				change.Detail = "downstream customization preserved; foundation did not change this file"
			default:
				change.Action = ActionConflict
				change.Detail = "both downstream and foundation changed this managed file"
			}

		case hadBase && !hasDesired:
			switch {
			case !currentExists:
				change.Action = ActionUnchanged
				change.Detail = "file was already removed locally"
			case currentHash == base.SHA256:
				change.Action = ActionDelete
				change.Detail = "foundation removed an unmodified managed file"
			default:
				change.Action = ActionConflict
				change.Detail = "foundation removed a locally customized managed file"
			}

		case !hadBase && hasDesired:
			switch {
			case !currentExists:
				change.Action = ActionCreate
				change.Detail = "foundation added a new managed file"
			case currentHash == desiredHash:
				change.Action = ActionUnchanged
				change.Detail = "existing downstream file already matches the new foundation"
			default:
				change.Action = ActionConflict
				change.Detail = "new foundation file collides with an existing downstream file"
			}
		}
		if change.Action == ActionConflict {
			plan.Success = false
		}
		plan.Changes = append(plan.Changes, change)
	}
	return plan, nil
}

func applyUpgrade(ctx context.Context, root string, plan UpgradePlan, desired map[string]desiredFile, records SnapshotRecordPaths) error {
	return applyUpgradeWithHooks(ctx, root, plan, desired, records, upgradeApplyHooks{})
}

func applyUpgradeWithHooks(
	ctx context.Context,
	rootPath string,
	plan UpgradePlan,
	desired map[string]desiredFile,
	records SnapshotRecordPaths,
	hooks upgradeApplyHooks,
) (result error) {
	root, err := openManagedRoot(rootPath, false)
	if err != nil {
		return err
	}
	defer root.Close()
	release, err := acquireSnapshotWriter(ctx, root)
	if err != nil {
		return err
	}
	defer release()
	if err := verifyUpgradePlanCAS(root, plan); err != nil {
		return err
	}
	backups := make(map[string]recordBackup)
	for _, change := range plan.Changes {
		if change.Path == records.LockPath || change.Path == records.ManifestPath {
			continue
		}
		switch change.Action {
		case ActionCreate, ActionUpdate, ActionDelete:
			backup, err := backupRecord(root, change.Path)
			if err != nil {
				return err
			}
			backups[change.Path] = backup
		}
	}
	needsWrite := false
	for _, change := range plan.Changes {
		switch change.Action {
		case ActionCreate, ActionUpdate, ActionDelete:
			needsWrite = true
		}
	}
	if !needsWrite {
		return nil
	}
	finishTransaction, err := root.beginSnapshotTransaction("upgrade")
	if err != nil {
		return err
	}
	applied := make([]UpgradeChange, 0, len(backups))
	defer func() {
		rollbackOK := true
		if result != nil {
			var restoreFailure *snapshotRestoreError
			if errors.As(result, &restoreFailure) {
				rollbackOK = false
			}
			for index := len(applied) - 1; index >= 0; index-- {
				change := applied[index]
				backup := backups[change.Path]
				if err := restoreRecord(root, change.Path, backup); err != nil {
					result = errors.Join(result, fmt.Errorf("roll back upgraded file %s: %w", change.Path, err))
					rollbackOK = false
					continue
				}
				if !backup.Exists {
					root.removeEmptyParents(filepath.ToSlash(filepath.Dir(filepath.FromSlash(change.Path))))
				}
			}
		}
		if rollbackOK {
			if err := finishTransaction(); err != nil {
				result = errors.Join(result, fmt.Errorf("finish blueprint snapshot transaction: %w", err))
			}
		}
	}()
	for _, change := range plan.Changes {
		if change.Path == records.LockPath || change.Path == records.ManifestPath {
			continue
		}
		switch change.Action {
		case ActionCreate, ActionUpdate, ActionDelete:
			// Atomic rename/removal can succeed before a durability flush reports
			// failure, so record the rollback obligation before applying.
			applied = append(applied, change)
		}
		if err := applyUpgradeChange(root, change, desired); err != nil {
			return err
		}
	}
	if err := verifyAppliedUpgradeState(root, plan, desired, records); err != nil {
		return err
	}
	if hooks.BeforeSnapshotCommit != nil {
		if err := hooks.BeforeSnapshotCommit(); err != nil {
			return err
		}
	}
	return commitSnapshotPair(root, records, desired, upgradeRecordExpectations(plan, records), hooks.Snapshot)
}

func verifyUpgradePlanCAS(root *managedRoot, plan UpgradePlan) error {
	for _, change := range plan.Changes {
		data, exists, _, err := root.readFile(change.Path)
		if err != nil {
			return fmt.Errorf("verify upgrade plan for %s: %w", change.Path, err)
		}
		current := ""
		if exists {
			current = digest(data)
		}
		if current != change.CurrentSHA256 {
			return fmt.Errorf("managed file %s changed after planning; regenerate the upgrade plan", change.Path)
		}
	}
	return nil
}

func verifyAppliedUpgradeState(root *managedRoot, plan UpgradePlan, desired map[string]desiredFile, records SnapshotRecordPaths) error {
	for _, change := range plan.Changes {
		if change.Path == records.LockPath || change.Path == records.ManifestPath {
			continue
		}
		data, exists, _, err := root.readFile(change.Path)
		if err != nil {
			return fmt.Errorf("verify applied upgrade file %s: %w", change.Path, err)
		}
		actual := ""
		if exists {
			actual = digest(data)
		}
		expected := change.CurrentSHA256
		switch change.Action {
		case ActionCreate, ActionUpdate:
			file, ok := desired[change.Path]
			if !ok {
				return fmt.Errorf("upgrade plan requires missing desired file %s", change.Path)
			}
			expected = digest(file.Data)
		case ActionDelete:
			expected = ""
		}
		if actual != expected {
			return fmt.Errorf("managed file %s changed during upgrade apply; snapshot was not committed", change.Path)
		}
	}
	return nil
}

func applyUpgradeChange(root *managedRoot, change UpgradeChange, desired map[string]desiredFile) error {
	data, exists, _, err := root.readFile(change.Path)
	if err != nil {
		return fmt.Errorf("verify current managed file %s: %w", change.Path, err)
	}
	current := ""
	if exists {
		current = digest(data)
	}
	if current != change.CurrentSHA256 {
		return fmt.Errorf("managed file %s changed after planning; regenerate the upgrade plan", change.Path)
	}
	switch change.Action {
	case ActionCreate, ActionUpdate:
		file, exists := desired[change.Path]
		if !exists {
			return fmt.Errorf("upgrade plan requires missing desired file %s", change.Path)
		}
		if err := root.writeAtomic(change.Path, file.Data, file.Mode); err != nil {
			return fmt.Errorf("write upgraded file %s: %w", change.Path, err)
		}
	case ActionDelete:
		if err := root.removeFile(change.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("delete obsolete managed file %s: %w", change.Path, err)
		}
		root.removeEmptyParents(filepath.ToSlash(filepath.Dir(filepath.FromSlash(change.Path))))
	case ActionUnchanged, ActionPreserve:
		return nil
	case ActionConflict:
		return errors.New("refusing to apply an upgrade plan containing conflicts")
	default:
		return fmt.Errorf("unsupported upgrade action %q", change.Action)
	}
	return nil
}

// JSON returns stable indented JSON.
func (p UpgradePlan) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// Text returns a review-oriented upgrade plan.
func (p UpgradePlan) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "mss foundation upgrade: %s\n", p.Application.Name)
	fmt.Fprintf(&builder, "blueprint: %s %s -> %s\n", p.Blueprint, p.FromBlueprintVersion, p.ToBlueprintVersion)
	fmt.Fprintf(&builder, "foundation: %s -> %s\n", p.FromFoundationCommit, p.ToFoundationCommit)
	fmt.Fprintf(&builder, "target foundation version: %s\n", p.ToIdentities.Foundation.Version)
	fmt.Fprintf(&builder, "target blueprint digest: %s\n", p.ToIdentities.Blueprint.SHA256)
	fmt.Fprintf(&builder, "target generator: %s@%s\n", p.ToIdentities.Generator.Tool, p.ToIdentities.Generator.Version)
	fmt.Fprintf(&builder, "target snapshot digest: %s\n", p.ToIdentities.Snapshot.SHA256)
	if p.LegacyInput {
		builder.WriteString("input snapshot: legacy v1alpha1 upgrade-only baseline\n")
	}
	fmt.Fprintf(&builder, "application root: %s\n", p.ApplicationRoot)
	fmt.Fprintf(&builder, "foundation root: %s\n", p.FoundationRoot)
	fmt.Fprintf(&builder, "dry run: %t\n", p.DryRun)
	fmt.Fprintf(&builder, "success: %t\n\n", p.Success)
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

func readManagedFile(root, relative string) ([]byte, bool, error) {
	if !safeRelativePath(relative) {
		return nil, false, fmt.Errorf("managed path is unsafe: %s", relative)
	}
	if err := rejectSymlinkParents(root, relative); err != nil {
		return nil, false, err
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("managed path is a symlink: %s", relative)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("managed path is not a regular file: %s", relative)
	}
	data, err := os.ReadFile(path)
	return data, err == nil, err
}

func rejectSymlinkParents(root, relative string) error {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("managed repository root must be a real directory")
	}
	parts := strings.Split(filepath.Clean(filepath.FromSlash(relative)), string(filepath.Separator))
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed path has a symlink parent: %s", relative)
		}
		if !info.IsDir() {
			return fmt.Errorf("managed path parent is not a directory: %s", relative)
		}
	}
	return nil
}

func removeEmptyParents(root, directory string) {
	root = filepath.Clean(root)
	for directory != root {
		relative, err := filepath.Rel(root, directory)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 0 {
			return
		}
		if err := os.Remove(directory); err != nil {
			return
		}
		directory = filepath.Dir(directory)
	}
}

// EqualDesired is used by tests and future protocol adapters to compare content.
func EqualDesired(data []byte, file desiredFile) bool {
	return bytes.Equal(data, file.Data)
}
