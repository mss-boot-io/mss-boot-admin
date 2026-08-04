package pkg

import (
	"reflect"
	"testing"
	"text/template/parse"
)

func TestGetKeys(t *testing.T) {
	tests := []struct {
		name string
		args string
		want []string
	}{
		{
			name: "test0",
			args: "{{.ab}}dsdf{{.b}}{{.c}} {{$abe = .a}}",
			want: []string{"ab", "b", "c", "a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := parse.Parse(tt.name, tt.args, "{{", "}}")
			got := getParseKeys(d[tt.name].Root)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetParseKeys() = %v, want %v", got, tt.want)
			}
			t.Log(got)
		})
	}
}
