package gormdb

import (
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

var policies = map[string]dbresolver.Policy{
	"random": dbresolver.RandomPolicy{},
}

// Configure db config
type Configure interface {
	Init(*gorm.Config, func(string) gorm.Dialector) (*gorm.DB, error)
}

// ResolverConfigure db resolver config
type ResolverConfigure interface {
	Init(*dbresolver.DBResolver, func(string) gorm.Dialector) *dbresolver.DBResolver
}
