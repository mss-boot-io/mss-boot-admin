package blueprint

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const snapshotRuntimePrefix = ".mss/run/"

type recordExpectation struct {
	Exists bool
	SHA256 string
}

type recordBackup struct {
	Exists bool
	Data   []byte
	Mode   fs.FileMode
}

type snapshotRestoreError struct {
	err error
}

func (err *snapshotRestoreError) Error() string {
	return "restore previous snapshot pair: " + err.err.Error()
}

func (err *snapshotRestoreError) Unwrap() error {
	return err.err
}

type snapshotCommitHooks struct {
	BeforeLockCommit     func() error
	BeforeManifestCommit func() error
	AfterManifestCommit  func() error
}

type generationWriteHooks struct {
	BeforeSnapshotCommit func() error
	Snapshot             snapshotCommitHooks
}

type upgradeApplyHooks struct {
	BeforeSnapshotCommit func() error
	Snapshot             snapshotCommitHooks
}

func writeGeneratedSnapshot(
	ctx context.Context,
	destination string,
	blueprint *Document,
	plan Plan,
	desired map[string]desiredFile,
	initialize bool,
) error {
	return writeGeneratedSnapshotWithHooks(ctx, destination, blueprint, plan, desired, initialize, generationWriteHooks{})
}

func writeGeneratedSnapshotWithHooks(
	ctx context.Context,
	destination string,
	blueprint *Document,
	plan Plan,
	desired map[string]desiredFile,
	initialize bool,
	hooks generationWriteHooks,
) (result error) {
	root, err := openManagedRoot(destination, true)
	if err != nil {
		return err
	}
	defer root.Close()
	release, err := acquireSnapshotWriter(ctx, root)
	if err != nil {
		return err
	}
	defer release()

	current, err := planDestinationManaged(root, blueprint, plan.Application, Manifest{
		Metadata: ManifestMetadata{FoundationCommit: plan.FoundationCommit},
	}, desired, false)
	if err != nil {
		return fmt.Errorf("revalidate application generation plan: %w", err)
	}
	if !current.Success || !sameGenerationPlanState(plan, current) {
		return errors.New("application destination changed after planning; regenerate the plan")
	}
	records := SnapshotRecordPaths{
		LockPath:     normalizedPath(blueprint.Spec.LockPath),
		ManifestPath: normalizedPath(blueprint.Spec.ManifestPath),
	}
	gitExists := false
	if initialize {
		info, statErr := root.root.Lstat(".git")
		switch {
		case statErr == nil:
			if info.Mode()&os.ModeSymlink != 0 {
				return errors.New("downstream .git path must not be a symlink")
			}
			gitExists = true
		case !errors.Is(statErr, os.ErrNotExist):
			return fmt.Errorf("inspect downstream Git repository: %w", statErr)
		}
	}
	needsWrite := initialize && !gitExists
	for _, change := range plan.Changes {
		if change.Action != ActionUnchanged {
			needsWrite = true
			break
		}
	}
	if !needsWrite {
		return nil
	}
	finishTransaction, err := root.beginSnapshotTransaction("generate")
	if err != nil {
		return err
	}
	created := make([]string, 0, len(plan.Changes))
	gitCreated := false
	defer func() {
		rollbackOK := true
		if result != nil {
			var restoreFailure *snapshotRestoreError
			if errors.As(result, &restoreFailure) {
				rollbackOK = false
			}
			if gitCreated {
				if err := root.removeTree(".git"); err != nil {
					result = errors.Join(result, fmt.Errorf("roll back generated Git repository: %w", err))
					rollbackOK = false
				}
			}
			for index := len(created) - 1; index >= 0; index-- {
				relative := created[index]
				if err := root.removeFile(relative); err != nil && !errors.Is(err, os.ErrNotExist) {
					result = errors.Join(result, fmt.Errorf("roll back generated file %s: %w", relative, err))
					rollbackOK = false
					continue
				}
				root.removeEmptyParents(filepath.ToSlash(filepath.Dir(filepath.FromSlash(relative))))
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
		if change.Action == ActionUnchanged {
			continue
		}
		if change.Action != ActionCreate {
			return fmt.Errorf("refusing unexpected generation action %s for %s", change.Action, change.Path)
		}
		file, ok := desired[change.Path]
		if !ok {
			return fmt.Errorf("generation plan requires missing desired file %s", change.Path)
		}
		if _, exists, _, err := root.readFile(change.Path); err != nil {
			return fmt.Errorf("verify generation target %s: %w", change.Path, err)
		} else if exists {
			return fmt.Errorf("generation target %s changed after planning", change.Path)
		}
		// A post-rename durability error means the target can exist even though
		// writeDesiredFile returns an error, so install the rollback obligation
		// before invoking the atomic writer.
		created = append(created, change.Path)
		if err := writeDesiredFile(root, change.Path, file); err != nil {
			return err
		}
	}
	if initialize {
		gitCreated = !gitExists
		if err := initializeGit(ctx, root); err != nil {
			return err
		}
	}
	if err := verifyGeneratedState(root, desired, records); err != nil {
		return err
	}
	if hooks.BeforeSnapshotCommit != nil {
		if err := hooks.BeforeSnapshotCommit(); err != nil {
			return err
		}
	}
	expected := generationRecordExpectations(plan, records)
	if err := commitSnapshotPair(root, records, desired, expected, hooks.Snapshot); err != nil {
		return err
	}
	return nil
}

func verifyGeneratedState(root *managedRoot, desired map[string]desiredFile, records SnapshotRecordPaths) error {
	for relative, file := range desired {
		if relative == records.LockPath || relative == records.ManifestPath {
			continue
		}
		data, exists, _, err := root.readFile(relative)
		if err != nil {
			return fmt.Errorf("verify generated file %s: %w", relative, err)
		}
		if !exists || digest(data) != digest(file.Data) {
			return fmt.Errorf("generated file %s changed before snapshot commit", relative)
		}
	}
	return nil
}

func sameGenerationPlanState(original, current Plan) bool {
	if len(original.Changes) != len(current.Changes) {
		return false
	}
	byPath := make(map[string]FileChange, len(original.Changes))
	for _, change := range original.Changes {
		byPath[change.Path] = change
	}
	for _, change := range current.Changes {
		before, ok := byPath[change.Path]
		if !ok || before.Action != change.Action || before.SHA256 != change.SHA256 {
			return false
		}
	}
	return true
}

func generationRecordExpectations(plan Plan, records SnapshotRecordPaths) map[string]recordExpectation {
	result := map[string]recordExpectation{}
	for _, change := range plan.Changes {
		if change.Path != records.LockPath && change.Path != records.ManifestPath {
			continue
		}
		expectation := recordExpectation{}
		if change.Action == ActionUnchanged {
			expectation.Exists = true
			expectation.SHA256 = change.SHA256
		}
		result[change.Path] = expectation
	}
	return result
}

func upgradeRecordExpectations(plan UpgradePlan, records SnapshotRecordPaths) map[string]recordExpectation {
	result := map[string]recordExpectation{}
	for _, change := range plan.Changes {
		if change.Path != records.LockPath && change.Path != records.ManifestPath {
			continue
		}
		result[change.Path] = recordExpectation{
			Exists: change.CurrentSHA256 != "",
			SHA256: change.CurrentSHA256,
		}
	}
	return result
}

func commitSnapshotPair(
	root *managedRoot,
	records SnapshotRecordPaths,
	desired map[string]desiredFile,
	expected map[string]recordExpectation,
	hooks snapshotCommitHooks,
) error {
	lockFile, ok := desired[records.LockPath]
	if !ok {
		return fmt.Errorf("desired snapshot is missing lock record %s", records.LockPath)
	}
	manifestFile, ok := desired[records.ManifestPath]
	if !ok {
		return fmt.Errorf("desired snapshot is missing manifest record %s", records.ManifestPath)
	}
	manifest, legacy, err := decodeManifest(manifestFile.Data, false)
	if err != nil || legacy {
		if err == nil {
			err = errors.New("desired snapshot manifest unexpectedly uses the legacy schema")
		}
		return err
	}
	lock, err := decodeFoundationLock(lockFile.Data)
	if err != nil {
		return err
	}
	if err := validateSnapshotPair(manifest, lock, lockFile.Data); err != nil {
		return fmt.Errorf("validate desired snapshot pair: %w", err)
	}
	for _, relative := range []string{records.LockPath, records.ManifestPath} {
		expectation, ok := expected[relative]
		if !ok {
			return fmt.Errorf("snapshot transaction is missing the planned current state for %s", relative)
		}
		if err := verifyRecordExpectation(root, relative, expectation); err != nil {
			return err
		}
	}
	lockExpectation := expected[records.LockPath]
	manifestExpectation := expected[records.ManifestPath]
	if lockExpectation.Exists && manifestExpectation.Exists &&
		lockExpectation.SHA256 == digest(lockFile.Data) &&
		manifestExpectation.SHA256 == digest(manifestFile.Data) {
		return nil
	}
	lockBackup, err := backupRecord(root, records.LockPath)
	if err != nil {
		return err
	}
	manifestBackup, err := backupRecord(root, records.ManifestPath)
	if err != nil {
		return err
	}
	stagedLock, err := stageRecord(root, records.LockPath, lockFile)
	if err != nil {
		return err
	}
	defer stagedLock.Close()
	stagedManifest, err := stageRecord(root, records.ManifestPath, manifestFile)
	if err != nil {
		return err
	}
	defer stagedManifest.Close()

	lockCommitted := false
	manifestCommitted := false
	rollback := func(cause error) error {
		if !lockCommitted && !manifestCommitted {
			return cause
		}
		if restoreErr := restoreSnapshotPair(root, records, lockBackup, manifestBackup, lockCommitted, manifestCommitted); restoreErr != nil {
			return errors.Join(cause, &snapshotRestoreError{err: restoreErr})
		}
		return cause
	}
	if hooks.BeforeLockCommit != nil {
		if err := hooks.BeforeLockCommit(); err != nil {
			return rollback(err)
		}
	}
	// Commit can rename successfully and then fail its durability flush. Mark
	// the record first so every error path restores the planned previous bytes.
	lockCommitted = true
	if err := stagedLock.Commit(); err != nil {
		return rollback(fmt.Errorf("commit foundation lock: %w", err))
	}
	if hooks.BeforeManifestCommit != nil {
		if err := hooks.BeforeManifestCommit(); err != nil {
			return rollback(err)
		}
	}
	manifestCommitted = true
	if err := stagedManifest.Commit(); err != nil {
		return rollback(fmt.Errorf("commit blueprint manifest: %w", err))
	}
	if hooks.AfterManifestCommit != nil {
		if err := hooks.AfterManifestCommit(); err != nil {
			return rollback(err)
		}
	}
	if _, err := readSnapshotUnlocked(root, records.ManifestPath); err != nil {
		return rollback(fmt.Errorf("verify committed snapshot pair: %w", err))
	}
	return nil
}

func verifyRecordExpectation(root *managedRoot, relative string, expectation recordExpectation) error {
	data, exists, _, err := root.readFile(relative)
	if err != nil {
		return fmt.Errorf("verify planned snapshot record %s: %w", relative, err)
	}
	if exists != expectation.Exists {
		return fmt.Errorf("snapshot record %s changed after planning", relative)
	}
	if exists && digest(data) != expectation.SHA256 {
		return fmt.Errorf("snapshot record %s changed after planning", relative)
	}
	return nil
}

func backupRecord(root *managedRoot, relative string) (recordBackup, error) {
	data, exists, mode, err := root.readFile(relative)
	if err != nil {
		return recordBackup{}, fmt.Errorf("backup snapshot record %s: %w", relative, err)
	}
	if !exists {
		return recordBackup{}, nil
	}
	return recordBackup{Exists: true, Data: data, Mode: mode.Perm()}, nil
}

type stagedRecord struct {
	parent    *os.Root
	temporary string
	target    string
	relative  string
}

func stageRecord(root *managedRoot, relative string, file desiredFile) (*stagedRecord, error) {
	parent, target, err := root.openParent(relative, true)
	if err != nil {
		return nil, err
	}
	temporary, name, err := root.createTemporary(parent, ".mss-snapshot-", file.Mode)
	if err != nil {
		_ = parent.Close()
		return nil, fmt.Errorf("stage snapshot record %s: %w", relative, err)
	}
	cleanup := func(cause error) (*stagedRecord, error) {
		_ = temporary.Close()
		_ = parent.Remove(name)
		_ = parent.Close()
		return nil, cause
	}
	if err := temporary.Chmod(file.Mode.Perm()); err != nil {
		return cleanup(fmt.Errorf("chmod staged snapshot record %s: %w", relative, err))
	}
	if _, err := temporary.Write(file.Data); err != nil {
		return cleanup(fmt.Errorf("write staged snapshot record %s: %w", relative, err))
	}
	if err := temporary.Sync(); err != nil {
		return cleanup(fmt.Errorf("sync staged snapshot record %s: %w", relative, err))
	}
	if err := temporary.Close(); err != nil {
		_ = parent.Remove(name)
		_ = parent.Close()
		return nil, fmt.Errorf("close staged snapshot record %s: %w", relative, err)
	}
	return &stagedRecord{parent: parent, temporary: name, target: target, relative: relative}, nil
}

func (record *stagedRecord) Commit() error {
	if err := record.parent.Rename(record.temporary, record.target); err != nil {
		return err
	}
	record.temporary = ""
	return syncCommittedEntry(record.parent, record.target)
}

func (record *stagedRecord) Close() error {
	if record == nil {
		return nil
	}
	if record.temporary != "" {
		_ = record.parent.Remove(record.temporary)
	}
	return record.parent.Close()
}

func restoreSnapshotPair(
	root *managedRoot,
	records SnapshotRecordPaths,
	lock, manifest recordBackup,
	lockCommitted, manifestCommitted bool,
) error {
	var result error
	if lockCommitted {
		if err := restoreRecord(root, records.LockPath, lock); err != nil {
			result = errors.Join(result, err)
		}
	}
	// Manifest is restored last because it is the snapshot commit marker.
	if manifestCommitted {
		if err := restoreRecord(root, records.ManifestPath, manifest); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func restoreRecord(root *managedRoot, relative string, backup recordBackup) error {
	if !backup.Exists {
		if err := root.removeFile(relative); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove new snapshot record %s: %w", relative, err)
		}
		return nil
	}
	if err := root.writeAtomic(relative, backup.Data, backup.Mode); err != nil {
		return fmt.Errorf("restore snapshot record %s: %w", relative, err)
	}
	return nil
}

func writeDesiredFile(root *managedRoot, relative string, file desiredFile) error {
	if err := root.writeAtomic(relative, file.Data, file.Mode); err != nil {
		return fmt.Errorf("write %s: %w", relative, err)
	}
	return nil
}
