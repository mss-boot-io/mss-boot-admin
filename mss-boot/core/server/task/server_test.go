package task

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/4/30 10:33:36
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/4/30 10:33:36
 */

import (
	"context"
	"fmt"
	"testing"
	"time"
)

var chanResult = make(chan string, 4)

type testJob struct {
	key string
}

func (t *testJob) Run() {
	fmt.Printf("%v test job run: %s\n", time.Now(), t.key)
	go func() {
		chanResult <- t.key
	}()
}

// TestNew non-blocking test
func TestNew(t *testing.T) {
	type args struct {
		opts []Option
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantArr []string
	}{
		{
			name: "test",
			args: args{
				opts: []Option{},
			},
			want:    true,
			wantArr: []string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			err := c.Start(context.TODO())
			if (err == nil) != tt.want {
				t.Errorf("New() error = %v, wantErr %v", err, tt.want)
				return
			}
			for _, k := range tt.wantArr {
				err = UpdateJob(fmt.Sprintf("test-%s", k), "* * * * * *", &testJob{key: k})
				if (err == nil) != tt.want {
					t.Errorf("New() error = %v, wantErr %v", err, tt.want)
					return
				}
			}
			var count int
			for r := range chanResult {
				count++
				var ok bool
				for _, k := range tt.wantArr {
					if r == k {
						t.Logf("New() success, wantArr %v", tt.wantArr)
						ok = true
						break
					}
				}
				if !ok {
					t.Errorf("New() error, wantArr %v", tt.wantArr)
					return
				}
				if count == len(tt.wantArr) {
					break
				}
			}
			for i := range tt.wantArr {
				count++
				if i == len(tt.wantArr)-1 {
					break
				}
				err = RemoveJob(fmt.Sprintf("test-%s", tt.wantArr[i]))
				if (err == nil) != tt.want {
					t.Errorf("New() error = %v, wantErr %v", err, tt.want)
					return
				}
			}
			k := <-chanResult
			t.Logf("Print %s", k)
			if count-1 < len(tt.wantArr) {
				t.Errorf("New() error, wantArr %v", tt.wantArr)
				return
			}
		})
	}
}
