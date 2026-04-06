package service

import (
	"context"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type MockDepartmentDB struct {
	mock.Mock
}

func (m *MockDepartmentDB) Create(ctx context.Context, dept *models.Department) error {
	args := m.Called(ctx, dept)
	return args.Error(0)
}

func (m *MockDepartmentDB) First(ctx context.Context, id string) (*models.Department, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Department), args.Error(1)
}

func (m *MockDepartmentDB) Find(ctx context.Context, page, pageSize int) ([]models.Department, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]models.Department), args.Get(1).(int64), args.Error(2)
}

func (m *MockDepartmentDB) Update(ctx context.Context, dept *models.Department) error {
	args := m.Called(ctx, dept)
	return args.Error(0)
}

func (m *MockDepartmentDB) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestDepartmentService_Create(t *testing.T) {
	t.Run("should create department successfully", func(t *testing.T) {
		mockDB := new(MockDepartmentDB)
		dept := &models.Department{
			Name:   "Engineering",
			Code:   "ENG",
			Status: enum.Enabled,
		}

		mockDB.On("Create", mock.Anything, dept).Return(nil)

		err := mockDB.Create(context.Background(), dept)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestDepartmentService_Get(t *testing.T) {
	t.Run("should get department by id", func(t *testing.T) {
		mockDB := new(MockDepartmentDB)
		expectedDept := &models.Department{
			Name:   "Engineering",
			Code:   "ENG",
			Status: enum.Enabled,
		}
		expectedDept.ID = "1"

		mockDB.On("First", mock.Anything, "1").Return(expectedDept, nil)

		dept, err := mockDB.First(context.Background(), "1")
		assert.NoError(t, err)
		assert.Equal(t, "Engineering", dept.Name)
		mockDB.AssertExpectations(t)
	})

	t.Run("should return error when department not found", func(t *testing.T) {
		mockDB := new(MockDepartmentDB)

		mockDB.On("First", mock.Anything, "999").Return(nil, gorm.ErrRecordNotFound)

		dept, err := mockDB.First(context.Background(), "999")
		assert.Error(t, err)
		assert.Nil(t, dept)
		mockDB.AssertExpectations(t)
	})
}

func TestDepartmentService_List(t *testing.T) {
	t.Run("should list departments with pagination", func(t *testing.T) {
		mockDB := new(MockDepartmentDB)
		depts := []models.Department{
			{Name: "Engineering", Code: "ENG"},
			{Name: "Marketing", Code: "MKT"},
		}

		mockDB.On("Find", mock.Anything, 1, 10).Return(depts, int64(2), nil)

		result, total, err := mockDB.Find(context.Background(), 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, int64(2), total)
		mockDB.AssertExpectations(t)
	})
}

func TestDepartmentService_Update(t *testing.T) {
	t.Run("should update department successfully", func(t *testing.T) {
		mockDB := new(MockDepartmentDB)
		dept := &models.Department{
			Name: "Updated Department",
			Code: "UPD",
		}
		dept.ID = "1"

		mockDB.On("Update", mock.Anything, dept).Return(nil)

		err := mockDB.Update(context.Background(), dept)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestDepartmentService_Delete(t *testing.T) {
	t.Run("should delete department successfully", func(t *testing.T) {
		mockDB := new(MockDepartmentDB)

		mockDB.On("Delete", mock.Anything, "1").Return(nil)

		err := mockDB.Delete(context.Background(), "1")
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestDepartment_Tree(t *testing.T) {
	t.Run("should build department tree correctly", func(t *testing.T) {
		root := &models.Department{Name: "Root", ParentID: "", Code: "ROOT"}
		root.ID = "1"
		child1 := &models.Department{Name: "Child1", ParentID: "1", Code: "CH1"}
		child1.ID = "2"
		child2 := &models.Department{Name: "Child2", ParentID: "1", Code: "CH2"}
		child2.ID = "3"

		depts := models.DepartmentList{root, child1, child2}

		assert.Equal(t, 3, len(depts))
		assert.Equal(t, "", depts[0].ParentID)
		assert.Equal(t, "1", depts[1].ParentID)
	})
}