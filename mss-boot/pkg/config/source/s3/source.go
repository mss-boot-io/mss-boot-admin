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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
)

// Source is a s3 file source
type Source struct {
	opt *source.Options
}

var ErrWatchUnsupported = errors.New("S3 configuration source watch is unsupported")

// Open a file for reading
func (s *Source) Open(string) (fs.File, error) {
	return nil, errors.New("method Get not implemented")
}

// ReadFile read file
func (s *Source) ReadFile(name string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(s.opt.Context, s.opt.Timeout)
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
			if isObjectNotFound(err) {
				continue
			}
			return nil, err
		}
		rb, err := io.ReadAll(object.Body)
		closeErr := object.Body.Close()
		if err != nil {
			return nil, errors.Join(err, closeErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		s.opt.Extend = source.Extends[i]
		return rb, nil
	}
	return nil, fs.ErrNotExist
}

func isObjectNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	switch apiError.ErrorCode() {
	case "NoSuchKey":
		return true
	default:
		return false
	}
}

func (s *Source) Watch(_ source.Entity, _ func([]byte, any) error) error {
	return ErrWatchUnsupported
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
	if s.opt.Context == nil {
		return nil, errors.New("S3 source context is required")
	}
	if s.opt.ProjectName != "" {
		projectIndex := strings.Index(s.opt.Dir, s.opt.ProjectName+"/")
		if projectIndex < 0 {
			return nil, errors.New("S3 source directory does not contain project name")
		}
		s.opt.Dir = s.opt.Dir[projectIndex:]
	}
	if s.opt.S3Client == nil {
		return nil, errors.New("borrowed S3 source client is required")
	}
	return s, nil
}
