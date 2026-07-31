package fs

/*
 * @Author: lwnmengjing
 * @Date: 2022/7/18 10:06:17
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/7/18 10:06:17
 */

import (
	"fmt"
	"io/fs"

	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

// Source is a local file source
type Source struct {
	opt *source.Options
}

// Open a file for reading
func (s *Source) Open(name string) (fs.File, error) {
	return s.opt.FS.Open(name)
}

// ReadFile read file
func (s *Source) ReadFile(name string) (rb []byte, err error) {
	for i := range source.Extends {
		rb, err = s.opt.FS.ReadFile(fmt.Sprintf("%s.%s", name, source.Extends[i]))
		if err == nil {
			s.opt.Extend = source.Extends[i]
			return rb, nil
		}
	}
	return nil, err
}

func (s *Source) Watch(_ source.Entity, _ func([]byte, any) error) error {
	return nil
}

// GetExtend get extend
func (s *Source) GetExtend() source.Scheme {
	return s.opt.Extend
}

// New source
func New(options ...source.Option) (*Source, error) {
	s := &Source{
		opt: source.DefaultOptions(),
	}
	for _, opt := range options {
		opt(s.opt)
	}
	return s, nil
}
