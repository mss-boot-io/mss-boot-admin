package model

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/go-openapi/spec"

	"github.com/google/uuid"
	"github.com/spf13/cast"
	"gorm.io/gorm/schema"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/10 16:11:55
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/10 16:11:55
 */

type Field struct {
	ModelID                string          `json:"modelID,omitempty" yaml:"modelID" binding:"required" gorm:"column:model_id;type:varchar(64);not null;comment:模型id"`
	Name                   string          `json:"name" yaml:"name" binding:"required"`
	JsonTag                string          `json:"jsonTag" yaml:"jsonTag"`
	DataType               schema.DataType `json:"type" yaml:"type" binding:"required"`
	PrimaryKey             string          `json:"primaryKey" yaml:"primaryKey"`
	AutoIncrement          bool            `json:"autoIncrement" yaml:"autoIncrement"`
	AutoIncrementIncrement int64           `json:"autoIncrementIncrement" yaml:"autoIncrementIncrement"`
	Creatable              bool            `json:"creatable" yaml:"creatable"`
	Updatable              bool            `json:"updatable" yaml:"updatable"`
	Readable               bool            `json:"readable" yaml:"readable"`
	DefaultValue           string          `json:"defaultValue" yaml:"defaultValue"`
	DefaultValueFN         func() string   `json:"-" yaml:"-"`
	NotNull                bool            `json:"notNull" yaml:"notNull"`
	Unique                 string          `json:"unique" yaml:"unique"`
	Index                  string          `json:"index" yaml:"index"`
	Comment                string          `json:"comment" yaml:"comment"`
	Size                   int             `json:"size" yaml:"size"`
	Precision              int             `json:"precision" yaml:"precision"`
	Scale                  int             `json:"scale" yaml:"scale"`
	Search                 string          `json:"search" yaml:"search"`
}

type DefaultFN string

const (
	UUID DefaultFN = "uuid"
	Now  DefaultFN = "now"
)

var UUIDFN = func() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

var NowFN = func() string {
	return time.Now().String()
}

func (f *Field) Init() {
	if f.JsonTag == "" {
		f.JsonTag = f.Name
	}
	if f.PrimaryKey != "" && f.Name == "id" {
		f.DefaultValueFN = UUIDFN
	}
	if f.DataType == schema.Time && f.NotNull {
		f.DefaultValueFN = NowFN
	}
}

func (f *Field) GetName() string {
	return strings.ToUpper(f.Name[:1]) + f.Name[1:]
}

func (f *Field) MakeField() reflect.StructField {
	gormTag := fmt.Sprintf(`gorm:"column:%s`, f.Name)
	uriTag := ""
	if f.Size > 0 {
		gormTag = fmt.Sprintf(`%s;size:%d`, gormTag, f.Size)
	}
	if f.Index != "" {
		gormTag = fmt.Sprintf(`%s;index:%s`, gormTag, f.Index)
	}
	if f.NotNull {
		gormTag = fmt.Sprintf(`%s;not null`, gormTag)
	}
	if f.Unique != "" {
		gormTag = fmt.Sprintf(`%s;unique:%s`, gormTag, f.Unique)
	}
	if f.PrimaryKey != "" {
		if y, err := cast.ToBoolE(f.PrimaryKey); err == nil && y {
			gormTag = fmt.Sprintf(`%s;primaryKey`, gormTag)
		} else {
			gormTag = fmt.Sprintf(`%s;primaryKey:%s`, gormTag, f.PrimaryKey)
		}
		uriTag = fmt.Sprintf(` uri:"%s"`, f.JsonTag)
	}
	field := reflect.StructField{
		Name: f.GetName(),
		Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s,omitempty" %s"%s`, f.JsonTag, gormTag, uriTag)),
	}
	switch f.DataType {
	case schema.Bool:
		field.Type = reflect.TypeOf(false)
	case schema.Float:
		field.Type = reflect.TypeOf(float64(0))
	case schema.Int:
		field.Type = reflect.TypeOf(int(0))
	case schema.Uint:
		field.Type = reflect.TypeOf(uint(0))
	case schema.Time:
		field.Type = reflect.TypeOf(time.Time{})
	default:
		field.Type = reflect.TypeOf("")
	}
	return field
}

func (f *Field) GenOpenAPIFie() *spec.Schema {
	s := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type: []string{string(f.DataType)},
		},
	}
	if f.DataType == schema.String {
		s.MaxLength = ptr.Int64(int64(f.Size))
	}
	// todo float64
	// if f.DataType == schema.Float {
	//	s.Maximum = ptr.Float64(f.Precision)
	//	s.Minimum = &f.Scale
	//}
	if f.DataType == schema.Time {
		s.Format = "date-time"
	}
	if f.DefaultValue != "" {
		s.Default = f.DefaultValue
	}
	if f.NotNull {
		s.Nullable = false
	}
	// if f.PrimaryKey != "" {
	//	s.Extensions = spec.Extensions(map[string]interface{}{
	//		"x-order": f.PrimaryKey,
	//	})
	//}
	if f.Comment != "" {
		s.Description = f.Comment
	}
	return s
}
