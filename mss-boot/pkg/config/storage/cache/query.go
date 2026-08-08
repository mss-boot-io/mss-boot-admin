package cache

import (
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
)

func QueryDB(tx *gorm.DB) {
	if tx.Error != nil || tx.DryRun {
		return
	}
	if tx.Statement.SQL.Len() == 0 {
		callbacks.BuildQuerySQL(tx)
	}
	rows, err := tx.Statement.ConnPool.QueryContext(tx.Statement.Context, tx.Statement.SQL.String(), tx.Statement.Vars...)
	if err != nil {
		_ = tx.AddError(err)
		return
	}

	defer func() {
		_ = tx.AddError(rows.Close())
	}()

	gorm.Scan(rows, tx, 0)
}
