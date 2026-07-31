package local

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/21 18:25:06
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/21 18:25:06
 */

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

// Source is a local file source
type Source struct {
	opt  *source.Options
	path []string
}

// Open a file for reading
func (s *Source) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(s.opt.Dir, name))
}

// ReadFile read file
func (s *Source) ReadFile(name string) (rb []byte, err error) {
	for i := range source.Extends {
		path := filepath.Join(s.opt.Dir,
			fmt.Sprintf("%s.%s", name, source.Extends[i]))
		_, err = os.Stat(path)
		if err != nil {
			continue
		}
		rb, err = os.ReadFile(filepath.Join(s.opt.Dir,
			fmt.Sprintf("%s.%s", name, source.Extends[i])))
		if err == nil {
			if s.path == nil {
				s.path = make([]string, 0)
			}
			s.path = append(s.path, path)
			s.opt.Extend = source.Extends[i]
			return rb, nil
		}
	}
	return nil, err
}

func (s *Source) Watch(c source.Entity, unm func([]byte, any) error) error {
	slog.Debug("watch", slog.Any("path", s.path))
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add(s.path[0])

	for i := range s.path {
		err = watcher.Add(s.path[i])
		if err != nil {
			return err
		}
	}
	go func(sc *Source, cfg source.Entity, w *fsnotify.Watcher, decoder func([]byte, any) error) {
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					break
				}
				if filepath.Ext(event.Name) != "."+sc.opt.Extend.String() {
					continue
				}
				var rb []byte
				for i := range sc.path {
					rb, err = os.ReadFile(sc.path[i])
					if err != nil {
						slog.Error("read file error", slog.Any("error", err))
						continue
					}
					err = decoder(rb, cfg)
					if err != nil {
						slog.Error("unmarshal error", slog.Any("error", err))
						continue
					}
				}
				cfg.OnChange()
			case err, ok := <-w.Errors:
				if !ok {
					break
				}
				slog.Error("watch error", slog.Any("error", err))
			}
		}
	}(s, c, watcher, unm)
	return nil
}

// GetExtend get extend
func (s *Source) GetExtend() source.Scheme {
	return s.opt.Extend
}

// New returns a new source
func New(options ...source.Option) (*Source, error) {
	s := &Source{
		opt:  source.DefaultOptions(),
		path: make([]string, 0),
	}
	for _, opt := range options {
		opt(s.opt)
	}
	return s, nil
}
