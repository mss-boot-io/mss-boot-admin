/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/28 03:38:06
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/28 03:38:06
 */

package pack

import (
	"os"
	"testing"
)

func TestTar(t *testing.T) {
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

			name: "test tar",
			args: args{
				root:   "../../testdata",
				ignore: []string{"test.tar.gz", "test.zip"},
				file:   "../../testdata/test.tar.gz",
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
			err = Tar(tt.args.root, tt.args.src, writer, tt.args.ignore...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Tar() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestTarX(t *testing.T) {
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
				src: "../../testdata/test.tar.gz",
				dst: "../../testdata/test",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TarX(tt.args.src, tt.args.dst)
			if (err != nil) != tt.wantErr {
				t.Errorf("Zip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestTarXByContent(t *testing.T) {
	content, _ := os.ReadFile("../../testdata/test.tar.gz")
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
			err := TarXByContent(tt.args.content, tt.args.dst)
			if (err != nil) != tt.wantErr {
				t.Errorf("Zip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
