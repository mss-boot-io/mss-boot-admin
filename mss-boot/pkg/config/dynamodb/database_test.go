package dynamodb

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/6 22:12:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/6 22:12:16
 */

import (
	"testing"
	"time"
)

func TestDatabase_Init(t *testing.T) {
	type fields struct {
		Region          string
		AccessKeyID     string
		AccessSecretKey string
		Timeout         time.Duration
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			"default",
			fields{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Database{
				Region:          tt.fields.Region,
				AccessKeyID:     tt.fields.AccessKeyID,
				AccessSecretKey: tt.fields.AccessSecretKey,
				Timeout:         tt.fields.Timeout,
			}
			e.Init()
		})
	}
}
