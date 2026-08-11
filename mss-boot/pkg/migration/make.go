package migration

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/12 09:15:17
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/12 09:15:17
 */

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"gorm.io/gorm"
)

//go:embed *.tpl
var FS embed.FS

var Migrate = New()

type registeredMigration struct {
	id        MigrationID
	legacyIDs []MigrationID
	run       MigrationFunc
}

type Migration struct {
	db    *gorm.DB
	Model Version

	mutex              sync.RWMutex
	runMutex           sync.Mutex
	versions           map[MigrationID]registeredMigration
	legacyOwners       map[MigrationID][]MigrationID
	registrationErrors []error
}

// New returns an isolated migration runner. Registrations are deterministic
// regardless of registration order because MigrateContext sorts by MigrationID.
func New() *Migration {
	return &Migration{
		versions:     make(map[MigrationID]registeredMigration),
		legacyOwners: make(map[MigrationID][]MigrationID),
	}
}

func (e *Migration) GetDb() *gorm.DB {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.db
}

func (e *Migration) SetDb(db *gorm.DB) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.db = db
}

func (e *Migration) SetModel(v Version) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Model = v
}

// Register adds a migration under its complete, lossless ID.
//
// A registration error is retained for MigrateContext preflight even when a
// compatibility caller ignores the returned error. This prevents a duplicate
// registration from being overwritten and then silently executed.
func (e *Migration) Register(id MigrationID, f MigrationFunc) error {
	return e.register(id, nil, f)
}

// RegisterWithLegacyIDs associates explicitly reviewed, previously persisted
// IDs with a canonical migration. Legacy aliases must never be inferred for new
// migrations because the old 13-character format can collide.
func (e *Migration) RegisterWithLegacyIDs(
	id MigrationID,
	legacyIDs []MigrationID,
	f MigrationFunc,
) error {
	return e.register(id, legacyIDs, f)
}

func (e *Migration) register(id MigrationID, legacyIDs []MigrationID, f MigrationFunc) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.versions == nil {
		e.versions = make(map[MigrationID]registeredMigration)
	}
	if e.legacyOwners == nil {
		e.legacyOwners = make(map[MigrationID][]MigrationID)
	}

	if err := id.validate(); err != nil {
		return e.recordRegistrationErrorLocked(err)
	}
	if f == nil {
		return e.recordRegistrationErrorLocked(
			fmt.Errorf("migration %s: function is nil", id),
		)
	}
	if _, exists := e.versions[id]; exists {
		return e.recordRegistrationErrorLocked(
			fmt.Errorf("%w: %s", ErrDuplicateMigrationID, id),
		)
	}
	if owners, exists := e.legacyOwners[id]; exists {
		return e.recordRegistrationErrorLocked(fmt.Errorf(
			"%w: canonical ID %s is already a legacy alias of %v",
			ErrDuplicateMigrationID,
			id,
			owners,
		))
	}

	aliases := make([]MigrationID, 0, len(legacyIDs))
	seenAliases := make(map[MigrationID]struct{}, len(legacyIDs))
	for _, legacyID := range legacyIDs {
		if legacyID == id {
			continue
		}
		if err := legacyID.validate(); err != nil {
			return e.recordRegistrationErrorLocked(fmt.Errorf(
				"migration %s legacy alias: %w",
				id,
				err,
			))
		}
		if _, duplicate := seenAliases[legacyID]; duplicate {
			return e.recordRegistrationErrorLocked(fmt.Errorf(
				"%w: legacy ID %s is repeated for %s",
				ErrDuplicateMigrationID,
				legacyID,
				id,
			))
		}
		if _, exists := e.versions[legacyID]; exists {
			return e.recordRegistrationErrorLocked(fmt.Errorf(
				"%w: legacy ID %s is already a canonical migration",
				ErrDuplicateMigrationID,
				legacyID,
			))
		}
		seenAliases[legacyID] = struct{}{}
		aliases = append(aliases, legacyID)
	}

	sortMigrationIDs(aliases)
	e.versions[id] = registeredMigration{id: id, legacyIDs: aliases, run: f}
	for _, legacyID := range aliases {
		e.legacyOwners[legacyID] = append(e.legacyOwners[legacyID], id)
	}
	return nil
}

func (e *Migration) recordRegistrationErrorLocked(err error) error {
	e.registrationErrors = append(e.registrationErrors, err)
	return err
}

func (e *Migration) recordRegistrationError(err error) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.recordRegistrationErrorLocked(err)
}

// ValidateRegistrations reports every registration error without requiring a
// database handle or version model. Composition roots must call this before
// creating or altering the migration table so duplicate or malformed IDs fail
// before any database work starts.
func (e *Migration) ValidateRegistrations() error {
	if e == nil {
		return fmt.Errorf("%w: migration runner is nil", ErrMigrationNotReady)
	}
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.validateRegistrationsLocked()
}

func (e *Migration) validateRegistrationsLocked() error {
	if len(e.registrationErrors) == 0 {
		return nil
	}
	return fmt.Errorf(
		"migration registration preflight: %w",
		errors.Join(e.registrationErrors...),
	)
}

// SetVersion preserves the v1.0 registration signature for downstream source
// compatibility. Registration failures are retained for migration preflight.
//
// Deprecated: use Register with a typed MigrationID.
func (e *Migration) SetVersion(k int, f func(db *gorm.DB, version string) error) {
	id := MigrationID(strconv.Itoa(k))
	if err := id.validate(); err != nil {
		e.recordRegistrationError(err)
		return
	}
	e.Register(id, MigrationFunc(f))
}

// SetV100Version registers a migration that shipped in v1.0.0 with its exact
// canonical ID and the historical ID that v1.0.0 persisted. Explicit use at a
// reviewed old call site keeps collision compatibility out of future generated
// migrations. Multiple canonical migrations may intentionally share one
// historical alias: the old runner could persist only that ambiguous row, so
// every migration in the reviewed collision group treats it as applied.
// Renamed historical files must use RegisterWithLegacyIDs with their original
// persisted ID instead of deriving an alias from the current filename.
func (e *Migration) SetV100Version(
	filename string,
	f func(db *gorm.DB, version string) error,
) error {
	id := FilenameMigrationID(filename)
	legacyID, err := V100MigrationIDFromFilename(filename)
	if err != nil {
		return e.recordRegistrationError(err)
	}
	if legacyID == id {
		return e.Register(id, MigrationFunc(f))
	}
	return e.RegisterWithLegacyIDs(id, []MigrationID{legacyID}, MigrationFunc(f))
}

func (e *Migration) cloneModel() (Version, error) {
	e.mutex.RLock()
	model := e.Model
	e.mutex.RUnlock()
	modelType, err := versionModelType(model)
	if err != nil {
		return nil, err
	}
	return newVersion(modelType)
}

func versionModelType(model Version) (reflect.Type, error) {
	if model == nil {
		return nil, fmt.Errorf("%w: version model is nil", ErrMigrationNotReady)
	}
	t := reflect.TypeOf(model)
	v := reflect.ValueOf(model)
	if t.Kind() != reflect.Pointer || v.IsNil() {
		return nil, fmt.Errorf("%w: version model %T must be a non-nil pointer", ErrMigrationNotReady, model)
	}
	modelType := t.Elem()
	if _, ok := reflect.New(modelType).Interface().(Version); !ok {
		return nil, fmt.Errorf("%w: version model %T cannot be cloned", ErrMigrationNotReady, model)
	}
	return modelType, nil
}

func newVersion(modelType reflect.Type) (Version, error) {
	model, ok := reflect.New(modelType).Interface().(Version)
	if !ok {
		return nil, fmt.Errorf("%w: version model %s cannot be cloned", ErrMigrationNotReady, modelType)
	}
	return model, nil
}

func (e *Migration) CreateVersion(tx *gorm.DB, version string) error {
	if tx == nil {
		return fmt.Errorf("%w: database is nil", ErrMigrationNotReady)
	}
	id, err := ParseMigrationID(version)
	if err != nil {
		return err
	}
	m, err := e.cloneModel()
	if err != nil {
		return err
	}
	m.SetVersion(id.String())
	return tx.Create(m).Error
}

type migrationSnapshot struct {
	db        *gorm.DB
	modelType reflect.Type
	versions  []registeredMigration
}

func (e *Migration) snapshot() (migrationSnapshot, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if err := e.validateRegistrationsLocked(); err != nil {
		return migrationSnapshot{}, err
	}
	if e.db == nil {
		return migrationSnapshot{}, fmt.Errorf("%w: database is nil", ErrMigrationNotReady)
	}
	modelType, err := versionModelType(e.Model)
	if err != nil {
		return migrationSnapshot{}, err
	}

	versions := make([]registeredMigration, 0, len(e.versions))
	for _, registered := range e.versions {
		registered.legacyIDs = append([]MigrationID(nil), registered.legacyIDs...)
		versions = append(versions, registered)
	}
	sort.Slice(versions, func(i, j int) bool {
		return migrationIDLess(versions[i].id, versions[j].id)
	})
	return migrationSnapshot{db: e.db, modelType: modelType, versions: versions}, nil
}

// MigrateContext applies every pending migration in deterministic numeric ID
// order. It returns on the first lookup or migration error and never terminates
// the process. Successful earlier migrations remain recorded, while the failed
// and later migrations remain pending.
func (e *Migration) MigrateContext(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("%w: migration runner is nil", ErrMigrationNotReady)
	}
	if ctx == nil {
		return fmt.Errorf("migration context is nil")
	}

	e.runMutex.Lock()
	defer e.runMutex.Unlock()

	snapshot, err := e.snapshot()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("migration canceled before execution: %w", err)
	}
	db := snapshot.db.WithContext(ctx)
	for _, registered := range snapshot.versions {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("migration %s canceled: %w", registered.id, err)
		}
		done, err := migrationDone(db, snapshot.modelType, registered)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", registered.id, err)
		}
		if done {
			continue
		}
		if err := registered.run(db, registered.id.String()); err != nil {
			return fmt.Errorf("migrate version %s: %w", registered.id, err)
		}
	}
	return nil
}

// Migrate preserves the v1.0 no-return signature for source compatibility.
// It is a deprecated best-effort bridge: returned errors are logged rather than
// handled with Exit, Fatal, or panic. Because this signature cannot communicate
// success, correctness-sensitive code must use MigrateContext and handle its
// returned error.
//
// Deprecated: use MigrateContext.
func (e *Migration) Migrate() {
	if err := e.MigrateContext(context.Background()); err != nil {
		slog.Error(
			"legacy migration failed; use MigrateContext to handle the error",
			"err",
			err,
		)
	}
}

func migrationDone(
	db *gorm.DB,
	modelType reflect.Type,
	registered registeredMigration,
) (bool, error) {
	ids := make([]MigrationID, 0, 1+len(registered.legacyIDs))
	ids = append(ids, registered.id)
	ids = append(ids, registered.legacyIDs...)
	for _, id := range ids {
		model, err := newVersion(modelType)
		if err != nil {
			return false, err
		}
		model.SetVersion(id.String())
		done, err := model.Done(db)
		if err != nil {
			return false, err
		}
		if done {
			return true, nil
		}
	}
	return false, nil
}

func sortMigrationIDs(ids []MigrationID) {
	sort.Slice(ids, func(i, j int) bool {
		return migrationIDLess(ids[i], ids[j])
	})
}

func migrationIDLess(left, right MigrationID) bool {
	if len(left) != len(right) {
		return len(left) < len(right)
	}
	return left < right
}

func GenFile(system bool, path string) error {
	t1, err := template.ParseFS(FS, "migrate.tpl")
	if err != nil {
		return fmt.Errorf("parse migration template: %w", err)
	}
	m := map[string]string{}
	m["GenerateTime"] = strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	m["Package"] = "custom"
	if system {
		m["Package"] = "system"
	}
	var b1 bytes.Buffer
	if err := t1.Execute(&b1, m); err != nil {
		return fmt.Errorf("execute migration template: %w", err)
	}
	if system {
		return fileCreate(b1, filepath.Join(path, "system", m["GenerateTime"]+"_migrate.go"))
	}
	return fileCreate(b1, filepath.Join(path, "custom", m["GenerateTime"]+"_migrate.go"))
}

func fileCreate(content bytes.Buffer, name string) error {
	if !pkg.PathExist(filepath.Dir(name)) {
		if err := pkg.PathCreate(filepath.Dir(name)); err != nil {
			return fmt.Errorf("create migration directory: %w", err)
		}
	}
	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("create migration file: %w", err)
	}
	if _, err := file.WriteString(content.String()); err != nil {
		_ = file.Close()
		return fmt.Errorf("write migration file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close migration file: %w", err)
	}
	return nil
}
