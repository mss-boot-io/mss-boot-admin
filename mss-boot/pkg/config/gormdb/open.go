package gormdb

import (
	"github.com/glebarez/sqlite"
	dm "github.com/nfjBill/gorm-driver-dm"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var Opens = map[string]func(string) gorm.Dialector{
	"mysql":    mysql.Open,
	"postgres": postgres.Open,
	"dm":       dm.Open,
	"sqlite":   sqlite.Open,
}
