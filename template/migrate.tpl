package {{.Package}}

import (
	"runtime"

	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration"
	common "github.com/mss-boot-io/mss-boot-admin-api/common/models"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _{{.GenerateTime}}Migrate)
}

func _{{.GenerateTime}}Migrate(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

	    // TODO: here to write the content to be changed

	    // TODO: e.g. modify table field, please delete this code during use
        //err := tx.Migrator().RenameColumn(&models.SysConfig{}, "config_id", "id")
		//if err != nil {
		// 	return err
		//}

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
