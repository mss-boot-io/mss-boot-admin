package system

import (
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"gorm.io/gorm"
	"runtime"
	"time"

	"github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration"
	common "github.com/mss-boot-io/mss-boot-admin-api/common/models"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
)

var Username string
var Password string

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1691847581348Test)
}

func _1691847581348Test(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		// TODO: here to write the content to be changed

		// TODO: e.g. modify table field, please delete this code during use
		//err := tx.Migrator().RenameColumn(&models.SysConfig{}, "config_id", "id")
		//if err != nil {
		// 	return err
		//}
		role := models.Role{
			Name:   "admin",
			Root:   true,
			Status: enum.Enabled,
			Remark: "admin",
		}
		err := tx.Create(&role).Error
		if err != nil {
			return err
		}
		err = tx.Create(&models.User{
			UserLogin: models.UserLogin{
				RoleID:   role.ID,
				Username: Username,
				Password: Password,
				Email:    "lwnmengjing@gmail.com",
				Status:   enum.Enabled,
			},
			Name:             "王力群",
			Avatar:           "https://lf1-xgcdn-tos.pstatp.com/obj/vcloud/vadmin/start.8e0e4855ee346a46ccff8ff3e24db27b.png",
			Job:              "backend",
			JobName:          "后端开发工程师",
			Organization:     "Backend",
			OrganizationName: "后端",
			Location:         "huaian",
			LocationName:     "淮安",
			Introduction:     "王力群并非是一个真实存在的人。",
			PersonalWebsite:  "https://www.arco.design",
			Verified:         true,
			PhoneNumber:      "18012345678",
			AccountID:        "1234567890",
			RegistrationTime: time.Now(),
		}).Error
		if err != nil {
			return err
		}

		// TODO: e.g. add table structure, please delete this code during use
		//err = tx.Migrator().AutoMigrate(
		//		new(models.CasbinRule),
		// 		)
		//if err != nil {
		// 	return err
		//}

		return tx.Create(&common.Migration{
			Version: version,
		}).Error
	})
}
