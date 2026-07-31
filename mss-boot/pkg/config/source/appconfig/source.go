package appconfig

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/appconfigdata"

	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/7/18 19:23:30
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/7/18 19:23:30
 */

type Source struct {
	opt *source.Options
}

func (s *Source) Open(string) (fs.File, error) {
	return nil, errors.New("method Get not implemented")
}

func (s *Source) ReadFile(name string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), s.opt.Timeout)
	defer cancel()
	for i := range source.Extends {
		sessionOutput, err := s.opt.APPConfigDataClient.
			StartConfigurationSession(ctx, &appconfigdata.StartConfigurationSessionInput{
				ApplicationIdentifier:          aws.String(s.opt.ProjectName),
				EnvironmentIdentifier:          aws.String(s.opt.Namespace),
				ConfigurationProfileIdentifier: aws.String(fmt.Sprintf("%s.%s", name, source.Extends[i])),
			})
		if err != nil {
			if strings.Contains(err.Error(), "ConfigurationProfile not found") {
				continue
			}
			return nil, err
		}
		output, err := s.opt.APPConfigDataClient.
			GetLatestConfiguration(ctx, &appconfigdata.GetLatestConfigurationInput{
				ConfigurationToken: sessionOutput.InitialConfigurationToken,
			})
		if err != nil {
			return nil, err
		}
		s.opt.Extend = source.Extends[i]
		return output.Configuration, nil
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
	if s.opt.ProjectName == "" {
		return nil, errors.New("project name is required")
	}
	if s.opt.Namespace == "" {
		return nil, errors.New("namespace is required")
	}
	ctx, cancel := context.WithTimeout(context.TODO(), s.opt.Timeout)
	defer cancel()
	cfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	s.opt.APPConfigDataClient = appconfigdata.NewFromConfig(cfg)
	return s, nil
}
