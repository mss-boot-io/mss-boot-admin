package pkg

import (
	"reflect"
	"testing"
)

func TestGetSubPath(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    []string
		wantErr bool
	}{
		{
			"test0",
			"../example",
			[]string{"clone", "media", "scanf"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSubPath(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSubPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSubPath() got = %v, want %v", got, tt.want)
			}
		})
	}
}
