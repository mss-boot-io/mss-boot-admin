package model

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-openapi/spec"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/10 15:29:38
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/10 15:29:38
 */

type Model struct {
	Table       string   `json:"tableName" yaml:"tableName" binding:"required"`
	Name        string   `json:"name" yaml:"name" binding:"required"`
	Description string   `json:"description" yaml:"description"`
	HardDeleted bool     `json:"hardDeleted" yaml:"hardDeleted"`
	Fields      []*Field `json:"fields" yaml:"fields" binding:"required"`
	MultiTenant bool     `json:"multiTenant" yaml:"multiTenant"`
	Auth        bool     `json:"auth" yaml:"auth"`
}

// TableName get table name
func (m *Model) TableName() string {
	return m.Table
}

// PrimaryKeys get primary keys support multi keys
func (m *Model) PrimaryKeys() []string {
	var keys []string
	for i := range m.Fields {
		if m.Fields[i].PrimaryKey != "" {
			keys = append(keys, m.Fields[i].Name)
		}
	}
	return keys
}

func (m *Model) Init() {
	for i := range m.Fields {
		m.Fields[i].Init()
	}
}

func (m *Model) Default(data any) {
	for i := range m.Fields {
		df := m.Fields[i].DefaultValue
		if m.Fields[i].DefaultValueFN != nil {
			df = m.Fields[i].DefaultValueFN()
		}
		if df == "" {
			continue
		}
		reflect.ValueOf(data).Elem().FieldByName(m.Fields[i].GetName()).Set(reflect.ValueOf(df))
	}
}

// MakeModel make virtual model
func (m *Model) MakeModel() any {
	m.Init()
	s := reflect.New(reflect.StructOf(m.MakeField()))
	return s.Interface()
}

func (m *Model) MakeList() any {
	m.Init()
	return reflect.New(reflect.SliceOf(reflect.StructOf(m.MakeField()))).Interface()
}

func (m *Model) Migrate(db *gorm.DB) error {
	return db.Table(m.TableName()).AutoMigrate(m.MakeModel())
}

func (m *Model) MakeField() []reflect.StructField {
	fieldTypes := make([]reflect.StructField, 0)
	for i := range m.Fields {
		fieldTypes = append(fieldTypes, m.Fields[i].MakeField())
	}
	fieldTypes = append(fieldTypes, m.MakeTimeField()...)
	return fieldTypes
}

func (m *Model) MakeTimeField() []reflect.StructField {
	fieldTypes := []reflect.StructField{
		{
			Name: "CreatedAt",
			Type: reflect.TypeOf(time.Time{}),
			Tag:  "json:\"createdAt,omitempty\"",
		}, {
			Name: "UpdatedAt",
			Type: reflect.TypeOf(time.Time{}),
			Tag:  "json:\"updatedAt,omitempty\"",
		},
	}
	if !m.HardDeleted {
		fieldTypes = append(fieldTypes, reflect.StructField{
			Name: "DeletedAt",
			Type: reflect.TypeOf(gorm.DeletedAt{}),
			Tag:  "json:\"-\" gorm:\"index\"",
		})
	}
	if m.MultiTenant {
		fieldTypes = append(fieldTypes, reflect.StructField{
			Name: "TenantID",
			Type: reflect.TypeOf(any("")),
			Tag:  "json:\"tenantID,omitempty\" gorm:\"index\"",
		})
	}
	return fieldTypes
}

func (m *Model) TableScope(db *gorm.DB) *gorm.DB {
	return db.Table(m.TableName())
}

func (m *Model) TenantScope(ctx *gin.Context,
	fn func(ctx *gin.Context) (any, error)) func(db *gorm.DB) *gorm.DB {
	if !m.MultiTenant {
		return func(db *gorm.DB) *gorm.DB {
			return db
		}
	}
	return func(db *gorm.DB) *gorm.DB {
		tenantID, err := fn(ctx)
		if err != nil {
			_ = db.AddError(err)
			return db
		}
		return db.Where(fmt.Sprintf("%s.tenant_id = ?", m.TableName()), tenantID)
	}
}

func (m *Model) URI(ctx *gin.Context) (f func(*gorm.DB) *gorm.DB) {
	return func(db *gorm.DB) *gorm.DB {
		db = db.Table(m.TableName())
		for _, key := range m.PrimaryKeys() {
			db = db.Where(fmt.Sprintf("%s in (?)", key), strings.Split(ctx.Param(key), ","))
		}
		return db
	}
}

func (m *Model) Pagination(ctx *gin.Context, p PaginationImp) (f func(*gorm.DB) *gorm.DB) {
	err := ctx.ShouldBindQuery(p)
	return func(db *gorm.DB) *gorm.DB {
		if err != nil {
			_ = db.AddError(err)
			return db
		}
		offset := (p.GetPage() - 1) * p.GetPageSize()
		return db.Limit(int(p.GetPageSize())).Offset(int(offset))
	}
}

func (m *Model) Search(ctx *gin.Context) (f func(*gorm.DB) *gorm.DB) {
	return func(db *gorm.DB) *gorm.DB {
		for i := range m.Fields {
			if m.Fields[i].Search == "" {
				continue
			}
			v, ok := ctx.GetQuery(m.Fields[i].JsonTag)
			if !ok {
				continue
			}
			switch m.Fields[i].Search {
			case "exact", "iexact":
				db = db.Where(fmt.Sprintf("`%s`.`%s` = ?", m.Table, m.Fields[i].Name), v)
			case "contains", "icontains":
				db = db.Where(fmt.Sprintf("`%s`.`%s` like ?", m.Table, m.Fields[i].Name), "%"+v+"%")
			case "gt":
				db = db.Where(fmt.Sprintf("`%s`.`%s` > ?", m.Table, m.Fields[i].Name), v)
			case "gte":
				db = db.Where(fmt.Sprintf("`%s`.`%s` >= ?", m.Table, m.Fields[i].Name), v)
			case "lt":
				db = db.Where(fmt.Sprintf("`%s`.`%s` < ?", m.Table, m.Fields[i].Name), v)
			case "lte":
				db = db.Where(fmt.Sprintf("`%s`.`%s` <= ?", m.Table, m.Fields[i].Name), v)
			case "startWith", "istartWith":
				db = db.Where(fmt.Sprintf("`%s`.`%s` like ?", m.Table, m.Fields[i].Name), v+"%")
			case "endWith", "iendWith":
				db = db.Where(fmt.Sprintf("`%s`.`%s` like ?", m.Table, m.Fields[i].Name), "%"+v)
			case "in":
				arr, ok := ctx.GetQueryArray(m.Fields[i].JsonTag)
				if !ok {
					continue
				}
				db = db.Where(fmt.Sprintf("`%s`.`%s` in (?)", m.Table, m.Fields[i].JsonTag), arr)
			case "isnull":
				db = db.Where(fmt.Sprintf("`%s`.`%s` isnull", m.Table, m.Fields[i].JsonTag))
			case "order":
				switch v {
				case "desc":
					db = db.Order(fmt.Sprintf("`%s`.`%s` desc", m.Table, m.Fields[i].JsonTag))
				case "asc":
					db = db.Order(fmt.Sprintf("`%s`.`%s` asc", m.Table, m.Fields[i].JsonTag))
				}
			}
		}
		return db
	}
}

func (m *Model) GenOpenAPIModel() *spec.Schema {
	s := spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:       []string{"object"},
			Properties: make(map[string]spec.Schema),
		},
	}
	for i := range m.Fields {
		s.Properties[m.Fields[i].JsonTag] = *m.Fields[i].GenOpenAPIFie()
	}
	return &s
}
