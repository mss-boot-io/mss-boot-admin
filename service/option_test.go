package service

import (
	"encoding/json"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOptionTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.Option{}, &models.OptionVersion{})
	assert.NoError(t, err)

	return db
}

func TestOptionModel_Fields(t *testing.T) {
	option := models.Option{
		Category:    "system",
		Name:        "status",
		DisplayName: "Status Options",
		Description: "Basic status options",
		Remark:      "System options",
		Status:      enum.Enabled,
		Version:     1,
		BuiltIn:     true,
	}

	assert.Equal(t, "system", option.Category)
	assert.Equal(t, "status", option.Name)
	assert.Equal(t, "Status Options", option.DisplayName)
	assert.Equal(t, "Basic status options", option.Description)
	assert.True(t, option.BuiltIn)
	assert.Equal(t, 1, option.Version)
}

func TestOptionVersionModel(t *testing.T) {
	version := models.OptionVersion{
		OptionID:   "opt-123",
		Version:    1,
		ChangedBy:  "user-456",
		ChangeNote: "Initial version",
		Status:     enum.Enabled,
	}

	assert.Equal(t, "opt-123", version.OptionID)
	assert.Equal(t, 1, version.Version)
	assert.Equal(t, "user-456", version.ChangedBy)
	assert.Equal(t, "Initial version", version.ChangeNote)
}

func TestOptionItem_JSONSerialization(t *testing.T) {
	items := &models.OptionItems{
		{ID: "1", Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1, Icon: "check", Extra: map[string]any{"description": "Active status"}},
		{ID: "2", Key: "disabled", Label: "Disabled", Value: "disabled", Color: "red", Sort: 2},
	}

	data, err := json.Marshal(items)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "enabled")
	assert.Contains(t, string(data), "check")
	assert.Contains(t, string(data), "Active status")

	var decodedItems models.OptionItems
	err = json.Unmarshal(data, &decodedItems)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(decodedItems))
	assert.Equal(t, "check", decodedItems[0].Icon)
	assert.Equal(t, "Active status", decodedItems[0].Extra["description"])
}

func TestOption_CRUD(t *testing.T) {
	db := setupOptionTestDB(t)

	option := &models.Option{
		Category:    "system",
		Name:        "status",
		DisplayName: "Status Options",
		Description: "Test description",
		Status:      enum.Enabled,
		Version:     1,
		BuiltIn:     true,
		Items: &models.OptionItems{
			{Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1},
			{Key: "disabled", Label: "Disabled", Value: "disabled", Color: "red", Sort: 2},
		},
	}

	result := db.Create(option)
	assert.NoError(t, result.Error)
	assert.NotEmpty(t, option.ID)

	var fetched models.Option
	err := db.Where("category = ? AND name = ?", "system", "status").First(&fetched).Error
	assert.NoError(t, err)
	assert.Equal(t, "system", fetched.Category)
	assert.Equal(t, "status", fetched.Name)
	assert.Equal(t, 2, len(*fetched.Items))
}

func TestOptionVersion_Tracking(t *testing.T) {
	db := setupOptionTestDB(t)

	option := &models.Option{
		Category:    "system",
		Name:        "status",
		DisplayName: "Status Options",
		Status:      enum.Enabled,
		Version:     1,
		BuiltIn:     true,
		Items: &models.OptionItems{
			{Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1},
		},
	}
	result := db.Create(option)
	assert.NoError(t, result.Error)

	versionSnapshot := &models.OptionVersion{
		OptionID:   option.ID,
		Version:    option.Version,
		Items:      option.Items,
		ChangedBy:  "user-123",
		ChangeNote: "Initial version",
		Status:     enum.Enabled,
	}
	err := db.Create(versionSnapshot).Error
	assert.NoError(t, err)

	option.Version = 2
	option.Items = &models.OptionItems{
		{Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1},
		{Key: "disabled", Label: "Disabled", Value: "disabled", Color: "red", Sort: 2},
	}
	err = db.Save(option).Error
	assert.NoError(t, err)

	var updatedOption models.Option
	db.First(&updatedOption, "id = ?", option.ID)
	assert.Equal(t, 2, updatedOption.Version)
	assert.Equal(t, 2, len(*updatedOption.Items))

	var version models.OptionVersion
	db.First(&version, "option_id = ?", option.ID)
	assert.Equal(t, 1, version.Version)
	assert.NotNil(t, version.Items)
}

func TestOptionItem_Validation(t *testing.T) {
	item := models.OptionItem{
		Key:   "test",
		Label: "Test Option",
		Value: "test",
		Color: "blue",
		Sort:  1,
	}

	assert.Equal(t, "test", item.Key)
	assert.Equal(t, "Test Option", item.Label)
	assert.Equal(t, "test", item.Value)
	assert.Equal(t, "blue", item.Color)
	assert.Equal(t, 1, item.Sort)
}

func TestOptionItems_Value_Scan(t *testing.T) {
	items := &models.OptionItems{
		{ID: "1", Key: "a", Label: "A", Value: "a", Sort: 1},
		{ID: "2", Key: "b", Label: "B", Value: "b", Sort: 2},
	}

	value, err := items.Value()
	assert.NoError(t, err)
	assert.NotNil(t, value)

	var scannedItems models.OptionItems
	err = scannedItems.Scan(value)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(scannedItems))
}
