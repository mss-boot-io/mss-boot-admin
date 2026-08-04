package task

import (
	"reflect"
	"sync"
	"testing"
)

func TestDefaultStorageReturnsDeterministicKeys(t *testing.T) {
	storage := &defaultStorage{schedules: make(map[string]*schedule)}
	for _, key := range []string{"z", "a", "m"} {
		if err := storage.Set(key, 0, "@every 1s", testJob{}); err != nil {
			t.Fatalf("set %q: %v", key, err)
		}
	}
	keys, err := storage.ListKeys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if want := []string{"a", "m", "z"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
}

func TestDefaultStorageSupportsConcurrentAccess(t *testing.T) {
	storage := &defaultStorage{schedules: make(map[string]*schedule)}
	var wait sync.WaitGroup
	for i := 0; i < 20; i++ {
		wait.Add(2)
		go func() {
			defer wait.Done()
			_ = storage.Set("job", 1, "@every 1s", testJob{})
		}()
		go func() {
			defer wait.Done()
			_, _, _, _, _ = storage.Get("job")
			_, _ = storage.ListKeys()
		}()
	}
	wait.Wait()
}
