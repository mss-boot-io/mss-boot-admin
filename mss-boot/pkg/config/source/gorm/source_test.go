package gorm

import (
	"io/fs"
	"reflect"
	"testing"

	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/10/30 10:46:00
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/10/30 10:46:00
 */

func TestSource_Open(t *testing.T) {
	type fields struct {
		opt *source.Options
	}
	type args struct {
		in0 string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    fs.File
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{
				opt: tt.fields.opt,
			}
			got, err := s.Open(tt.args.in0)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Open() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSource_ReadFile(t *testing.T) {
	type fields struct {
		opt *source.Options
	}
	type args struct {
		in0 string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{
				opt: tt.fields.opt,
			}
			got, err := s.ReadFile(tt.args.in0)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
