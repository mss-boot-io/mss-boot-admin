package pkg

import "testing"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/5/29 07:59:57
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/5/29 07:59:57
 */

func TestCompareHashAndPassword(t *testing.T) {
	type args struct {
		hash     string
		password string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "test password",
			args: args{
				hash:     "$2a$10$1Z0Z1Z0Z1Z0Z1Z0Z1Z0Z1uJZ1Z0Z1Z0Z1Z0Z1Z0Z1Z0Z1Z0Z1Z0Z1",
				password: "123456",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareHashAndPassword(tt.args.hash, tt.args.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareHashAndPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CompareHashAndPassword() got = %v, want %v", got, tt.want)
			}
		})
	}
}
