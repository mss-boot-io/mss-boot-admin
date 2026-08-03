package enum

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/22 10:10:32
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/22 10:10:32
 */

type DataType string

const (
	DataTypeString DataType = "string"
	DataTypeInt    DataType = "int"
	DataTypeFloat  DataType = "float"
	DataTypeBool   DataType = "bool"
)

func (d DataType) String() string {
	return string(d)
}
