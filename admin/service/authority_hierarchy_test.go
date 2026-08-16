package service

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestPrepareDepartmentWriteOwnsIdentityAndRejectsCycles(t *testing.T) {
	db := authorityHierarchyTestDB(t)
	root := authorityDepartment("dept-root", "")
	child := authorityDepartment("dept-child", root.ID)
	grandchild := authorityDepartment("dept-grandchild", child.ID)
	require.NoError(t, db.Create(root).Error)
	require.NoError(t, db.Create(child).Error)
	require.NoError(t, db.Create(grandchild).Error)

	createdAt := time.Now().Add(-time.Hour)
	candidate := &models.Department{
		ParentID: root.ID,
		Name:     "  Platform  ",
		Code:     "  PLATFORM  ",
		Status:   enum.Enabled,
	}
	candidate.ID = "client-owned"
	candidate.CreatedAt = createdAt
	candidate.Children = []*models.Department{authorityDepartment("injected-child", "")}
	require.NoError(t, PrepareDepartmentCreate(context.Background(), db, candidate))
	require.Empty(t, candidate.ID)
	require.True(t, candidate.CreatedAt.IsZero())
	require.Nil(t, candidate.Children)
	require.Equal(t, "Platform", candidate.Name)
	require.Equal(t, "PLATFORM", candidate.Code)

	unknownParent := &models.Department{
		ParentID: "missing",
		Name:     "Unknown parent",
		Code:     "UNKNOWN",
		Status:   enum.Enabled,
	}
	require.ErrorIs(
		t,
		PrepareDepartmentCreate(context.Background(), db, unknownParent),
		ErrAuthorityReferenceInvalid,
	)

	cycle := authorityDepartment("attacker-controlled", grandchild.ID)
	require.ErrorIs(
		t,
		PrepareDepartmentUpdate(context.Background(), db, root.ID, cycle),
		ErrAuthorityHierarchyCycle,
	)
	require.Equal(t, root.ID, cycle.ID)

	selfParent := authorityDepartment("attacker-controlled", child.ID)
	require.ErrorIs(
		t,
		PrepareDepartmentUpdate(context.Background(), db, child.ID, selfParent),
		ErrAuthorityHierarchyCycle,
	)
	require.Equal(t, child.ID, selfParent.ID)
}

func TestPreparePostWriteResolvesCustomDepartmentScope(t *testing.T) {
	db := authorityHierarchyTestDB(t)
	enabled := authorityDepartment("dept-enabled", "")
	disabled := authorityDepartment("dept-disabled", "")
	disabled.Status = enum.Disabled
	require.NoError(t, db.Create(enabled).Error)
	require.NoError(t, db.Create(disabled).Error)

	post := authorityPost("client-owned", "")
	post.DataScope = models.DataScopeCustomDept
	post.DeptIDSArr = []string{enabled.ID}
	require.NoError(t, PreparePostCreate(context.Background(), db, post))
	require.Empty(t, post.ID)
	require.Equal(t, []string{enabled.ID}, post.DeptIDSArr)
	require.Equal(t, enabled.ID, post.DeptIDS)

	unknown := authorityPost("unknown", "")
	unknown.DataScope = models.DataScopeCustomDept
	unknown.DeptIDSArr = []string{"missing"}
	require.ErrorIs(
		t,
		PreparePostCreate(context.Background(), db, unknown),
		ErrAuthorityReferenceInvalid,
	)

	inactive := authorityPost("inactive", "")
	inactive.DataScope = models.DataScopeCustomDept
	inactive.DeptIDSArr = []string{disabled.ID}
	require.ErrorIs(
		t,
		PreparePostCreate(context.Background(), db, inactive),
		ErrAuthorityReferenceInvalid,
	)

	duplicate := authorityPost("duplicate", "")
	duplicate.DataScope = models.DataScopeCustomDept
	duplicate.DeptIDSArr = []string{enabled.ID, enabled.ID}
	require.ErrorIs(
		t,
		PreparePostCreate(context.Background(), db, duplicate),
		ErrAuthorityPayloadInvalid,
	)

	nonCustom := authorityPost("non-custom", "")
	nonCustom.DataScope = models.DataScopeSelf
	nonCustom.DeptIDS = enabled.ID
	nonCustom.DeptIDSArr = []string{enabled.ID}
	require.NoError(t, PreparePostCreate(context.Background(), db, nonCustom))
	require.Empty(t, nonCustom.DeptIDS)
	require.Nil(t, nonCustom.DeptIDSArr)
}

func TestAuthorityDeleteRejectsLiveHierarchyUserAndScopeReferences(t *testing.T) {
	db := authorityHierarchyTestDB(t)
	rootDepartment := authorityDepartment("dept-root", "")
	childDepartment := authorityDepartment("dept-child", rootDepartment.ID)
	require.NoError(t, db.Create(rootDepartment).Error)
	require.NoError(t, db.Create(childDepartment).Error)

	require.ErrorIs(
		t,
		ValidateDepartmentDelete(context.Background(), db, []string{rootDepartment.ID}),
		ErrAuthorityRecordInUse,
	)
	require.NoError(
		t,
		ValidateDepartmentDelete(
			context.Background(),
			db,
			[]string{rootDepartment.ID, childDepartment.ID},
		),
	)

	user := authorityUser("dept-user")
	user.DepartmentID = childDepartment.ID
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).
		Omit(clause.Associations).
		Create(user).Error)
	require.ErrorIs(
		t,
		ValidateDepartmentDelete(
			context.Background(),
			db,
			[]string{rootDepartment.ID, childDepartment.ID},
		),
		ErrAuthorityRecordInUse,
	)
	require.NoError(t, db.Unscoped().Delete(user).Error)

	scopedPost := authorityPost("scope-post", "")
	scopedPost.DataScope = models.DataScopeCustomDept
	scopedPost.DeptIDSArr = []string{childDepartment.ID}
	scopedPost.DeptIDS = childDepartment.ID
	require.NoError(t, db.Create(scopedPost).Error)
	require.ErrorIs(
		t,
		ValidateDepartmentDelete(
			context.Background(),
			db,
			[]string{rootDepartment.ID, childDepartment.ID},
		),
		ErrAuthorityRecordInUse,
	)

	rootPost := authorityPost("post-root", "")
	childPost := authorityPost("post-child", rootPost.ID)
	require.NoError(t, db.Create(rootPost).Error)
	require.NoError(t, db.Create(childPost).Error)
	require.ErrorIs(
		t,
		ValidatePostDelete(context.Background(), db, []string{rootPost.ID}),
		ErrAuthorityRecordInUse,
	)

	postUser := authorityUser("post-user")
	postUser.PostID = childPost.ID
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).
		Omit(clause.Associations).
		Create(postUser).Error)
	require.ErrorIs(
		t,
		ValidatePostDelete(context.Background(), db, []string{rootPost.ID, childPost.ID}),
		ErrAuthorityRecordInUse,
	)
	require.ErrorIs(
		t,
		ValidatePostDelete(context.Background(), db, []string{"missing"}),
		gorm.ErrRecordNotFound,
	)
}

func authorityHierarchyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(
		t,
		db.AutoMigrate(
			&models.Department{},
			&models.Post{},
			&models.Role{},
			&models.User{},
		),
	)
	return db
}

func authorityDepartment(id, parentID string) *models.Department {
	department := &models.Department{
		ParentID: parentID,
		Name:     id,
		Code:     id,
		Status:   enum.Enabled,
	}
	department.ID = id
	return department
}

func authorityPost(id, parentID string) *models.Post {
	post := &models.Post{
		ParentID:  parentID,
		Name:      id,
		Code:      id,
		Status:    enum.Enabled,
		DataScope: models.DataScopeSelf,
	}
	post.ID = id
	return post
}

func authorityUser(id string) *models.User {
	user := &models.User{UserLogin: models.UserLogin{
		Username: id,
		Status:   enum.Enabled,
	}}
	user.ID = id
	return user
}
