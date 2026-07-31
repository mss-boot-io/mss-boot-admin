package task

import (
	"sync"

	"github.com/robfig/cron/v3"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 16:56:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 16:56:16
 */

// Storage storage interface
type Storage interface {
	Get(key string) (entryID cron.EntryID, spec string, job cron.Job, exist bool, err error)
	Set(key string, entryID cron.EntryID, spec string, job cron.Job) error
	Update(key string, entryID cron.EntryID) error
	Remove(key string) error
	ListKeys() ([]string, error)
}

type defaultStorage struct {
	schedules map[string]*schedule
	mux       sync.Mutex
}

// Get schedule
func (s *defaultStorage) Get(key string) (entryID cron.EntryID, spec string, job cron.Job, exist bool, err error) {
	if s.schedules == nil {
		return
	}
	item, ok := s.schedules[key]
	if !ok {
		return
	}
	entryID = item.entryID
	spec = item.spec
	job = item.job
	exist = true
	return
}

// Set schedule
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

// Update schedule
func (s *defaultStorage) Update(key string, entryID cron.EntryID) error {
	if s.schedules == nil {
		s.schedules = make(map[string]*schedule)
		return nil
	}
	item, ok := s.schedules[key]
	if !ok {
		return nil
	}
	item.entryID = entryID
	return nil
}

func (s *defaultStorage) Remove(key string) error {
	if s.schedules == nil {
		return nil
	}
	delete(s.schedules, key)
	return nil
}

// ListKeys list keys
func (s *defaultStorage) ListKeys() ([]string, error) {
	keys := make([]string, 0, len(s.schedules))
	for k := range s.schedules {
		keys = append(keys, k)
	}
	return keys, nil
}
