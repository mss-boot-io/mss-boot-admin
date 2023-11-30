package pkg

import (
	"testing"
)

func Test_GistClone(t *testing.T) {
	tests := []struct {
		id    string
		dir   string
		token string
		want  []string
	}{
		{
			id:   "1",
			dir:  "../test",
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			err := GistClone(tt.id, tt.dir, tt.token)
			if err != nil {
				t.Errorf("GistClone() err %v", err)
			}
		})
	}
}
