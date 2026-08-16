package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const (
	maxNoticeDescriptionBytes = 16 * 1024
	maxTaskBodyBytes          = 64 * 1024
	maxTaskJSONBytes          = 16 * 1024
	maxTaskListEntries        = 32
	maxSystemConfigBytes      = 256 * 1024
)

var (
	ErrOperationalPayloadInvalid = errors.New("operational payload is invalid")
	ErrOperationalConflict       = errors.New("operational resource conflicts with current state")
	ErrOperationalBuiltIn        = errors.New("built-in operational resource is protected")
	ErrOperationalIdentity       = errors.New("operational identity is unavailable")

	taskScheduleParser = cron.NewParser(
		cron.Second |
			cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow |
			cron.Descriptor,
	)
	kubernetesScheduleParser = cron.NewParser(
		cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow |
			cron.Descriptor,
	)
	kubernetesNamePattern = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$`)
)

func PrepareTaskCreate(ctx context.Context, db *gorm.DB, definition *models.Task) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	if definition == nil {
		return ErrOperationalPayloadInvalid
	}
	definition.ModelGormTenant = models.ModelGormTenant{}
	definition.ModelCreator = models.ModelCreator{}
	definition.EntryID = 0
	definition.CheckedAt = sql.NullTime{}
	definition.CheckedAtR = nil
	definition.Once = false
	definition.Status = enum.Disabled
	return validateTaskDefinition(ctx, db, definition)
}

func PrepareTaskUpdate(ctx context.Context, db *gorm.DB, id string, definition *models.Task) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	if definition == nil {
		return ErrOperationalPayloadInvalid
	}
	id, err := authorityIdentifier(id, false)
	if err != nil {
		return ErrOperationalPayloadInvalid
	}
	var current models.Task
	if err := db.WithContext(ctx).
		Select(
			"id", "created_at", "updated_at", "deleted_at", "creator_id", "entry_id", "checked_at", "status",
			"provider", "cluster", "namespace",
		).
		First(&current, "id = ?", id).Error; err != nil {
		return err
	}
	currentProvider := current.Provider
	if currentProvider == "" {
		currentProvider = models.TaskProviderDefault
	}
	incomingProvider := definition.Provider
	if incomingProvider == "" {
		incomingProvider = currentProvider
	}
	if incomingProvider != currentProvider {
		return ErrOperationalConflict
	}
	definition.Provider = currentProvider
	if currentProvider == models.TaskProviderK8S {
		currentCluster := strings.TrimSpace(current.Cluster)
		currentNamespace := strings.TrimSpace(current.Namespace)
		if currentNamespace == "" {
			currentNamespace = "default"
		}
		incomingNamespace := strings.TrimSpace(definition.Namespace)
		if incomingNamespace == "" {
			incomingNamespace = "default"
		}
		if strings.TrimSpace(definition.Cluster) != currentCluster ||
			incomingNamespace != currentNamespace {
			return ErrOperationalConflict
		}
		definition.Cluster = currentCluster
		definition.Namespace = currentNamespace
	}
	definition.ModelGormTenant = current.ModelGormTenant
	definition.ModelCreator = current.ModelCreator
	definition.EntryID = current.EntryID
	definition.CheckedAt = current.CheckedAt
	definition.CheckedAtR = current.CheckedAtR
	definition.Once = false
	definition.Status = current.Status
	return validateTaskDefinition(ctx, db, definition)
}

func ValidateTaskDelete(ctx context.Context, db *gorm.DB, rawIDs []string) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	ids, err := authorityDeleteIDs(rawIDs)
	if err != nil {
		return ErrOperationalPayloadInvalid
	}
	var definitions []models.Task
	if err := db.WithContext(ctx).
		Select("id", "status").
		Where("id IN ?", ids).
		Find(&definitions).Error; err != nil {
		return err
	}
	if len(definitions) != len(ids) {
		return gorm.ErrRecordNotFound
	}
	for i := range definitions {
		if definitions[i].Status != enum.Disabled {
			return ErrOperationalConflict
		}
	}
	return nil
}

func validateTaskDefinition(_ context.Context, _ *gorm.DB, definition *models.Task) error {
	name, err := operationalText(definition.Name, 255, false)
	if err != nil {
		return err
	}
	spec, err := operationalText(definition.Spec, 255, false)
	if err != nil {
		return err
	}
	remark, err := operationalText(definition.Remark, 4_096, true)
	if err != nil {
		return err
	}
	if definition.Timeout == 0 {
		definition.Timeout = 30
	}
	if definition.Timeout < 1 || definition.Timeout > 3_600 {
		return ErrOperationalPayloadInvalid
	}
	if definition.Status != enum.Enabled && definition.Status != enum.Disabled {
		return ErrOperationalPayloadInvalid
	}
	if definition.Provider == "" {
		definition.Provider = models.TaskProviderDefault
	}
	scheduleParser := taskScheduleParser
	if definition.Provider == models.TaskProviderK8S {
		scheduleParser = kubernetesScheduleParser
	}
	if _, err := scheduleParser.Parse(spec); err != nil {
		return ErrOperationalPayloadInvalid
	}
	definition.Name = name
	definition.Spec = spec
	definition.Remark = remark

	switch definition.Provider {
	case models.TaskProviderDefault:
		return validateHTTPTask(definition)
	case models.TaskProviderFunc:
		return validateRegisteredTask(definition)
	case models.TaskProviderK8S:
		return validateKubernetesTask(definition)
	default:
		return ErrOperationalPayloadInvalid
	}
}

func validateHTTPTask(definition *models.Task) error {
	protocol := strings.ToLower(strings.TrimSpace(definition.Protocol))
	if protocol != "http" && protocol != "https" {
		return ErrOperationalPayloadInvalid
	}
	endpoint, err := operationalText(definition.Endpoint, 255, false)
	if err != nil || strings.Contains(endpoint, "://") || strings.ContainsAny(endpoint, "\r\n") {
		return ErrOperationalPayloadInvalid
	}
	parsed, err := url.Parse(protocol + "://" + endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return ErrOperationalPayloadInvalid
	}
	method := strings.ToUpper(strings.TrimSpace(definition.Method))
	switch method {
	case "GET", "POST", "PUT", "DELETE":
	default:
		return ErrOperationalPayloadInvalid
	}
	if len(definition.Body) > maxTaskBodyBytes || strings.ContainsAny(definition.Body, "\x00") {
		return ErrOperationalPayloadInvalid
	}
	metadata, err := normalizeTaskMetadata(definition.Metadata)
	if err != nil {
		return err
	}
	definition.Protocol = protocol
	definition.Endpoint = endpoint
	definition.Method = method
	definition.Metadata = metadata
	definition.Cluster = ""
	definition.Namespace = ""
	definition.Image = ""
	definition.Command = "[]"
	definition.Args = "[]"
	definition.Python = ""
	return nil
}

func validateRegisteredTask(definition *models.Task) error {
	method, err := operationalText(definition.Method, 128, false)
	if err != nil {
		return err
	}
	if _, exists := models.TaskFuncMap[method]; !exists {
		return ErrOperationalPayloadInvalid
	}
	args, err := normalizeTaskStringArray(definition.Args, false)
	if err != nil {
		return err
	}
	definition.Method = method
	definition.Args = args
	definition.Protocol = ""
	definition.Endpoint = ""
	definition.Body = ""
	definition.Metadata = ""
	definition.Cluster = ""
	definition.Namespace = ""
	definition.Image = ""
	definition.Command = "[]"
	definition.Python = ""
	return nil
}

func validateKubernetesTask(definition *models.Task) error {
	cluster, err := operationalText(definition.Cluster, 50, false)
	if err != nil {
		return err
	}
	namespace, err := operationalText(definition.Namespace, 63, true)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = "default"
	}
	if !kubernetesNamePattern.MatchString(namespace) {
		return ErrOperationalPayloadInvalid
	}
	image, err := operationalText(definition.Image, 255, false)
	if err != nil {
		return err
	}
	command, err := normalizeTaskStringArray(definition.Command, false)
	if err != nil {
		return err
	}
	args, err := normalizeTaskStringArray(definition.Args, false)
	if err != nil {
		return err
	}
	definition.Cluster = cluster
	definition.Namespace = namespace
	definition.Image = image
	definition.Command = command
	definition.Args = args
	definition.Protocol = ""
	definition.Endpoint = ""
	definition.Method = ""
	definition.Body = ""
	definition.Metadata = ""
	definition.Python = ""
	return nil
}

func normalizeTaskStringArray(raw models.JsonRawMessage, required bool) (models.JsonRawMessage, error) {
	encoded := strings.TrimSpace(string(raw))
	if encoded == "" {
		if required {
			return "", ErrOperationalPayloadInvalid
		}
		return "[]", nil
	}
	if len(encoded) > maxTaskJSONBytes {
		return "", ErrOperationalPayloadInvalid
	}
	var values []string
	if err := json.Unmarshal([]byte(encoded), &values); err != nil || values == nil ||
		len(values) > maxTaskListEntries || (required && len(values) == 0) {
		return "", ErrOperationalPayloadInvalid
	}
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
		if values[index] == "" || utf8.RuneCountInString(values[index]) > 1_024 ||
			strings.ContainsRune(values[index], '\x00') {
			return "", ErrOperationalPayloadInvalid
		}
	}
	canonical, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return models.JsonRawMessage(canonical), nil
}

func normalizeTaskMetadata(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > maxTaskJSONBytes {
		return "", ErrOperationalPayloadInvalid
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil || len(values) > 64 {
		return "", ErrOperationalPayloadInvalid
	}
	for key, value := range values {
		if _, err := operationalText(key, 128, false); err != nil ||
			utf8.RuneCountInString(value) > 4_096 || strings.ContainsAny(key+value, "\r\n\x00") {
			return "", ErrOperationalPayloadInvalid
		}
	}
	canonical, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(canonical), nil
}

func PrepareNoticeCreate(_ context.Context, _ *gorm.DB, userID string, notice *models.Notice) error {
	userID, err := authorityIdentifier(userID, false)
	if err != nil || notice == nil {
		return ErrOperationalPayloadInvalid
	}
	notice.ModelGormTenant = models.ModelGormTenant{}
	notice.UserID = userID
	notice.Read = false
	return validateNotice(notice)
}

func PrepareNoticeUpdate(
	ctx context.Context,
	db *gorm.DB,
	userID string,
	id string,
	notice *models.Notice,
) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	userID, err := authorityIdentifier(userID, false)
	if err != nil || notice == nil {
		return ErrOperationalPayloadInvalid
	}
	id, err = authorityIdentifier(id, false)
	if err != nil {
		return ErrOperationalPayloadInvalid
	}
	var current models.Notice
	if err := db.WithContext(ctx).
		Select("id", "created_at", "updated_at", "deleted_at", "user_id", "read").
		Where("user_id = ?", userID).
		First(&current, "id = ?", id).Error; err != nil {
		return err
	}
	notice.ModelGormTenant = current.ModelGormTenant
	notice.UserID = current.UserID
	notice.Read = current.Read
	return validateNotice(notice)
}

func validateNotice(notice *models.Notice) error {
	title, err := operationalText(notice.Title, 255, false)
	if err != nil {
		return err
	}
	key, err := operationalText(notice.Key, 255, true)
	if err != nil {
		return err
	}
	avatar, err := operationalText(notice.Avatar, 255, true)
	if err != nil {
		return err
	}
	extra, err := operationalText(notice.Extra, 255, true)
	if err != nil {
		return err
	}
	status, err := operationalText(notice.Status, 10, true)
	if err != nil {
		return err
	}
	switch status {
	case "", "urgent", "doing", "processing", "todo":
	default:
		return ErrOperationalPayloadInvalid
	}
	if len(notice.Description) > maxNoticeDescriptionBytes ||
		strings.ContainsRune(notice.Description, '\x00') {
		return ErrOperationalPayloadInvalid
	}
	if notice.Type == "" {
		notice.Type = models.NoticeTypeNotification
	}
	switch notice.Type {
	case models.NoticeTypeNotification,
		models.NoticeTypeMessage,
		models.NoticeTypeEvent,
		models.NoticeTypeMail:
	default:
		return ErrOperationalPayloadInvalid
	}
	notice.Title = title
	notice.Key = key
	notice.Avatar = avatar
	notice.Extra = extra
	notice.Status = status
	return nil
}

func PrepareSystemConfigCreate(ctx context.Context, db *gorm.DB, config *models.SystemConfig) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	if config == nil {
		return ErrOperationalPayloadInvalid
	}
	config.ModelGorm = models.SystemConfig{}.ModelGorm
	config.BuiltIn = false
	if err := validateSystemConfig(config); err != nil {
		return err
	}
	return ensureSystemConfigNameAvailable(ctx, db, config.Name, "")
}

func PrepareSystemConfigUpdate(
	ctx context.Context,
	db *gorm.DB,
	id string,
	config *models.SystemConfig,
) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	if config == nil {
		return ErrOperationalPayloadInvalid
	}
	id, err := authorityIdentifier(id, false)
	if err != nil {
		return ErrOperationalPayloadInvalid
	}
	var current models.SystemConfig
	if err := db.WithContext(ctx).First(&current, "id = ?", id).Error; err != nil {
		return err
	}
	config.ModelGorm = current.ModelGorm
	config.BuiltIn = current.BuiltIn
	if current.BuiltIn {
		config.Name = current.Name
		config.Ext = current.Ext
	}
	if err := validateSystemConfig(config); err != nil {
		return err
	}
	return ensureSystemConfigNameAvailable(ctx, db, config.Name, current.ID)
}

func ValidateSystemConfigDelete(ctx context.Context, db *gorm.DB, rawIDs []string) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	ids, err := authorityDeleteIDs(rawIDs)
	if err != nil {
		return ErrOperationalPayloadInvalid
	}
	var configs []models.SystemConfig
	if err := db.WithContext(ctx).
		Select("id", "built_in").
		Where("id IN ?", ids).
		Find(&configs).Error; err != nil {
		return err
	}
	if len(configs) != len(ids) {
		return gorm.ErrRecordNotFound
	}
	for i := range configs {
		if configs[i].BuiltIn {
			return ErrOperationalBuiltIn
		}
	}
	return nil
}

func validateSystemConfig(config *models.SystemConfig) error {
	name, err := operationalText(config.Name, 128, false)
	if err != nil || strings.ContainsAny(name, "/\\\x00") {
		return ErrOperationalPayloadInvalid
	}
	remark, err := operationalText(config.Remark, 255, true)
	if err != nil || len(config.Content) > maxSystemConfigBytes ||
		strings.ContainsRune(config.Content, '\x00') {
		return ErrOperationalPayloadInvalid
	}
	switch config.Ext {
	case source.SchemeJSOM:
		if strings.TrimSpace(config.Content) != "" {
			var document any
			if err := json.Unmarshal([]byte(config.Content), &document); err != nil {
				return ErrOperationalPayloadInvalid
			}
		}
	case source.SchemeYaml, source.SchemeYml:
		if strings.TrimSpace(config.Content) != "" {
			var document yaml.Node
			if err := yaml.Unmarshal([]byte(config.Content), &document); err != nil {
				return ErrOperationalPayloadInvalid
			}
		}
	default:
		return ErrOperationalPayloadInvalid
	}
	config.Name = name
	config.Remark = remark
	return nil
}

func ensureSystemConfigNameAvailable(ctx context.Context, db *gorm.DB, name, exceptID string) error {
	query := db.WithContext(ctx).Model(&models.SystemConfig{}).Where("name = ?", name)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrOperationalConflict
	}
	return nil
}

func operationalText(value string, max int, optional bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && !optional {
		return "", ErrOperationalPayloadInvalid
	}
	if utf8.RuneCountInString(value) > max || strings.ContainsRune(value, '\x00') {
		return "", ErrOperationalPayloadInvalid
	}
	return value, nil
}
