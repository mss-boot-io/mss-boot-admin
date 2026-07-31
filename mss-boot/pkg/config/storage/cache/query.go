package cache

import "gorm.io/gorm"

func QueryDB(tx *gorm.DB) {
	if tx.Error != nil || tx.DryRun {
		return
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
