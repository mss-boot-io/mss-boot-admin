package service

import (
	"context"
	"errors"
	"net/mail"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

const (
	maxAuthorityIdentifierLength = 64
	maxAuthorityHierarchyWalk    = 2_000
	maxAuthorityDeleteBatch      = 100
	maxPostDepartmentEncoding    = 255
)

var (
	ErrAuthorityPayloadInvalid   = errors.New("authority payload is invalid")
	ErrAuthorityReferenceInvalid = errors.New("authority reference is invalid")
	ErrAuthorityHierarchyCycle   = errors.New("authority hierarchy contains a cycle")
	ErrAuthorityHierarchyCorrupt = errors.New("stored authority hierarchy is invalid")
	ErrAuthorityRecordInUse      = errors.New("authority record is in use")
)

type authorityHierarchyRow struct {
	ID       string      `gorm:"column:id"`
	ParentID string      `gorm:"column:parent_id"`
	Status   enum.Status `gorm:"column:status"`
}

func PrepareDepartmentCreate(ctx context.Context, db *gorm.DB, department *models.Department) error {
	if department == nil {
		return ErrAuthorityPayloadInvalid
	}
	department.ModelGormTenant = models.ModelGormTenant{}
	department.Children = nil
	return validateDepartmentWrite(ctx, db, department)
}

func PrepareDepartmentUpdate(
	ctx context.Context,
	db *gorm.DB,
	id string,
	department *models.Department,
) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	if department == nil {
		return ErrAuthorityPayloadInvalid
	}
	id, err := authorityIdentifier(id, false)
	if err != nil {
		return err
	}
	var current models.Department
	if err := db.WithContext(ctx).Select("id", "created_at", "updated_at", "deleted_at").
		First(&current, "id = ?", id).Error; err != nil {
		return err
	}
	department.ModelGormTenant = current.ModelGormTenant
	department.Children = nil
	return validateDepartmentWrite(ctx, db, department)
}

func PreparePostCreate(ctx context.Context, db *gorm.DB, post *models.Post) error {
	if post == nil {
		return ErrAuthorityPayloadInvalid
	}
	post.ModelGormTenant = models.ModelGormTenant{}
	post.Children = nil
	return validatePostWrite(ctx, db, post)
}

func PreparePostUpdate(ctx context.Context, db *gorm.DB, id string, post *models.Post) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	if post == nil {
		return ErrAuthorityPayloadInvalid
	}
	id, err := authorityIdentifier(id, false)
	if err != nil {
		return err
	}
	var current models.Post
	if err := db.WithContext(ctx).Select("id", "created_at", "updated_at", "deleted_at").
		First(&current, "id = ?", id).Error; err != nil {
		return err
	}
	post.ModelGormTenant = current.ModelGormTenant
	post.Children = nil
	return validatePostWrite(ctx, db, post)
}

func ValidateDepartmentDelete(ctx context.Context, db *gorm.DB, rawIDs []string) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	ids, err := authorityDeleteIDs(rawIDs)
	if err != nil {
		return err
	}
	query := db.WithContext(ctx)
	if err := ensureAuthorityRows(query, &models.Department{}, ids); err != nil {
		return err
	}
	if used, err := hasExternalChildren(query, &models.Department{}, ids); err != nil {
		return err
	} else if used {
		return ErrAuthorityRecordInUse
	}
	if used, err := hasAuthorityReference(query, &models.User{}, "department_id", ids); err != nil {
		return err
	} else if used {
		return ErrAuthorityRecordInUse
	}
	used, err := hasPostDepartmentScopeReference(query, ids)
	if err != nil {
		return err
	}
	if used {
		return ErrAuthorityRecordInUse
	}
	return nil
}

func ValidatePostDelete(ctx context.Context, db *gorm.DB, rawIDs []string) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	ids, err := authorityDeleteIDs(rawIDs)
	if err != nil {
		return err
	}
	query := db.WithContext(ctx)
	if err := ensureAuthorityRows(query, &models.Post{}, ids); err != nil {
		return err
	}
	if used, err := hasExternalChildren(query, &models.Post{}, ids); err != nil {
		return err
	} else if used {
		return ErrAuthorityRecordInUse
	}
	if used, err := hasAuthorityReference(query, &models.User{}, "post_id", ids); err != nil {
		return err
	} else if used {
		return ErrAuthorityRecordInUse
	}
	return nil
}

func validateDepartmentWrite(ctx context.Context, db *gorm.DB, department *models.Department) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	name, err := boundedAuthorityText(department.Name, 255, false)
	if err != nil {
		return err
	}
	code, err := boundedAuthorityText(department.Code, 255, false)
	if err != nil {
		return err
	}
	parentID, err := authorityIdentifier(department.ParentID, true)
	if err != nil {
		return err
	}
	leaderID, err := authorityIdentifier(department.LeaderID, true)
	if err != nil {
		return err
	}
	phone, err := boundedAuthorityText(department.Phone, 50, true)
	if err != nil {
		return err
	}
	email, err := boundedAuthorityText(department.Email, 255, true)
	if err != nil {
		return err
	}
	if email != "" {
		address, parseErr := mail.ParseAddress(email)
		if parseErr != nil || address.Address != email {
			return ErrAuthorityPayloadInvalid
		}
	}
	if department.Status == "" {
		department.Status = enum.Enabled
	}
	if department.Status != enum.Enabled && department.Status != enum.Disabled {
		return ErrAuthorityPayloadInvalid
	}
	if department.Sort < -1_000_000 || department.Sort > 1_000_000 {
		return ErrAuthorityPayloadInvalid
	}
	department.Name = name
	department.Code = code
	department.ParentID = parentID
	department.LeaderID = leaderID
	department.Phone = phone
	department.Email = email

	if err := validateAuthorityParent(ctx, db, &models.Department{}, department.ID, parentID); err != nil {
		return err
	}
	if leaderID != "" {
		var count int64
		if err := db.WithContext(ctx).Model(&models.User{}).
			Where("id = ? AND status = ?", leaderID, enum.Enabled).
			Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrAuthorityReferenceInvalid
		}
	}
	return nil
}

func validatePostWrite(ctx context.Context, db *gorm.DB, post *models.Post) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	name, err := boundedAuthorityText(post.Name, 255, false)
	if err != nil {
		return err
	}
	code, err := boundedAuthorityText(post.Code, 255, false)
	if err != nil {
		return err
	}
	parentID, err := authorityIdentifier(post.ParentID, true)
	if err != nil {
		return err
	}
	if post.Status == "" {
		post.Status = enum.Enabled
	}
	if post.Status != enum.Enabled && post.Status != enum.Disabled {
		return ErrAuthorityPayloadInvalid
	}
	if post.Sort < -1_000_000 || post.Sort > 1_000_000 {
		return ErrAuthorityPayloadInvalid
	}
	if post.DataScope == "" {
		post.DataScope = models.DataScopeSelf
	}
	switch post.DataScope {
	case models.DataScopeAll,
		models.DataScopeCurrentDept,
		models.DataScopeCurrentAndChildrenDept,
		models.DataScopeCustomDept,
		models.DataScopeSelf,
		models.DataScopeSelfAndChildren,
		models.DataScopeSelfAndAllChildren:
	default:
		return ErrAuthorityPayloadInvalid
	}
	post.Name = name
	post.Code = code
	post.ParentID = parentID

	if err := validateAuthorityParent(ctx, db, &models.Post{}, post.ID, parentID); err != nil {
		return err
	}
	if post.DataScope != models.DataScopeCustomDept {
		post.DeptIDS = ""
		post.DeptIDSArr = nil
		return nil
	}

	departmentIDs, err := normalizedAuthorityReferences(post.DeptIDSArr)
	if err != nil {
		return err
	}
	if len(departmentIDs) == 0 {
		return ErrAuthorityReferenceInvalid
	}
	encoded := strings.Join(departmentIDs, ",")
	if len(encoded) > maxPostDepartmentEncoding {
		return ErrAuthorityPayloadInvalid
	}
	var count int64
	if err := db.WithContext(ctx).Model(&models.Department{}).
		Where("id IN ? AND status = ?", departmentIDs, enum.Enabled).
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(departmentIDs)) {
		return ErrAuthorityReferenceInvalid
	}
	post.DeptIDSArr = departmentIDs
	post.DeptIDS = encoded
	return nil
}

func validateAuthorityParent(
	ctx context.Context,
	db *gorm.DB,
	model any,
	targetID string,
	parentID string,
) error {
	if parentID == "" {
		return nil
	}
	if targetID != "" && parentID == targetID {
		return ErrAuthorityHierarchyCycle
	}
	visited := make(map[string]struct{})
	current := parentID
	for step := 0; current != ""; step++ {
		if step >= maxAuthorityHierarchyWalk {
			return ErrAuthorityHierarchyCorrupt
		}
		if targetID != "" && current == targetID {
			return ErrAuthorityHierarchyCycle
		}
		if _, exists := visited[current]; exists {
			return ErrAuthorityHierarchyCorrupt
		}
		visited[current] = struct{}{}

		var row authorityHierarchyRow
		err := db.WithContext(ctx).Model(model).
			Select("id", "parent_id", "status").
			First(&row, "id = ?", current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAuthorityReferenceInvalid
		}
		if err != nil {
			return err
		}
		if row.Status != enum.Enabled {
			return ErrAuthorityReferenceInvalid
		}
		current = strings.TrimSpace(row.ParentID)
	}
	return nil
}

func authorityIdentifier(value string, optional bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && optional {
		return "", nil
	}
	if value == "" || len(value) > maxAuthorityIdentifierLength || strings.ContainsAny(value, ",\x00") {
		return "", ErrAuthorityPayloadInvalid
	}
	return value, nil
}

func boundedAuthorityText(value string, max int, optional bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && !optional {
		return "", ErrAuthorityPayloadInvalid
	}
	if utf8.RuneCountInString(value) > max {
		return "", ErrAuthorityPayloadInvalid
	}
	return value, nil
}

func normalizedAuthorityReferences(raw []string) ([]string, error) {
	if len(raw) > maxAuthorityDeleteBatch {
		return nil, ErrAuthorityPayloadInvalid
	}
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))
	for _, value := range raw {
		id, err := authorityIdentifier(value, false)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, ErrAuthorityPayloadInvalid
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func authorityDeleteIDs(raw []string) ([]string, error) {
	if len(raw) == 0 || len(raw) > maxAuthorityDeleteBatch {
		return nil, ErrAuthorityPayloadInvalid
	}
	return normalizedAuthorityReferences(raw)
}

func ensureAuthorityRows(db *gorm.DB, model any, ids []string) error {
	var count int64
	if err := db.Model(model).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func hasExternalChildren(db *gorm.DB, model any, ids []string) (bool, error) {
	var count int64
	err := db.Model(model).
		Where("parent_id IN ? AND id NOT IN ?", ids, ids).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

func hasAuthorityReference(db *gorm.DB, model any, column string, ids []string) (bool, error) {
	var count int64
	err := db.Model(model).Where(column+" IN ?", ids).Limit(1).Count(&count).Error
	return count > 0, err
}

func hasPostDepartmentScopeReference(db *gorm.DB, ids []string) (bool, error) {
	targets := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		targets[id] = struct{}{}
	}
	rows, err := db.Model(&models.Post{}).
		Select("dept_ids").
		Where("data_scope = ? AND dept_ids <> ?", models.DataScopeCustomDept, "").
		Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var encoded string
		if err := rows.Scan(&encoded); err != nil {
			return false, err
		}
		for _, id := range strings.Split(encoded, ",") {
			if _, exists := targets[strings.TrimSpace(id)]; exists {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}
