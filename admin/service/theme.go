package service

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

const (
	ThemeConfigGroup      = "theme"
	ThemeScopeApplication = "application"
	ThemeScopeUser        = "user"
	themeResourceVersion  = 1
	themeReadMaxAttempts  = 3
)

var (
	ErrInvalidThemePatch      = errors.New("invalid theme patch")
	ErrThemeGroupOnly         = errors.New("operation is supported only for the theme group")
	ErrThemeUserRequired      = errors.New("theme user id is required")
	ErrThemeRevisionConflict  = errors.New("theme revision conflict")
	ErrThemeKeyCollision      = errors.New("theme configuration key collision")
	ErrThemeGroupCaseMismatch = errors.New("theme group must use canonical lowercase casing")
)

// ThemeRevisionConflictError reports an optimistic-concurrency conflict and
// includes the current canonical resource so clients can reconcile without a
// second read.
type ThemeRevisionConflictError struct {
	Expected int64
	Actual   int64
	Current  *dto.ThemeResource
}

func (e *ThemeRevisionConflictError) Error() string {
	return fmt.Sprintf("%s: expected %d, current %d", ErrThemeRevisionConflict, e.Expected, e.Actual)
}

func (e *ThemeRevisionConflictError) Unwrap() error { return ErrThemeRevisionConflict }

// ThemeKeyCollisionError reports legacy rows that compare equal under a
// case-insensitive database collation but cannot be canonicalized
// unambiguously. It never includes configuration values.
type ThemeKeyCollisionError struct {
	Scope      string
	OwnerID    string
	Key        string
	Candidates int
}

func (e *ThemeKeyCollisionError) Error() string {
	if e.OwnerID == "" {
		return fmt.Sprintf("%s: scope=%s key=%s candidates=%d", ErrThemeKeyCollision, e.Scope, e.Key, e.Candidates)
	}
	return fmt.Sprintf(
		"%s: scope=%s owner=%s key=%s candidates=%d",
		ErrThemeKeyCollision,
		e.Scope,
		e.OwnerID,
		e.Key,
		e.Candidates,
	)
}

func (e *ThemeKeyCollisionError) Unwrap() error { return ErrThemeKeyCollision }

type themeValueKind uint8

const (
	themeString themeValueKind = iota + 1
	themeBoolean
)

type themeField struct {
	kind          themeValueKind
	allowedValues map[string]struct{}
}

var (
	themeColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	themeFields       = map[string]themeField{
		"navTheme": {
			kind:          themeString,
			allowedValues: stringSet("light", "realDark"),
		},
		"layout": {
			kind:          themeString,
			allowedValues: stringSet("side", "top", "mix"),
		},
		"contentWidth": {
			kind:          themeString,
			allowedValues: stringSet("Fluid", "Fixed"),
		},
		"fixedHeader": {kind: themeBoolean},
		"fixSiderbar": {kind: themeBoolean},
		"colorWeak":   {kind: themeBoolean},
		"colorPrimary": {
			kind: themeString,
		},
	}
	// legacyThemeWriteFields is a rolling-deployment compatibility window.
	// Old frontends submit pwa with the layout settings. The backend preserves
	// that value for old consumers, while all new ThemeResource projections and
	// resets remain limited to the canonical seven-field contract.
	legacyThemeWriteFields = map[string]themeField{
		"pwa": {kind: themeBoolean},
	}
	themeFieldNames = []string{
		"navTheme",
		"layout",
		"contentWidth",
		"fixedHeader",
		"fixSiderbar",
		"colorWeak",
		"colorPrimary",
	}
	legacyThemeFieldNames = append(append([]string(nil), themeFieldNames...), "pwa")
)

type themeOperation struct {
	name   string
	value  string
	delete bool
}

type themeRecord struct {
	name  string
	value string
}

type Theme struct{}

func (e *Theme) Application(ctx *gin.Context) (*dto.ThemeOverrides, error) {
	resource, err := e.ApplicationResource(ctx)
	if err != nil {
		return nil, err
	}
	result := resource.ThemeOverrides
	return &result, nil
}

func (e *Theme) ApplicationResource(ctx *gin.Context) (*dto.ThemeResource, error) {
	db := center.GetDB(ctx, &models.AppConfig{})
	key := applicationThemeRevisionKey()
	for attempt := 0; attempt < themeReadMaxAttempts; attempt++ {
		before, err := readConfigRevision(db, key)
		if err != nil {
			return nil, err
		}
		overrides, err := loadApplicationTheme(db)
		if err != nil {
			return nil, err
		}
		after, err := readConfigRevision(db, key)
		if err != nil {
			return nil, err
		}
		if before == after {
			return newThemeResource(ThemeScopeApplication, after, overrides), nil
		}
	}
	return nil, errors.New("application theme changed during read")
}

// LegacyApplicationSnapshot returns the canonical application-theme resource
// and its rolling-compatibility projection from one stable revision. The
// strong ETag must be derived from the returned resource; callers must not
// combine either value with a separately loaded pwa row.
func (e *Theme) LegacyApplicationSnapshot(
	ctx *gin.Context,
) (*dto.ThemeResource, map[string]any, error) {
	db := center.GetDB(ctx, &models.AppConfig{})
	key := applicationThemeRevisionKey()
	for attempt := 0; attempt < themeReadMaxAttempts; attempt++ {
		before, err := readConfigRevision(db, key)
		if err != nil {
			return nil, nil, err
		}
		overrides, pwa, err := loadLegacyApplicationTheme(db)
		if err != nil {
			return nil, nil, err
		}
		after, err := readConfigRevision(db, key)
		if err != nil {
			return nil, nil, err
		}
		if before == after {
			resource := newThemeResource(ThemeScopeApplication, after, overrides)
			return resource, legacyApplicationThemeProjection(overrides, pwa), nil
		}
	}
	return nil, nil, errors.New("application theme changed during read")
}

func loadApplicationTheme(db *gorm.DB) (*dto.ThemeOverrides, error) {
	var rows []models.AppConfig
	err := db.
		Where(&models.AppConfig{Group: ThemeConfigGroup}).
		Where("name IN ?", themeFieldNames).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	records := make([]themeRecord, 0, len(rows))
	for i := range rows {
		if rows[i].Group != ThemeConfigGroup || !isCanonicalThemeField(rows[i].Name) {
			continue
		}
		records = append(records, themeRecord{name: rows[i].Name, value: rows[i].Value})
	}
	return decodeThemeRecords(records)
}

func loadLegacyApplicationTheme(db *gorm.DB) (*dto.ThemeOverrides, *bool, error) {
	var rows []models.AppConfig
	err := db.
		Where(&models.AppConfig{Group: ThemeConfigGroup}).
		Where("name IN ?", legacyThemeFieldNames).
		Find(&rows).Error
	if err != nil {
		return nil, nil, err
	}
	records := make([]themeRecord, 0, len(rows))
	var pwa *bool
	for i := range rows {
		if rows[i].Group != ThemeConfigGroup {
			continue
		}
		if isCanonicalThemeField(rows[i].Name) {
			records = append(records, themeRecord{name: rows[i].Name, value: rows[i].Value})
			continue
		}
		if rows[i].Name != "pwa" {
			continue
		}
		switch rows[i].Value {
		case "true":
			value := true
			pwa = &value
		case "false":
			value := false
			pwa = &value
		default:
			slog.Warn("ignore invalid stored legacy pwa override")
		}
	}
	overrides, err := decodeThemeRecords(records)
	if err != nil {
		return nil, nil, err
	}
	return overrides, pwa, nil
}

func (e *Theme) User(ctx *gin.Context, userID string) (*dto.ThemeOverrides, error) {
	resource, err := e.UserResource(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := resource.ThemeOverrides
	return &result, nil
}

func (e *Theme) UserResource(ctx *gin.Context, userID string) (*dto.ThemeResource, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrThemeUserRequired
	}
	db := center.GetDB(ctx, &models.UserConfig{})
	key := userThemeRevisionKey(userID)
	for attempt := 0; attempt < themeReadMaxAttempts; attempt++ {
		before, err := readConfigRevision(db, key)
		if err != nil {
			return nil, err
		}
		overrides, err := loadUserTheme(db, userID)
		if err != nil {
			return nil, err
		}
		after, err := readConfigRevision(db, key)
		if err != nil {
			return nil, err
		}
		if before == after {
			return newThemeResource(ThemeScopeUser, after, overrides), nil
		}
	}
	return nil, errors.New("user theme changed during read")
}

func loadUserTheme(db *gorm.DB, userID string) (*dto.ThemeOverrides, error) {
	var rows []models.UserConfig
	err := db.
		Where(&models.UserConfig{UserID: userID, Group: ThemeConfigGroup}).
		Where("name IN ?", themeFieldNames).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	records := make([]themeRecord, 0, len(rows))
	for i := range rows {
		if rows[i].UserID != userID || rows[i].Group != ThemeConfigGroup || !isCanonicalThemeField(rows[i].Name) {
			continue
		}
		records = append(records, themeRecord{name: rows[i].Name, value: rows[i].Value})
	}
	return decodeThemeRecords(records)
}

func (e *Theme) PatchApplication(ctx *gin.Context, values map[string]any) error {
	_, err := e.PatchApplicationResource(ctx, values, nil)
	return err
}

func (e *Theme) PatchApplicationResource(
	ctx *gin.Context,
	values map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	resource, _, err := e.patchApplicationResource(ctx, values, expectedRevision, false)
	return resource, err
}

// PatchLegacyApplicationResource is a rolling-deployment compatibility path
// for application-theme clients that predate the canonical seven-field media
// type. It alone accepts the historical pwa boolean; pwa is persisted for old
// clients but is never projected into ThemeResource.
func (e *Theme) PatchLegacyApplicationResource(
	ctx *gin.Context,
	values map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	resource, _, err := e.patchApplicationResource(ctx, values, expectedRevision, true)
	return resource, err
}

// PatchLegacyApplicationSnapshot commits a legacy-compatible patch and forms
// both response representations inside that transaction. This prevents a
// successful commit from being reported as a 500 because of a later pwa read.
func (e *Theme) PatchLegacyApplicationSnapshot(
	ctx *gin.Context,
	values map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, map[string]any, error) {
	return e.patchApplicationResource(ctx, values, expectedRevision, true)
}

func (e *Theme) patchApplicationResource(
	ctx *gin.Context,
	values map[string]any,
	expectedRevision *int64,
	allowLegacyPWA bool,
) (*dto.ThemeResource, map[string]any, error) {
	operations, err := parseThemePatch(values, allowLegacyPWA)
	if err != nil {
		return nil, nil, err
	}
	db := center.GetDB(ctx, &models.AppConfig{})
	var resource *dto.ThemeResource
	var legacyProjection map[string]any
	var staleProfileRevision int64
	err = db.Transaction(func(tx *gorm.DB) error {
		profileRevision, err := lockConfigRevision(tx, applicationPublicProfileRevisionKey())
		if err != nil {
			return err
		}
		themeRevision, err := lockConfigRevision(tx, applicationThemeRevisionKey())
		if err != nil {
			return err
		}
		if expectedRevision != nil && *expectedRevision != themeRevision {
			current, loadErr := loadApplicationTheme(tx)
			if loadErr != nil {
				return loadErr
			}
			return &ThemeRevisionConflictError{
				Expected: *expectedRevision,
				Actual:   themeRevision,
				Current:  newThemeResource(ThemeScopeApplication, themeRevision, current),
			}
		}
		for _, operation := range operations {
			if operation.delete {
				if err := hardDeleteApplicationThemeValue(tx, operation.name); err != nil {
					return err
				}
				continue
			}
			if err := upsertApplicationThemeValue(tx, operation); err != nil {
				return err
			}
		}
		nextThemeRevision, err := advanceConfigRevision(tx, applicationThemeRevisionKey(), themeRevision)
		if err != nil {
			return err
		}
		if _, err = advanceConfigRevision(tx, applicationPublicProfileRevisionKey(), profileRevision); err != nil {
			return err
		}
		if allowLegacyPWA {
			overrides, pwa, loadErr := loadLegacyApplicationTheme(tx)
			if loadErr != nil {
				return loadErr
			}
			resource = newThemeResource(ThemeScopeApplication, nextThemeRevision, overrides)
			legacyProjection = legacyApplicationThemeProjection(overrides, pwa)
		} else {
			overrides, loadErr := loadApplicationTheme(tx)
			if loadErr != nil {
				return loadErr
			}
			resource = newThemeResource(ThemeScopeApplication, nextThemeRevision, overrides)
		}
		staleProfileRevision = profileRevision
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	invalidateLegacyApplicationThemeCache(ctx, operations)
	cleanupPublicProfileCache(ctx, staleProfileRevision)
	return resource, legacyProjection, nil
}

func (e *Theme) PatchUser(ctx *gin.Context, userID string, values map[string]any) error {
	_, err := e.PatchUserResource(ctx, userID, values, nil)
	return err
}

func (e *Theme) PatchUserResource(
	ctx *gin.Context,
	userID string,
	values map[string]any,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrThemeUserRequired
	}
	operations, err := parseThemePatch(values, false)
	if err != nil {
		return nil, err
	}
	db := center.GetDB(ctx, &models.UserConfig{})
	var resource *dto.ThemeResource
	err = db.Transaction(func(tx *gorm.DB) error {
		themeRevision, err := lockConfigRevision(tx, userThemeRevisionKey(userID))
		if err != nil {
			return err
		}
		if expectedRevision != nil && *expectedRevision != themeRevision {
			current, loadErr := loadUserTheme(tx, userID)
			if loadErr != nil {
				return loadErr
			}
			return &ThemeRevisionConflictError{
				Expected: *expectedRevision,
				Actual:   themeRevision,
				Current:  newThemeResource(ThemeScopeUser, themeRevision, current),
			}
		}
		for _, operation := range operations {
			if operation.delete {
				if err := hardDeleteUserThemeValue(tx, userID, operation.name); err != nil {
					return err
				}
				continue
			}
			if err := upsertUserThemeValue(tx, userID, operation); err != nil {
				return err
			}
		}
		nextRevision, err := advanceConfigRevision(tx, userThemeRevisionKey(userID), themeRevision)
		if err != nil {
			return err
		}
		overrides, err := loadUserTheme(tx, userID)
		if err != nil {
			return err
		}
		resource = newThemeResource(ThemeScopeUser, nextRevision, overrides)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func (e *Theme) ResetApplication(ctx *gin.Context) error {
	_, err := e.ResetApplicationResource(ctx, nil)
	return err
}

func (e *Theme) ResetApplicationResource(
	ctx *gin.Context,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	resource, _, err := e.resetApplicationResource(ctx, expectedRevision, false)
	return resource, err
}

// ResetLegacyApplicationSnapshot resets only the canonical seven overrides,
// preserving pwa for rolling compatibility, and forms the legacy response in
// the same transaction as the reset.
func (e *Theme) ResetLegacyApplicationSnapshot(
	ctx *gin.Context,
	expectedRevision *int64,
) (*dto.ThemeResource, map[string]any, error) {
	return e.resetApplicationResource(ctx, expectedRevision, true)
}

func (e *Theme) resetApplicationResource(
	ctx *gin.Context,
	expectedRevision *int64,
	includeLegacyProjection bool,
) (*dto.ThemeResource, map[string]any, error) {
	db := center.GetDB(ctx, &models.AppConfig{})
	var resource *dto.ThemeResource
	var legacyProjection map[string]any
	var staleProfileRevision int64
	err := db.Transaction(func(tx *gorm.DB) error {
		profileRevision, err := lockConfigRevision(tx, applicationPublicProfileRevisionKey())
		if err != nil {
			return err
		}
		themeRevision, err := lockConfigRevision(tx, applicationThemeRevisionKey())
		if err != nil {
			return err
		}
		if expectedRevision != nil && *expectedRevision != themeRevision {
			current, loadErr := loadApplicationTheme(tx)
			if loadErr != nil {
				return loadErr
			}
			return &ThemeRevisionConflictError{
				Expected: *expectedRevision,
				Actual:   themeRevision,
				Current:  newThemeResource(ThemeScopeApplication, themeRevision, current),
			}
		}
		if err := deleteApplicationThemeOverrides(tx); err != nil {
			return err
		}
		nextThemeRevision, err := advanceConfigRevision(tx, applicationThemeRevisionKey(), themeRevision)
		if err != nil {
			return err
		}
		if _, err = advanceConfigRevision(tx, applicationPublicProfileRevisionKey(), profileRevision); err != nil {
			return err
		}
		if includeLegacyProjection {
			overrides, pwa, loadErr := loadLegacyApplicationTheme(tx)
			if loadErr != nil {
				return loadErr
			}
			resource = newThemeResource(ThemeScopeApplication, nextThemeRevision, overrides)
			legacyProjection = legacyApplicationThemeProjection(overrides, pwa)
		} else {
			resource = newThemeResource(ThemeScopeApplication, nextThemeRevision, &dto.ThemeOverrides{})
		}
		staleProfileRevision = profileRevision
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	clearLegacyApplicationThemeCache(ctx)
	cleanupPublicProfileCache(ctx, staleProfileRevision)
	return resource, legacyProjection, nil
}

func (e *Theme) ResetUser(ctx *gin.Context, userID string) error {
	_, err := e.ResetUserResource(ctx, userID, nil)
	return err
}

func (e *Theme) ResetUserResource(
	ctx *gin.Context,
	userID string,
	expectedRevision *int64,
) (*dto.ThemeResource, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrThemeUserRequired
	}
	db := center.GetDB(ctx, &models.UserConfig{})
	var resource *dto.ThemeResource
	err := db.Transaction(func(tx *gorm.DB) error {
		themeRevision, err := lockConfigRevision(tx, userThemeRevisionKey(userID))
		if err != nil {
			return err
		}
		if expectedRevision != nil && *expectedRevision != themeRevision {
			current, loadErr := loadUserTheme(tx, userID)
			if loadErr != nil {
				return loadErr
			}
			return &ThemeRevisionConflictError{
				Expected: *expectedRevision,
				Actual:   themeRevision,
				Current:  newThemeResource(ThemeScopeUser, themeRevision, current),
			}
		}
		if err := deleteUserThemeOverrides(tx, userID); err != nil {
			return err
		}
		nextRevision, err := advanceConfigRevision(tx, userThemeRevisionKey(userID), themeRevision)
		if err != nil {
			return err
		}
		resource = newThemeResource(ThemeScopeUser, nextRevision, &dto.ThemeOverrides{})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func newThemeResource(scope string, revision int64, overrides *dto.ThemeOverrides) *dto.ThemeResource {
	resource := &dto.ThemeResource{
		Meta: dto.ThemeResourceMeta{
			Version:  themeResourceVersion,
			Scope:    scope,
			Revision: strconv.FormatInt(revision, 10),
		},
	}
	if overrides != nil {
		resource.ThemeOverrides = *overrides
	}
	return resource
}

func applicationThemeRevisionKey() configRevisionKey {
	return configRevisionKey{scope: ThemeScopeApplication, resource: configRevisionResourceTheme}
}

func applicationPublicProfileRevisionKey() configRevisionKey {
	return configRevisionKey{scope: ThemeScopeApplication, resource: configRevisionResourcePublicProfile}
}

func userThemeRevisionKey(userID string) configRevisionKey {
	return configRevisionKey{scope: ThemeScopeUser, ownerID: userID, resource: configRevisionResourceTheme}
}

func ThemeFieldNames() []string {
	return append([]string(nil), themeFieldNames...)
}

func rejectNonCanonicalThemeGroup(group string) error {
	if group != ThemeConfigGroup && canonicalConfigIdentifierFold(group) == ThemeConfigGroup {
		return fmt.Errorf("%w: got %q, want %q", ErrThemeGroupCaseMismatch, group, ThemeConfigGroup)
	}
	return nil
}

func canonicalConfigIdentifierFold(value string) string {
	value = strings.TrimRightFunc(value, unicode.IsSpace)
	decomposed := norm.NFKD.String(value)
	builder := strings.Builder{}
	builder.Grow(len(decomposed))
	for _, current := range decomposed {
		if unicode.Is(unicode.Mn, current) {
			continue
		}
		builder.WriteRune(unicode.ToLower(current))
	}
	return builder.String()
}

func parseThemePatch(values map[string]any, allowLegacyPWA bool) ([]themeOperation, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: data must contain at least one field", ErrInvalidThemePatch)
	}
	operations := make([]themeOperation, 0, len(values))
	for name, rawValue := range values {
		field, ok := themeWriteField(name, allowLegacyPWA)
		if !ok {
			return nil, fmt.Errorf("%w: unsupported field %q", ErrInvalidThemePatch, name)
		}
		operation := themeOperation{name: name}
		if rawValue == nil {
			operation.delete = true
			operations = append(operations, operation)
			continue
		}
		switch field.kind {
		case themeBoolean:
			value, ok := rawValue.(bool)
			if !ok {
				return nil, fmt.Errorf("%w: field %q must be a boolean or null", ErrInvalidThemePatch, name)
			}
			operation.value = fmt.Sprintf("%t", value)
		case themeString:
			value, ok := rawValue.(string)
			if !ok {
				return nil, fmt.Errorf("%w: field %q must be a string or null", ErrInvalidThemePatch, name)
			}
			if err := validateThemeString(name, value, field); err != nil {
				return nil, err
			}
			if name == "colorPrimary" {
				value = strings.ToLower(value)
			}
			operation.value = value
		default:
			return nil, fmt.Errorf("%w: unsupported field %q", ErrInvalidThemePatch, name)
		}
		operations = append(operations, operation)
	}
	sort.Slice(operations, func(i, j int) bool { return operations[i].name < operations[j].name })
	return operations, nil
}

func validateThemeString(name, value string, field themeField) error {
	if name == "colorPrimary" {
		if !themeColorPattern.MatchString(value) {
			return fmt.Errorf("%w: field %q must be a #RRGGBB color", ErrInvalidThemePatch, name)
		}
		return nil
	}
	if _, ok := field.allowedValues[value]; !ok {
		return fmt.Errorf("%w: unsupported value %q for field %q", ErrInvalidThemePatch, value, name)
	}
	return nil
}

func themeWriteField(name string, allowLegacyPWA bool) (themeField, bool) {
	if field, ok := themeFields[name]; ok {
		return field, true
	}
	if !allowLegacyPWA {
		return themeField{}, false
	}
	field, ok := legacyThemeWriteFields[name]
	return field, ok
}

func decodeThemeRecords(records []themeRecord) (*dto.ThemeOverrides, error) {
	result := &dto.ThemeOverrides{}
	for _, record := range records {
		field, ok := themeFields[record.name]
		if !ok {
			continue
		}
		switch field.kind {
		case themeBoolean:
			var value bool
			switch record.value {
			case "true":
				value = true
			case "false":
				value = false
			default:
				slog.Warn("ignore invalid stored theme override", "field", record.name)
				continue
			}
			setThemeBoolean(result, record.name, value)
		case themeString:
			if err := validateThemeString(record.name, record.value, field); err != nil {
				slog.Warn("ignore invalid stored theme override", "field", record.name)
				continue
			}
			value := record.value
			if record.name == "colorPrimary" {
				value = strings.ToLower(value)
			}
			setThemeString(result, record.name, value)
		}
	}
	return result, nil
}

func setThemeBoolean(result *dto.ThemeOverrides, name string, value bool) {
	copyOfValue := value
	switch name {
	case "fixedHeader":
		result.FixedHeader = &copyOfValue
	case "fixSiderbar":
		result.FixSiderbar = &copyOfValue
	case "colorWeak":
		result.ColorWeak = &copyOfValue
	}
}

func setThemeString(result *dto.ThemeOverrides, name, value string) {
	copyOfValue := value
	switch name {
	case "navTheme":
		result.NavTheme = &copyOfValue
	case "layout":
		result.Layout = &copyOfValue
	case "contentWidth":
		result.ContentWidth = &copyOfValue
	case "colorPrimary":
		result.ColorPrimary = &copyOfValue
	}
}

func themeOverridesMap(overrides *dto.ThemeOverrides) map[string]any {
	result := make(map[string]any)
	if overrides == nil {
		return result
	}
	if overrides.NavTheme != nil {
		result["navTheme"] = *overrides.NavTheme
	}
	if overrides.Layout != nil {
		result["layout"] = *overrides.Layout
	}
	if overrides.ContentWidth != nil {
		result["contentWidth"] = *overrides.ContentWidth
	}
	if overrides.FixedHeader != nil {
		result["fixedHeader"] = *overrides.FixedHeader
	}
	if overrides.FixSiderbar != nil {
		result["fixSiderbar"] = *overrides.FixSiderbar
	}
	if overrides.ColorWeak != nil {
		result["colorWeak"] = *overrides.ColorWeak
	}
	if overrides.ColorPrimary != nil {
		result["colorPrimary"] = *overrides.ColorPrimary
	}
	return result
}

func legacyApplicationThemeProjection(overrides *dto.ThemeOverrides, pwa *bool) map[string]any {
	result := themeOverridesMap(overrides)
	if pwa != nil {
		result["pwa"] = *pwa
	}
	return result
}

func themeResourceMap(resource *dto.ThemeResource) map[string]any {
	if resource == nil {
		return map[string]any{}
	}
	result := themeOverridesMap(&resource.ThemeOverrides)
	result["_meta"] = map[string]any{
		"v":        resource.Meta.Version,
		"scope":    resource.Meta.Scope,
		"revision": resource.Meta.Revision,
	}
	return result
}

func normalizeThemeProfileValue(name string, value any) (any, bool) {
	field, ok := themeFields[name]
	if !ok {
		return nil, false
	}
	switch field.kind {
	case themeBoolean:
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			if typed == "true" {
				return true, true
			}
			if typed == "false" {
				return false, true
			}
		}
	case themeString:
		typed, ok := value.(string)
		if ok && validateThemeString(name, typed, field) == nil {
			if name == "colorPrimary" {
				typed = strings.ToLower(typed)
			}
			return typed, true
		}
	}
	return nil, false
}

func isCanonicalThemeField(name string) bool {
	_, ok := themeFields[name]
	return ok
}

func hardDeleteApplicationThemeValue(tx *gorm.DB, name string) error {
	candidates, err := applicationThemeKeyCandidates(tx, name, false)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(candidates))
	for i := range candidates {
		if candidates[i].Group == ThemeConfigGroup && candidates[i].Name == name {
			ids = append(ids, candidates[i].ID)
		}
	}
	return hardDeleteApplicationConfigIDs(tx, ids)
}

func hardDeleteUserThemeValue(tx *gorm.DB, userID, name string) error {
	candidates, err := userThemeKeyCandidates(tx, userID, name, false)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(candidates))
	for i := range candidates {
		if candidates[i].UserID == userID && candidates[i].Group == ThemeConfigGroup && candidates[i].Name == name {
			ids = append(ids, candidates[i].ID)
		}
	}
	return hardDeleteUserConfigIDs(tx, ids)
}

func upsertApplicationThemeValue(tx *gorm.DB, operation themeOperation) error {
	candidates, err := applicationThemeKeyCandidates(tx, operation.name, true)
	if err != nil {
		return err
	}
	if len(candidates) > 1 {
		return &ThemeKeyCollisionError{
			Scope: ThemeScopeApplication, Key: operation.name, Candidates: len(candidates),
		}
	}
	now := time.Now()
	if len(candidates) == 1 {
		result := tx.Unscoped().Model(&models.AppConfig{}).
			Where("id = ?", candidates[0].ID).
			Updates(map[string]any{
				"name":       operation.name,
				"group":      ThemeConfigGroup,
				"auth":       false,
				"value":      operation.value,
				"updated_at": now,
				"deleted_at": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var count int64
			if err := tx.Unscoped().Model(&models.AppConfig{}).
				Where("id = ?", candidates[0].ID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("canonicalize application theme key %q: row disappeared", operation.name)
			}
		}
		return nil
	}

	record := &models.AppConfig{
		Group: ThemeConfigGroup,
		Name:  operation.name,
		Value: operation.value,
		Auth:  false,
	}
	record.UpdatedAt = now
	return tx.Create(record).Error
}

func upsertUserThemeValue(tx *gorm.DB, userID string, operation themeOperation) error {
	candidates, err := userThemeKeyCandidates(tx, userID, operation.name, true)
	if err != nil {
		return err
	}
	if len(candidates) > 1 || (len(candidates) == 1 && candidates[0].UserID != userID) {
		return &ThemeKeyCollisionError{
			Scope: ThemeScopeUser, OwnerID: userID, Key: operation.name, Candidates: len(candidates),
		}
	}
	now := time.Now()
	if len(candidates) == 1 {
		result := tx.Unscoped().Model(&models.UserConfig{}).
			Where("id = ?", candidates[0].ID).
			Updates(map[string]any{
				"user_id":    userID,
				"name":       operation.name,
				"group":      ThemeConfigGroup,
				"value":      operation.value,
				"updated_at": now,
				"deleted_at": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var count int64
			if err := tx.Unscoped().Model(&models.UserConfig{}).
				Where("id = ?", candidates[0].ID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("canonicalize user theme key %q: row disappeared", operation.name)
			}
		}
		return nil
	}

	record := &models.UserConfig{
		UserID: userID,
		Group:  ThemeConfigGroup,
		Name:   operation.name,
		Value:  operation.value,
	}
	record.UpdatedAt = now
	return tx.Create(record).Error
}

func applicationThemeKeyCandidates(
	db *gorm.DB,
	name string,
	includeDeleted bool,
) ([]models.AppConfig, error) {
	return appConfigCollationCandidates(db, ThemeConfigGroup, name, includeDeleted)
}

func appConfigCollationCandidates(
	db *gorm.DB,
	group string,
	name string,
	includeDeleted bool,
) ([]models.AppConfig, error) {
	newQuery := func() *gorm.DB {
		query := db.Session(&gorm.Session{NewDB: true})
		if includeDeleted {
			query = query.Unscoped()
		}
		return query
	}
	var rows []models.AppConfig
	// Keep the normal equality predicate first so deployed composite indexes
	// remain useful for every canonical write and for case/accent aliases under
	// *_ai_ci collations.
	if err := newQuery().Where(&models.AppConfig{Group: group, Name: name}).Find(&rows).Error; err != nil {
		return nil, err
	}

	// MySQL 8's utf8mb4_0900_ai_ci is accent insensitive but uses NO PAD, so a
	// historical trailing ASCII space is not returned by equality and can
	// coexist with the canonical key. The bounded repair query runs only on
	// administrative writes, after the indexed lookup, and lets the caller
	// canonicalize one row or diagnose multiple candidates.
	fallbackQuery := newQuery()
	quotedGroup := fallbackQuery.Statement.Quote("group")
	quotedName := fallbackQuery.Statement.Quote("name")
	var trailingAliases []models.AppConfig
	if err := fallbackQuery.Where(
		fmt.Sprintf("RTRIM(%s) = ? AND RTRIM(%s) = ?", quotedGroup, quotedName),
		group,
		name,
	).Find(&trailingAliases).Error; err != nil {
		return nil, err
	}
	return mergeAppConfigCandidates(rows, trailingAliases), nil
}

func userThemeKeyCandidates(
	db *gorm.DB,
	userID string,
	name string,
	includeDeleted bool,
) ([]models.UserConfig, error) {
	newQuery := func() *gorm.DB {
		query := db.Session(&gorm.Session{NewDB: true})
		if includeDeleted {
			query = query.Unscoped()
		}
		return query
	}
	var rows []models.UserConfig
	if err := newQuery().
		Where(&models.UserConfig{UserID: userID, Group: ThemeConfigGroup, Name: name}).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	fallbackQuery := newQuery()
	quotedGroup := fallbackQuery.Statement.Quote("group")
	quotedName := fallbackQuery.Statement.Quote("name")
	var trailingAliases []models.UserConfig
	if err := fallbackQuery.
		Where("user_id = ?", userID).
		Where(
			fmt.Sprintf("RTRIM(%s) = ? AND RTRIM(%s) = ?", quotedGroup, quotedName),
			ThemeConfigGroup,
			name,
		).
		Find(&trailingAliases).Error; err != nil {
		return nil, err
	}
	// Keep every database-equal group/name candidate so accent-insensitive and
	// trailing-space aliases can be repaired. The caller separately requires an
	// exact owner ID, preventing a case-insensitive user ID match from crossing
	// tenant/user ownership.
	return mergeUserConfigCandidates(rows, trailingAliases), nil
}

func mergeAppConfigCandidates(groups ...[]models.AppConfig) []models.AppConfig {
	result := make([]models.AppConfig, 0)
	seen := make(map[string]struct{})
	for _, rows := range groups {
		for i := range rows {
			if _, ok := seen[rows[i].ID]; ok {
				continue
			}
			seen[rows[i].ID] = struct{}{}
			result = append(result, rows[i])
		}
	}
	return result
}

func mergeUserConfigCandidates(groups ...[]models.UserConfig) []models.UserConfig {
	result := make([]models.UserConfig, 0)
	seen := make(map[string]struct{})
	for _, rows := range groups {
		for i := range rows {
			if _, ok := seen[rows[i].ID]; ok {
				continue
			}
			seen[rows[i].ID] = struct{}{}
			result = append(result, rows[i])
		}
	}
	return result
}

func deleteApplicationThemeOverrides(tx *gorm.DB) error {
	var rows []models.AppConfig
	if err := tx.
		Where(&models.AppConfig{Group: ThemeConfigGroup}).
		Where("name IN ?", themeFieldNames).
		Find(&rows).Error; err != nil {
		return err
	}
	ids := make([]string, 0, len(themeFieldNames))
	for i := range rows {
		if rows[i].Group == ThemeConfigGroup && isCanonicalThemeField(rows[i].Name) {
			ids = append(ids, rows[i].ID)
		}
	}
	return hardDeleteApplicationConfigIDs(tx, ids)
}

func deleteUserThemeOverrides(tx *gorm.DB, userID string) error {
	var rows []models.UserConfig
	if err := tx.
		Where(&models.UserConfig{UserID: userID, Group: ThemeConfigGroup}).
		Where("name IN ?", themeFieldNames).
		Find(&rows).Error; err != nil {
		return err
	}
	ids := make([]string, 0, len(themeFieldNames))
	for i := range rows {
		if rows[i].UserID == userID && rows[i].Group == ThemeConfigGroup && isCanonicalThemeField(rows[i].Name) {
			ids = append(ids, rows[i].ID)
		}
	}
	return hardDeleteUserConfigIDs(tx, ids)
}

func hardDeleteApplicationConfigIDs(tx *gorm.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Unscoped().Where("id IN ?", ids).Delete(&models.AppConfig{}).Error
}

func hardDeleteUserConfigIDs(tx *gorm.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Unscoped().Where("id IN ?", ids).Delete(&models.UserConfig{}).Error
}

func invalidateLegacyApplicationThemeCache(ctx *gin.Context, operations []themeOperation) {
	cache := center.GetCache()
	if cache == nil {
		return
	}
	fields := make([]string, 0, len(operations))
	for _, operation := range operations {
		fields = append(fields, fmt.Sprintf("%s:%s", ThemeConfigGroup, operation.name))
	}
	if len(fields) > 0 {
		if err := cache.HDel(ctx, legacyAppConfigCacheHash, fields...).Err(); err != nil {
			slog.Warn("invalidate legacy application theme cache after commit", "err", err)
		}
	}
}

func clearLegacyApplicationThemeCache(ctx *gin.Context) {
	cache := center.GetCache()
	if cache == nil {
		return
	}
	fields := make([]string, 0, len(themeFieldNames))
	for _, name := range themeFieldNames {
		fields = append(fields, fmt.Sprintf("%s:%s", ThemeConfigGroup, name))
	}
	if err := cache.HDel(ctx, legacyAppConfigCacheHash, fields...).Err(); err != nil {
		slog.Warn("clear legacy application theme cache after commit", "err", err)
	}
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
