package gorm

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/2/11 01:22:28
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/2/11 01:22:28
 */

import (
	"errors"
	"io/fs"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

// Source source
type Source struct {
	opt *source.Options
	db  *gorm.DB
}

// Open method Get not implemented
func (s *Source) Open(string) (fs.File, error) {
	return nil, errors.New("method Get not implemented")
}

// ReadFile method Get not implemented
func (s *Source) ReadFile(name string) ([]byte, error) {
	if s.opt.Driver == nil {
		return nil, errors.New("method Get not implemented")
	}
	if strings.Contains(name, ".") {
		name = name[:strings.Index(name, ".")]
	}
	m := pkg.DeepCopy(s.opt.Driver).(source.Driver)
	err := s.db.Model(m).Where("name = ?", name).First(m).Error
	if err != nil {
		return nil, err
	}
	s.opt.Extend = m.GetExtend()
	return m.GenerateBytes()
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
	for _, o := range options {
		o(s.opt)
	}
	if s.opt.Timeout == 0 {
		s.opt.Timeout = 5 * time.Second
	}
	var err error
	s.db, err = gorm.Open(gormdb.Opens[s.opt.GORMDriver](s.opt.GORMDsn),
		&gorm.Config{
			NamingStrategy: schema.NamingStrategy{
				SingularTable: true,
			},
			Logger: logger.Default,
		})
	if err != nil {
		return nil, err
	}
	return s, nil
}
