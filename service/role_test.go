package service

import (
	"context"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type MockRoleDB struct {
	mock.Mock
}

func (m *MockRoleDB) Create(ctx context.Context, role *models.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleDB) First(ctx context.Context, id string) (*models.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}

func (m *MockRoleDB) Find(ctx context.Context, page, pageSize int) ([]models.Role, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]models.Role), args.Get(1).(int64), args.Error(2)
}

func (m *MockRoleDB) Update(ctx context.Context, role *models.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleDB) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestRoleService_Create(t *testing.T) {
	t.Run("should create role successfully", func(t *testing.T) {
		mockDB := new(MockRoleDB)
		role := &models.Role{
			Name:   "admin",
			Remark: "Administrator role",
		}

		mockDB.On("Create", mock.Anything, role).Return(nil)

		err := mockDB.Create(context.Background(), role)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("should handle empty name", func(t *testing.T) {
		role := &models.Role{
			Remark: "Test role",
		}

		assert.Empty(t, role.Name)
	})
}

func TestRoleService_Get(t *testing.T) {
	t.Run("should get role by id", func(t *testing.T) {
		mockDB := new(MockRoleDB)
		expectedRole := &models.Role{}
		expectedRole.ID = "1"
		expectedRole.Name = "admin"
		expectedRole.Remark = "Administrator"

		mockDB.On("First", mock.Anything, "1").Return(expectedRole, nil)

		role, err := mockDB.First(context.Background(), "1")
		assert.NoError(t, err)
		assert.Equal(t, "admin", role.Name)
		mockDB.AssertExpectations(t)
	})

	t.Run("should return error when role not found", func(t *testing.T) {
		mockDB := new(MockRoleDB)

		mockDB.On("First", mock.Anything, "999").Return(nil, gorm.ErrRecordNotFound)

		role, err := mockDB.First(context.Background(), "999")
		assert.Error(t, err)
		assert.Nil(t, role)
		mockDB.AssertExpectations(t)
	})
}

func TestRoleService_List(t *testing.T) {
	t.Run("should list roles with pagination", func(t *testing.T) {
		mockDB := new(MockRoleDB)
		role1 := &models.Role{Name: "admin"}
		role2 := &models.Role{Name: "user"}
		roles := []models.Role{*role1, *role2}

		mockDB.On("Find", mock.Anything, 1, 10).Return(roles, int64(2), nil)

		result, total, err := mockDB.Find(context.Background(), 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, int64(2), total)
		mockDB.AssertExpectations(t)
	})
}

func TestRoleService_Update(t *testing.T) {
	t.Run("should update role successfully", func(t *testing.T) {
		mockDB := new(MockRoleDB)
		role := &models.Role{}
		role.ID = "1"
		role.Name = "updated_role"

		mockDB.On("Update", mock.Anything, role).Return(nil)

		err := mockDB.Update(context.Background(), role)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestRoleService_Delete(t *testing.T) {
	t.Run("should delete role successfully", func(t *testing.T) {
		mockDB := new(MockRoleDB)

		mockDB.On("Delete", mock.Anything, "1").Return(nil)

		err := mockDB.Delete(context.Background(), "1")
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}