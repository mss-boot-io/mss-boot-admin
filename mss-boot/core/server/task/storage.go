package task

import (
	"sort"
	"sync"

	"github.com/robfig/cron/v3"
)

// Storage persists scheduled-job metadata.
type Storage interface {
	Get(key string) (entryID cron.EntryID, spec string, job cron.Job, exist bool, err error)
	Set(key string, entryID cron.EntryID, spec string, job cron.Job) error
	Update(key string, entryID cron.EntryID) error
	Remove(key string) error
	ListKeys() ([]string, error)
}

type defaultStorage struct {
	schedules map[string]*schedule
	mux       sync.RWMutex
}

// Get returns one schedule.
func (s *defaultStorage) Get(key string) (entryID cron.EntryID, spec string, job cron.Job, exist bool, err error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if s.schedules == nil {
		return
	}
	item, ok := s.schedules[key]
	if !ok || item == nil {
		return
	}
	entryID = item.entryID
	spec = item.spec
	job = item.job
	exist = true
	return
}

// Set creates or replaces a schedule.
func (s *defaultStorage) Set(key string, entryID cron.EntryID, spec string, job cron.Job) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.schedules == nil {
		s.schedules = make(map[string]*schedule)
	}
	s.schedules[key] = &schedule{
		spec:    spec,
		entryID: entryID,
		job:     job,
	}
	return nil
}

// Update changes the cron entry ID for a schedule.
func (s *defaultStorage) Update(key string, entryID cron.EntryID) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.schedules == nil {
		s.schedules = make(map[string]*schedule)
		return nil
	}
	item, ok := s.schedules[key]
	if !ok || item == nil {
		return nil
	}
	item.entryID = entryID
	return nil
}

// Remove deletes a schedule.
func (s *defaultStorage) Remove(key string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.schedules == nil {
		return nil
	}
	delete(s.schedules, key)
	return nil
}

// ListKeys returns schedule keys in deterministic order.
func (s *defaultStorage) ListKeys() ([]string, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	keys := make([]string, 0, len(s.schedules))
	for key := range s.schedules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}
