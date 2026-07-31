package mgdb

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/13 04:03:10
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/13 04:03:10
 */

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"time"

	mgm "github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
	mongoOptions "go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

// Source source
type Source struct {
	opt *source.Options
}

// Open method Get not implemented
func (s *Source) Open(string) (fs.File, error) {
	return nil, errors.New("method Get not implemented")
}

// ReadFile read file
func (s *Source) ReadFile(name string) ([]byte, error) {
	if s.opt.Driver == nil {
		return nil, errors.New("method Get not implemented")
	}
	if strings.Contains(name, ".") {
		name = name[:strings.Index(name, ".")]
	}
	m := pkg.DeepCopy(s.opt.Driver).(source.Driver)
	ctx, cancel := context.WithTimeout(
		context.TODO(),
		s.opt.Timeout)
	defer cancel()
	err := mgm.Coll(m.(mgm.Model)).FirstWithCtx(ctx, bson.M{"name": name}, m.(mgm.Model))
	if err != nil {
		return nil, err
	}
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
	for _, opt := range options {
		opt(s.opt)
	}
	if s.opt.Timeout == 0 {
		s.opt.Timeout = 5 * time.Second
	}
	serverAPIOptions := mongoOptions.ServerAPI(mongoOptions.ServerAPIVersion1)
	clientOptions := mongoOptions.Client().
		ApplyURI(s.opt.MongoDBURL).
		SetServerAPIOptions(serverAPIOptions)
	err := mgm.SetDefaultConfig(&mgm.Config{CtxTimeout: s.opt.Timeout}, s.opt.MongoDBName, clientOptions)
	if err != nil {
		return nil, err
	}
	return s, nil
}
