/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/28 03:32:35
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/28 03:32:35
 */

package pack

import (
	"os"
	"testing"
)

func TestZip(t *testing.T) {
	type args struct {
		root   string
		src    []string
		ignore []string
		file   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test zip",
			args: args{
				root:   "../../testdata",
				ignore: []string{"test.tar.gz", "test.zip"},
				file:   "../../testdata/test.zip",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := os.Create(tt.args.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("Zip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			defer writer.Close()
			err = Zip(tt.args.root, tt.args.src, writer, tt.args.ignore...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Zip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestUnzip(t *testing.T) {
	type args struct {
		src string
		dst string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test unzip",
			args: args{
				src: "../../testdata/test.zip",
				dst: "../../testdata/test",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Unzip(tt.args.src, tt.args.dst)
			if (err != nil) != tt.wantErr {
				t.Errorf("Zip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestUnzipByContent(t *testing.T) {
	content, _ := os.ReadFile("../../testdata/test.zip")
	type args struct {
		content []byte
		dst     string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test unzip",
			args: args{
				content: content,
				dst:     "../../testdata/test",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnzipByContent(tt.args.content, tt.args.dst)
			if (err != nil) != tt.wantErr {
				t.Errorf("Zip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
