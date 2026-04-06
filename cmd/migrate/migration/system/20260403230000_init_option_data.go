package system

import (
	"runtime"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260403230000InitOptionData)
}

func _20260403230000InitOptionData(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		statusOption := models.Option{
			Category:    "system",
			Name:        "status",
			DisplayName: "Status Options",
			Description: "Basic status options for all entities",
			Remark:      "System status options",
			Status:      enum.Enabled,
			Version:     1,
			BuiltIn:     true,
			Items: &models.OptionItems{
				{ID: uuid.New().String(), Key: "enabled", Label: "Enabled", Value: "enabled", Color: "green", Sort: 1},
				{ID: uuid.New().String(), Key: "disabled", Label: "Disabled", Value: "disabled", Color: "red", Sort: 2},
				{ID: uuid.New().String(), Key: "locked", Label: "Locked", Value: "locked", Color: "yellow", Sort: 3},
			},
		}
		err := tx.Create(&statusOption).Error
		if err != nil {
			return err
		}

		dataScopeOption := models.Option{
			Category:    "permission",
			Name:        "dataScope",
			DisplayName: "Data Scope Options",
			Description: "Data scope options for permission control",
			Remark:      "System data scope options",
			Status:      enum.Enabled,
			Version:     1,
			BuiltIn:     true,
			Items: &models.OptionItems{
				{ID: uuid.New().String(), Key: "all", Label: "All", Value: "all", Color: "green", Sort: 1},
				{ID: uuid.New().String(), Key: "currentDept", Label: "Current Department", Value: "currentDept", Color: "red", Sort: 2},
				{ID: uuid.New().String(), Key: "currentAndChildrenDept", Label: "Current and Children Departments", Value: "currentAndChildrenDept", Color: "yellow", Sort: 3},
				{ID: uuid.New().String(), Key: "customDept", Label: "Custom Departments", Value: "customDept", Color: "yellow", Sort: 4},
				{ID: uuid.New().String(), Key: "self", Label: "Self Only", Value: "self", Color: "yellow", Sort: 5},
				{ID: uuid.New().String(), Key: "selfAndChildren", Label: "Self and Children", Value: "selfAndChildren", Color: "yellow", Sort: 6},
				{ID: uuid.New().String(), Key: "selfAndAllChildren", Label: "Self and All Children", Value: "selfAndAllChildren", Color: "yellow", Sort: 7},
			},
		}
		err = tx.Create(&dataScopeOption).Error
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}