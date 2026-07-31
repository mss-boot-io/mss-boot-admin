package s3

/*
 * @Author: lwnmengjing
 * @Date: 2022/7/18 10:06:11
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/7/18 10:06:11
 */

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

// Source is a s3 file source
type Source struct {
	opt *source.Options
}

// Open a file for reading
func (s *Source) Open(string) (fs.File, error) {
	return nil, errors.New("method Get not implemented")
}

// ReadFile read file
func (s *Source) ReadFile(name string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), s.opt.Timeout)
	defer cancel()
	for i := range source.Extends {
		object, err := s.opt.S3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.opt.Bucket),
			Key: aws.String(
				fmt.Sprintf("%s/%s",
					s.opt.Dir,
					fmt.Sprintf("%s.%s", name, source.Extends[i]))),
		})
		if err != nil {
			return nil, err
		}
		rb, err := io.ReadAll(object.Body)
		if err == nil {
			_ = object.Body.Close()
			s.opt.Extend = source.Extends[i]
			return rb, nil
		}
	}
	return nil, nil
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
	if s.opt.ProjectName != "" {
		s.opt.Dir = s.opt.Dir[strings.Index(s.opt.Dir, s.opt.ProjectName+"/"):]
	}
	if s.opt.S3Client != nil {
		return s, nil
	}

	ctx, cancel := context.WithTimeout(context.TODO(), s.opt.Timeout)
	defer cancel()
	cfg, err := s3config.LoadDefaultConfig(ctx, s3config.WithRegion(s.opt.Region))
	if err != nil {
		return nil, err
	}
	s.opt.S3Client = s3.NewFromConfig(cfg)
	return s, nil
}
