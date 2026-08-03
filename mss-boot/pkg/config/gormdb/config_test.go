package gormdb

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var dsn0 = "file:primary?mode=memory&cache=shared"
var dsn1 = "file:replica?mode=memory&cache=shared"
var tables = []interface{}{"sys_user", "sys_role"}

func TestDBConfig_Init(t *testing.T) {
	type fields struct {
		dsn             string
		connMaxIdleTime int
		connMaxLifetime int
		maxIdleConns    int
		maxOpenConns    int
		registers       []ResolverConfigure
	}
	type args struct {
		config *gorm.Config
		open   func(string) gorm.Dialector
	}
	registers := make([]ResolverConfigure, 0)
	registers = append(registers, &dbResolverConfig{
		sources:  []string{dsn0},
		replicas: []string{dsn1},
		policy:   "random",
		tables:   tables,
	})
	registers = append(registers, &dbResolverConfig{
		sources:  []string{dsn0},
		replicas: []string{dsn1},
		policy:   "random",
		tables:   tables,
	})
	registers = append(registers, &dbResolverConfig{
		sources:  []string{dsn0},
		replicas: []string{dsn1},
		policy:   "random",
		//tables:   tables,
	})
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"test0",
			fields{
				dsn: dsn0,
			},
			args{
				config: &gorm.Config{},
				open:   sqlite.Open,
			},
			false,
		},
		{
			"test1",
			fields{
				dsn:             dsn0,
				connMaxIdleTime: 600,
				connMaxLifetime: 60,
				maxIdleConns:    200,
				maxOpenConns:    100,
				registers:       registers,
			},
			args{
				config: &gorm.Config{},
				open:   sqlite.Open,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &DBConfig{
				dsn:             tt.fields.dsn,
				connMaxIdleTime: tt.fields.connMaxIdleTime,
				connMaxLifetime: tt.fields.connMaxLifetime,
				maxIdleConns:    tt.fields.maxIdleConns,
				maxOpenConns:    tt.fields.maxOpenConns,
				registers:       tt.fields.registers,
			}
			_, err := e.Init(tt.args.config, tt.args.open)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
