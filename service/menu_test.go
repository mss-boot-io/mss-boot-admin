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

type MockMenuDB struct {
	mock.Mock
}

func (m *MockMenuDB) Create(ctx context.Context, menu *models.Menu) error {
	args := m.Called(ctx, menu)
	return args.Error(0)
}

func (m *MockMenuDB) First(ctx context.Context, id string) (*models.Menu, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Menu), args.Error(1)
}

func (m *MockMenuDB) Find(ctx context.Context, page, pageSize int) ([]models.Menu, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]models.Menu), args.Get(1).(int64), args.Error(2)
}

func (m *MockMenuDB) Update(ctx context.Context, menu *models.Menu) error {
	args := m.Called(ctx, menu)
	return args.Error(0)
}

func (m *MockMenuDB) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestMenuService_Create(t *testing.T) {
	t.Run("should create menu successfully", func(t *testing.T) {
		mockDB := new(MockMenuDB)
		menu := &models.Menu{
			Name:      "Dashboard",
			Path:      "/dashboard",
			Icon:      "dashboard",
			Component: "@/pages/Dashboard",
			Status:    enum.Enabled,
		}

		mockDB.On("Create", mock.Anything, menu).Return(nil)

		err := mockDB.Create(context.Background(), menu)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestMenuService_Get(t *testing.T) {
	t.Run("should get menu by id", func(t *testing.T) {
		mockDB := new(MockMenuDB)
		expectedMenu := &models.Menu{
			Name: "Dashboard",
			Path: "/dashboard",
		}
		expectedMenu.ID = "1"

		mockDB.On("First", mock.Anything, "1").Return(expectedMenu, nil)

		menu, err := mockDB.First(context.Background(), "1")
		assert.NoError(t, err)
		assert.Equal(t, "Dashboard", menu.Name)
		mockDB.AssertExpectations(t)
	})

	t.Run("should return error when menu not found", func(t *testing.T) {
		mockDB := new(MockMenuDB)

		mockDB.On("First", mock.Anything, "999").Return(nil, gorm.ErrRecordNotFound)

		menu, err := mockDB.First(context.Background(), "999")
		assert.Error(t, err)
		assert.Nil(t, menu)
		mockDB.AssertExpectations(t)
	})
}

func TestMenuService_List(t *testing.T) {
	t.Run("should list menus with pagination", func(t *testing.T) {
		mockDB := new(MockMenuDB)
		menus := []models.Menu{
			{Name: "Dashboard", Path: "/dashboard"},
			{Name: "Users", Path: "/users"},
		}

		mockDB.On("Find", mock.Anything, 1, 10).Return(menus, int64(2), nil)

		result, total, err := mockDB.Find(context.Background(), 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, int64(2), total)
		mockDB.AssertExpectations(t)
	})
}

func TestMenuService_Update(t *testing.T) {
	t.Run("should update menu successfully", func(t *testing.T) {
		mockDB := new(MockMenuDB)
		menu := &models.Menu{
			Name: "Updated Menu",
			Path: "/updated",
		}
		menu.ID = "1"

		mockDB.On("Update", mock.Anything, menu).Return(nil)

		err := mockDB.Update(context.Background(), menu)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestMenuService_Delete(t *testing.T) {
	t.Run("should delete menu successfully", func(t *testing.T) {
		mockDB := new(MockMenuDB)

		mockDB.On("Delete", mock.Anything, "1").Return(nil)

		err := mockDB.Delete(context.Background(), "1")
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestMenu_Tree(t *testing.T) {
	t.Run("should build menu tree correctly", func(t *testing.T) {
		root := &models.Menu{Name: "Parent", ParentID: "", Sort: 1}
		root.ID = "1"
		child1 := &models.Menu{Name: "Child1", ParentID: "1", Sort: 1}
		child1.ID = "2"
		child2 := &models.Menu{Name: "Child2", ParentID: "1", Sort: 2}
		child2.ID = "3"

		menus := models.MenuList{root, child1, child2}

		assert.Equal(t, 3, len(menus))
		assert.Equal(t, "", menus[0].ParentID)
		assert.Equal(t, "1", menus[1].ParentID)
	})
}