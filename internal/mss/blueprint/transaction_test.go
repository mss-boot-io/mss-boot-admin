package blueprint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateWritesSnapshotPairAfterManagedFiles(t *testing.T) {
	fixture := generationTransactionFixture(t)
	observedManagedBoundary := false
	observedManifestBoundary := false
	err := writeGeneratedSnapshotWithHooks(
		context.Background(),
		fixture.destination,
		fixture.blueprint,
		fixture.plan,
		fixture.desired,
		false,
		generationWriteHooks{BeforeSnapshotCommit: func() error {
			observedManagedBoundary = true
			if _, err := os.Stat(filepath.Join(fixture.destination, "admin", "main.go")); err != nil {
				return errors.New("ordinary managed file was not written before snapshot commit")
			}
			for _, relative := range []string{fixture.records.LockPath, fixture.records.ManifestPath} {
				if _, err := os.Stat(filepath.Join(fixture.destination, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
					return errors.New("snapshot record existed before snapshot commit")
				}
			}
			return nil
		}, Snapshot: snapshotCommitHooks{BeforeManifestCommit: func() error {
			observedManifestBoundary = true
			if _, err := os.Stat(filepath.Join(fixture.destination, filepath.FromSlash(fixture.records.LockPath))); err != nil {
				return errors.New("lock record was not committed before the manifest boundary")
			}
			if _, err := os.Stat(filepath.Join(fixture.destination, filepath.FromSlash(fixture.records.ManifestPath))); !errors.Is(err, os.ErrNotExist) {
				return errors.New("manifest record existed before its final commit boundary")
			}
			return nil
		}},
		},
	)
	if err != nil {
		t.Fatalf("writeGeneratedSnapshotWithHooks() error = %v", err)
	}
	if !observedManagedBoundary || !observedManifestBoundary {
		t.Fatal("snapshot ordering boundaries were not both observed")
	}
	if _, err := ReadSnapshot(fixture.destination, fixture.records.ManifestPath); err != nil {
		t.Fatalf("ReadSnapshot() after generation error = %v", err)
	}
}

func TestGenerateFailureBeforeSnapshotCommitRollsBackManagedFilesAndSnapshot(t *testing.T) {
	fixture := generationTransactionFixture(t)
	want := errors.New("injected before snapshot")
	err := writeGeneratedSnapshotWithHooks(
		context.Background(), fixture.destination, fixture.blueprint, fixture.plan, fixture.desired, false,
		generationWriteHooks{BeforeSnapshotCommit: func() error { return want }},
	)
	if !errors.Is(err, want) {
		t.Fatalf("writeGeneratedSnapshotWithHooks() error = %v, want injected error", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.destination, "admin", "main.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ordinary managed file survived rollback: %v", err)
	}
	assertSnapshotRecordsAbsent(t, fixture.destination, fixture.records)
}

func TestGenerateManifestCommitFailureRemovesNewLock(t *testing.T) {
	fixture := generationTransactionFixture(t)
	want := errors.New("injected manifest commit failure")
	err := writeGeneratedSnapshotWithHooks(
		context.Background(), fixture.destination, fixture.blueprint, fixture.plan, fixture.desired, false,
		generationWriteHooks{Snapshot: snapshotCommitHooks{
			BeforeManifestCommit: func() error { return want },
		}},
	)
	if !errors.Is(err, want) {
		t.Fatalf("writeGeneratedSnapshotWithHooks() error = %v, want injected error", err)
	}
	assertSnapshotRecordsAbsent(t, fixture.destination, fixture.records)
}

func TestUpgradeFailureBeforeSnapshotCommitRetainsPreviousSnapshotPair(t *testing.T) {
	fixture := upgradeTransactionFixture(t)
	oldLock, oldManifest := readSnapshotRecordBytes(t, fixture.applicationRoot, fixture.records)
	oldMain, err := os.ReadFile(filepath.Join(fixture.applicationRoot, "admin", "main.go"))
	if err != nil {
		t.Fatalf("read old managed file: %v", err)
	}
	oldRemoved, err := os.ReadFile(filepath.Join(fixture.applicationRoot, "web", "antd", "public", "fixture.bin"))
	if err != nil {
		t.Fatalf("read old soon-to-be-removed file: %v", err)
	}
	want := errors.New("injected before upgrade snapshot")
	err = applyUpgradeWithHooks(
		context.Background(), fixture.applicationRoot, fixture.plan, fixture.desired, fixture.records,
		upgradeApplyHooks{BeforeSnapshotCommit: func() error { return want }},
	)
	if !errors.Is(err, want) {
		t.Fatalf("applyUpgradeWithHooks() error = %v, want injected error", err)
	}
	assertSnapshotRecordBytes(t, fixture.applicationRoot, fixture.records, oldLock, oldManifest)
	gotMain, err := os.ReadFile(filepath.Join(fixture.applicationRoot, "admin", "main.go"))
	if err != nil || !bytes.Equal(gotMain, oldMain) {
		t.Fatalf("updated managed file was not rolled back: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.applicationRoot, "NEW.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created managed file survived rollback: %v", err)
	}
	gotRemoved, err := os.ReadFile(filepath.Join(fixture.applicationRoot, "web", "antd", "public", "fixture.bin"))
	if err != nil || !bytes.Equal(gotRemoved, oldRemoved) {
		t.Fatalf("deleted managed file was not restored: err=%v", err)
	}
}

func TestUpgradeManifestCommitFailureRestoresPreviousSnapshotPair(t *testing.T) {
	fixture := upgradeTransactionFixture(t)
	oldLock, oldManifest := readSnapshotRecordBytes(t, fixture.applicationRoot, fixture.records)
	want := errors.New("injected upgrade manifest commit failure")
	err := applyUpgradeWithHooks(
		context.Background(), fixture.applicationRoot, fixture.plan, fixture.desired, fixture.records,
		upgradeApplyHooks{Snapshot: snapshotCommitHooks{
			BeforeManifestCommit: func() error { return want },
		}},
	)
	if !errors.Is(err, want) {
		t.Fatalf("applyUpgradeWithHooks() error = %v, want injected error", err)
	}
	assertSnapshotRecordBytes(t, fixture.applicationRoot, fixture.records, oldLock, oldManifest)
}

func TestUpgradePostCommitFailureRestoresPreviousSnapshotPair(t *testing.T) {
	fixture := upgradeTransactionFixture(t)
	oldLock, oldManifest := readSnapshotRecordBytes(t, fixture.applicationRoot, fixture.records)
	want := errors.New("injected post-commit verification failure")
	err := applyUpgradeWithHooks(
		context.Background(), fixture.applicationRoot, fixture.plan, fixture.desired, fixture.records,
		upgradeApplyHooks{Snapshot: snapshotCommitHooks{
			AfterManifestCommit: func() error { return want },
		}},
	)
	if !errors.Is(err, want) {
		t.Fatalf("applyUpgradeWithHooks() error = %v, want injected error", err)
	}
	assertSnapshotRecordBytes(t, fixture.applicationRoot, fixture.records, oldLock, oldManifest)
}

func TestUpgradeApplyRejectsCurrentFileChangedAfterPlan(t *testing.T) {
	fixture := upgradeTransactionFixture(t)
	path := filepath.Join(fixture.applicationRoot, "admin", "main.go")
	if err := os.WriteFile(path, []byte("package changed_after_plan\n"), 0o644); err != nil {
		t.Fatalf("change planned file: %v", err)
	}
	err := applyUpgrade(context.Background(), fixture.applicationRoot, fixture.plan, fixture.desired, fixture.records)
	if err == nil || !strings.Contains(err.Error(), "changed after planning") {
		t.Fatalf("applyUpgrade() error = %v, want apply-time CAS rejection", err)
	}
}

func TestUpgradeSerializesConcurrentSnapshotWriters(t *testing.T) {
	fixture := upgradeTransactionFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- applyUpgradeWithHooks(
			context.Background(), fixture.applicationRoot, fixture.plan, fixture.desired, fixture.records,
			upgradeApplyHooks{BeforeSnapshotCommit: func() error {
				close(entered)
				<-release
				return nil
			}},
		)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first writer did not reach the serialized snapshot boundary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	secondErr := applyUpgrade(ctx, fixture.applicationRoot, fixture.plan, fixture.desired, fixture.records)
	if !errors.Is(secondErr, context.DeadlineExceeded) {
		t.Fatalf("second writer error = %v, want context deadline while waiting for writer lock", secondErr)
	}
	close(release)
	if err := <-firstResult; err != nil {
		t.Fatalf("first writer error = %v", err)
	}
}

func TestReadSnapshotWaitsForAtomicPairCommit(t *testing.T) {
	fixture := generationTransactionFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	writeResult := make(chan error, 1)
	go func() {
		writeResult <- writeGeneratedSnapshotWithHooks(
			context.Background(), fixture.destination, fixture.blueprint, fixture.plan, fixture.desired, false,
			generationWriteHooks{Snapshot: snapshotCommitHooks{
				BeforeManifestCommit: func() error {
					close(entered)
					<-release
					return nil
				},
			}},
		)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not reach the lock-first manifest boundary")
	}
	readResult := make(chan error, 1)
	go func() {
		_, err := ReadSnapshot(fixture.destination, fixture.records.ManifestPath)
		readResult <- err
	}()
	select {
	case err := <-readResult:
		t.Fatalf("snapshot reader observed an in-flight pair instead of waiting: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-writeResult; err != nil {
		t.Fatalf("snapshot writer error = %v", err)
	}
	if err := <-readResult; err != nil {
		t.Fatalf("snapshot reader error after commit = %v", err)
	}
}

func TestSnapshotReaderCreatesSharedLockBeforeFirstWriter(t *testing.T) {
	rootPath := t.TempDir()
	readerRoot, err := openManagedRoot(rootPath, false)
	if err != nil {
		t.Fatalf("open reader root: %v", err)
	}
	defer readerRoot.Close()
	releaseReader, err := acquireSnapshotReader(readerRoot)
	if err != nil {
		t.Fatalf("acquire first reader: %v", err)
	}
	defer releaseReader()
	if _, err := os.Stat(filepath.Join(rootPath, filepath.FromSlash(snapshotLockPath))); err != nil {
		t.Fatalf("first reader did not create the shared lock inode: %v", err)
	}
	writerRoot, err := openManagedRoot(rootPath, false)
	if err != nil {
		t.Fatalf("open writer root: %v", err)
	}
	defer writerRoot.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := acquireSnapshotWriter(ctx, writerRoot); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first writer error = %v, want deadline while reader owns shared lock", err)
	}
}

func TestSnapshotWriterLockSerializesAcrossProcesses(t *testing.T) {
	if os.Getenv("MSS_BLUEPRINT_LOCK_HELPER") == "1" {
		runSnapshotLockHelper(t)
		return
	}
	rootPath := t.TempDir()
	ready := filepath.Join(t.TempDir(), "ready")
	releasePath := filepath.Join(t.TempDir(), "release")
	command := exec.Command(os.Args[0], "-test.run=^TestSnapshotWriterLockSerializesAcrossProcesses$")
	command.Env = append(os.Environ(),
		"MSS_BLUEPRINT_LOCK_HELPER=1",
		"MSS_BLUEPRINT_LOCK_ROOT="+rootPath,
		"MSS_BLUEPRINT_LOCK_READY="+ready,
		"MSS_BLUEPRINT_LOCK_RELEASE="+releasePath,
	)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start lock helper: %v", err)
	}
	t.Cleanup(func() {
		if command != nil {
			_ = os.WriteFile(releasePath, []byte("release\n"), 0o600)
			_ = command.Wait()
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cross-process writer did not acquire lock: %s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	root, err := openManagedRoot(rootPath, false)
	if err != nil {
		t.Fatalf("open competing writer root: %v", err)
	}
	defer root.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	if _, err := acquireSnapshotWriter(ctx, root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("competing process writer error = %v, want deadline", err)
	}
	if err := os.WriteFile(releasePath, []byte("release\n"), 0o600); err != nil {
		t.Fatalf("release helper: %v", err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("lock helper failed: %v: %s", err, stderr.String())
	}
	command = nil
}

func runSnapshotLockHelper(t *testing.T) {
	root, err := openManagedRoot(os.Getenv("MSS_BLUEPRINT_LOCK_ROOT"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	release, err := acquireSnapshotWriter(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := os.WriteFile(os.Getenv("MSS_BLUEPRINT_LOCK_READY"), []byte("ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(os.Getenv("MSS_BLUEPRINT_LOCK_RELEASE")); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for helper release")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestReadSnapshotRejectsIncompleteTransactionJournal(t *testing.T) {
	rootPath, manifest := generatedSnapshotFixture(t)
	root, err := openManagedRoot(rootPath, false)
	if err != nil {
		t.Fatalf("open managed root: %v", err)
	}
	finish, err := root.beginSnapshotTransaction("crash-test")
	if err != nil {
		_ = root.Close()
		t.Fatalf("begin crash fixture: %v", err)
	}
	_ = root.Close()
	defer finish()
	if _, err := ReadSnapshot(rootPath, manifest.Records.ManifestPath); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("ReadSnapshot() error = %v, want incomplete transaction detection", err)
	}
}

func TestPinnedManagedRootCannotBeRedirectedOutsideRepository(t *testing.T) {
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "repository")
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(rootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := openManagedRoot(rootPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	moved := filepath.Join(parent, "repository-moved")
	if err := os.Rename(rootPath, moved); err != nil {
		t.Skipf("platform cannot rename an opened directory: %v", err)
	}
	if err := os.Symlink(outside, rootPath); err != nil {
		t.Skipf("platform cannot create the path-swap symlink: %v", err)
	}
	if err := root.writeAtomic("nested/result.txt", []byte("confined\n"), 0o644); err != nil {
		t.Fatalf("write through pinned root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "nested", "result.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("write escaped through replaced root path: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(moved, "nested", "result.txt"))
	if err != nil || string(data) != "confined\n" {
		t.Fatalf("pinned root did not retain original directory: data=%q err=%v", data, err)
	}
}

func TestManagedRootRejectsSymlinkInRootPathComponents(t *testing.T) {
	parent := t.TempDir()
	outside := filepath.Join(parent, "outside")
	rootPath := filepath.Join(outside, "repository")
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(outside, alias); err != nil {
		t.Skipf("platform cannot create root-component symlink fixture: %v", err)
	}
	root, err := openManagedRoot(filepath.Join(alias, "repository"), false)
	if root != nil {
		_ = root.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "not a real directory") {
		t.Fatalf("openManagedRoot() error = %v, want root component symlink rejection", err)
	}
}

func TestSnapshotLockRejectsFinalSymlink(t *testing.T) {
	rootPath := t.TempDir()
	lockPath := filepath.Join(rootPath, filepath.FromSlash(snapshotLockPath))
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.lock")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, lockPath); err != nil {
		t.Skipf("platform cannot create symlink fixture: %v", err)
	}
	root, err := openManagedRoot(rootPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := acquireSnapshotWriter(context.Background(), root); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("acquireSnapshotWriter() error = %v, want final symlink rejection", err)
	}
}

func TestSnapshotLockDetectsPathReplacementDuringAcquisition(t *testing.T) {
	rootPath := t.TempDir()
	root, err := openManagedRoot(rootPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := root.openLockFile()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	lockPath := filepath.Join(rootPath, filepath.FromSlash(snapshotLockPath))
	movedPath := lockPath + ".replaced"
	if err := os.Rename(lockPath, movedPath); err != nil {
		t.Skipf("platform prevents replacement of an opened lock file: %v", err)
	}
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.verifyLockFile(file); err == nil || !strings.Contains(err.Error(), "replaced") {
		t.Fatalf("verifyLockFile() error = %v, want path replacement rejection", err)
	}
}

type generationTransactionState struct {
	destination string
	blueprint   *Document
	desired     map[string]desiredFile
	plan        Plan
	records     SnapshotRecordPaths
}

func generationTransactionFixture(t *testing.T) generationTransactionState {
	t.Helper()
	foundation, blueprint, application := identityFixture(t)
	desired, manifest, err := BuildDesired(context.Background(), foundation, blueprint, application)
	if err != nil {
		t.Fatalf("BuildDesired() error = %v", err)
	}
	destination := filepath.Join(t.TempDir(), "generated-admin")
	plan, err := planDestination(destination, blueprint, application, manifest, desired, true)
	if err != nil {
		t.Fatalf("planDestination() error = %v", err)
	}
	if !plan.Success {
		t.Fatalf("generation plan is not successful: %#v", plan)
	}
	return generationTransactionState{
		destination: destination,
		blueprint:   blueprint,
		desired:     desired,
		plan:        plan,
		records: SnapshotRecordPaths{
			LockPath:     blueprint.Spec.LockPath,
			ManifestPath: blueprint.Spec.ManifestPath,
		},
	}
}

type upgradeTransactionState struct {
	applicationRoot string
	desired         map[string]desiredFile
	plan            UpgradePlan
	records         SnapshotRecordPaths
}

func upgradeTransactionFixture(t *testing.T) upgradeTransactionState {
	t.Helper()
	oldFoundation := writeBlueprintFixture(t)
	newFoundation := writeBlueprintFixture(t)
	prepareNewFoundation(t, newFoundation)
	applicationRoot := filepath.Join(t.TempDir(), "upgrade-admin")
	application := Application{
		Name:        "upgrade-admin",
		DisplayName: "Upgrade Administration",
		Module:      "github.com/acme/upgrade-admin",
		Repository:  "acme/upgrade-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation,
		Destination:    applicationRoot,
		Application:    application,
		Write:          true,
	}); err != nil {
		t.Fatalf("generate old application: %v", err)
	}
	oldManifest, err := ReadManifest(applicationRoot, "")
	if err != nil {
		t.Fatalf("read old manifest: %v", err)
	}
	newBlueprint, err := Load(newFoundation, oldManifest.Metadata.Blueprint)
	if err != nil {
		t.Fatalf("load new blueprint: %v", err)
	}
	desired, newManifest, err := BuildDesired(context.Background(), newFoundation, newBlueprint, application)
	if err != nil {
		t.Fatalf("build new desired state: %v", err)
	}
	plan, err := buildUpgradePlan(
		applicationRoot,
		newFoundation,
		oldManifest.Records.ManifestPath,
		oldManifest.Records.LockPath,
		oldManifest,
		newBlueprint,
		newManifest,
		desired,
		false,
		application,
	)
	if err != nil {
		t.Fatalf("build upgrade plan: %v", err)
	}
	if !plan.Success {
		t.Fatalf("upgrade plan is not successful: %#v", plan)
	}
	return upgradeTransactionState{
		applicationRoot: applicationRoot,
		desired:         desired,
		plan:            plan,
		records:         oldManifest.Records.SnapshotRecordPaths,
	}
}

func assertSnapshotRecordsAbsent(t *testing.T, root string, records SnapshotRecordPaths) {
	t.Helper()
	for _, relative := range []string{records.LockPath, records.ManifestPath} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot record %s exists after failed transaction: %v", relative, err)
		}
	}
}

func readSnapshotRecordBytes(t *testing.T, root string, records SnapshotRecordPaths) ([]byte, []byte) {
	t.Helper()
	lock, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(records.LockPath)))
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(records.ManifestPath)))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return lock, manifest
}

func assertSnapshotRecordBytes(t *testing.T, root string, records SnapshotRecordPaths, wantLock, wantManifest []byte) {
	t.Helper()
	gotLock, gotManifest := readSnapshotRecordBytes(t, root, records)
	if !bytes.Equal(gotLock, wantLock) {
		t.Fatal("foundation lock was not restored byte-for-byte")
	}
	if !bytes.Equal(gotManifest, wantManifest) {
		t.Fatal("blueprint manifest was not restored byte-for-byte")
	}
	if _, err := ReadSnapshot(root, records.ManifestPath); err != nil {
		t.Fatalf("restored snapshot pair does not validate: %v", err)
	}
}
