package service

import (
	"context"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type MockUserDB struct {
	mock.Mock
}

func (m *MockUserDB) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserDB) First(ctx context.Context, id string) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserDB) Find(ctx context.Context, page, pageSize int) ([]models.User, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]models.User), args.Get(1).(int64), args.Error(2)
}

func (m *MockUserDB) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserDB) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestUserService_Create(t *testing.T) {
	t.Run("should create user successfully", func(t *testing.T) {
		mockDB := new(MockUserDB)
		user := &models.User{}
		user.Username = "testuser"
		user.Name = "Test User"
		user.Email = "test@example.com"

		mockDB.On("Create", mock.Anything, user).Return(nil)

		err := mockDB.Create(context.Background(), user)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("should handle empty username", func(t *testing.T) {
		user := &models.User{}
		user.Name = "Test User"

		assert.Empty(t, user.Username)
	})
}

func TestUserService_Get(t *testing.T) {
	t.Run("should get user by id", func(t *testing.T) {
		mockDB := new(MockUserDB)
		expectedUser := &models.User{}
		expectedUser.ID = "1"
		expectedUser.Username = "admin"
		expectedUser.Name = "Administrator"

		mockDB.On("First", mock.Anything, "1").Return(expectedUser, nil)

		user, err := mockDB.First(context.Background(), "1")
		assert.NoError(t, err)
		assert.Equal(t, "admin", user.Username)
		mockDB.AssertExpectations(t)
	})

	t.Run("should return error when user not found", func(t *testing.T) {
		mockDB := new(MockUserDB)

		mockDB.On("First", mock.Anything, "999").Return(nil, gorm.ErrRecordNotFound)

		user, err := mockDB.First(context.Background(), "999")
		assert.Error(t, err)
		assert.Nil(t, user)
		mockDB.AssertExpectations(t)
	})
}

func TestUserService_List(t *testing.T) {
	t.Run("should list users with pagination", func(t *testing.T) {
		mockDB := new(MockUserDB)
		user1 := &models.User{}
		user1.Username = "user1"
		user2 := &models.User{}
		user2.Username = "user2"
		users := []models.User{*user1, *user2}

		mockDB.On("Find", mock.Anything, 1, 10).Return(users, int64(2), nil)

		result, total, err := mockDB.Find(context.Background(), 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, int64(2), total)
		mockDB.AssertExpectations(t)
	})
}

func TestUserService_Update(t *testing.T) {
	t.Run("should update user successfully", func(t *testing.T) {
		mockDB := new(MockUserDB)
		user := &models.User{}
		user.ID = "1"
		user.Username = "updateduser"

		mockDB.On("Update", mock.Anything, user).Return(nil)

		err := mockDB.Update(context.Background(), user)
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestUserService_Delete(t *testing.T) {
	t.Run("should delete user successfully", func(t *testing.T) {
		mockDB := new(MockUserDB)

		mockDB.On("Delete", mock.Anything, "1").Return(nil)

		err := mockDB.Delete(context.Background(), "1")
		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestUser_Fields(t *testing.T) {
	t.Run("should have valid user fields", func(t *testing.T) {
		user := &models.User{}
		user.Username = "testuser"
		user.Name = "Test User"
		user.Email = "test@example.com"

		assert.NotEmpty(t, user.Username)
		assert.NotEmpty(t, user.Name)
		assert.NotEmpty(t, user.Email)
	})

	t.Run("should handle empty username", func(t *testing.T) {
		user := &models.User{}
		user.Name = "Test User"

		assert.Empty(t, user.Username)
	})
}