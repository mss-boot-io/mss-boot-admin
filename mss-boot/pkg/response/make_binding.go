package response

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/24 10:03 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/24 10:03 上午
 */

import (
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin/binding"
)

const (
	_ uint8 = iota
	json
	xml
	yaml
	form
	query
)

var constructor = &bindConstructor{}

type bindConstructor struct {
	cache map[string][]uint8
	mux   sync.Mutex
}

func (e *bindConstructor) GetBindingForGin(d any) []binding.Binding {
	bs := e.getBinding(reflect.TypeOf(d).String())
	if bs == nil {
		// 重新构建
		bs = e.resolve(d)
	}
	gbsMap := make(map[uint8]binding.Binding)
	for i := range bs {
		switch bs[i] {
		case json:
			gbsMap[bs[i]] = binding.JSON
		case xml:
			gbsMap[bs[i]] = binding.XML
		case yaml:
			gbsMap[bs[i]] = binding.YAML
		case form:
			gbsMap[bs[i]] = binding.Form
		case query:
			gbsMap[bs[i]] = binding.Query
		default:
			gbsMap[bs[i]] = nil
		}
	}
	gbs := make([]binding.Binding, 0)
	for k := range gbsMap {
		gbs = append(gbs, gbsMap[k])
	}
	return gbs
}

func (e *bindConstructor) resolve(d any) []uint8 {
	bs := make([]uint8, 0)
	qType := reflect.TypeOf(d)
	if qType.Kind() == reflect.Ptr {
		qType = qType.Elem()
	}
	qValue := reflect.ValueOf(d)
	if qValue.Kind() == reflect.Ptr {
		qValue = qValue.Elem()
	}
	var tag reflect.StructTag
	var ok bool
	var v string
	for i := 0; i < qType.NumField(); i++ {
		if qType.Field(i).Type.Kind() == reflect.Struct {
			// 递归
			bs = append(bs, e.resolve(qValue.Field(i).Interface())...)
		}
		tag = qType.Field(i).Tag
		if v, ok = tag.Lookup("json"); ok && v != "-" {
			bs = append(bs, json)
		}
		if v, ok = tag.Lookup("xml"); ok && v != "-" {
			bs = append(bs, xml)
		}
		if v, ok = tag.Lookup("yaml"); ok && v != "-" {
			bs = append(bs, yaml)
		}
		// if v, ok = tag.Lookup("form"); ok && v != "-" {
		//	bs = append(bs, form)
		//}
		if v, ok = tag.Lookup("query"); ok && v != "-" {
			bs = append(bs, query)
		}
		if v, ok = tag.Lookup("uri"); ok && v != "-" {
			bs = append(bs, 0)
		}
		if t, ok := tag.Lookup("binding"); ok && strings.Contains(t, "dive") {
			qValue := reflect.ValueOf(d)
			bs = append(bs, e.resolve(qValue.Field(i))...)
			continue
		}
		if t, ok := tag.Lookup("validate"); ok && strings.Contains(t, "dive") {
			qValue := reflect.ValueOf(d)
			bs = append(bs, e.resolve(qValue.Field(i))...)
		}
	}
	return bs
}

func (e *bindConstructor) getBinding(name string) []uint8 {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.cache[name]
}

func (e *bindConstructor) setBinding(name string, bs []uint8) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e.cache == nil {
		e.cache = make(map[string][]uint8)
	}
	e.cache[name] = bs
}
