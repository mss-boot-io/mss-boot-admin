package pkg

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/24 11:18 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/24 11:18 上午
 */

import (
	"reflect"
)

// Translate 结构体环转
func Translate(from, to any) {
	fType := reflect.TypeOf(from)
	fValue := reflect.ValueOf(from)
	if fType.Kind() == reflect.Ptr {
		fType = fType.Elem()
		fValue = fValue.Elem()
	}
	tType := reflect.TypeOf(to)
	tValue := reflect.ValueOf(to)
	if tType.Kind() == reflect.Ptr {
		tType = tType.Elem()
		tValue = tValue.Elem()
	}
	for i := 0; i < fType.NumField(); i++ {
		for j := 0; j < tType.NumField(); j++ {
			if fType.Field(i).Name == tType.Field(j).Name &&
				fType.Field(i).Type.ConvertibleTo(tType.Field(j).Type) {
				tValue.Field(j).Set(fValue.Field(i))
			}
		}
	}
}
