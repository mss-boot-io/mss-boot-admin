package dynamodb

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/6 21:47:30
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/6 21:47:30
 */

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// DB dynamodb client
var DB *dynamodb.Client

var tables = make([]Tabler, 0)

// Database database
type Database struct {
	Region          string        `yaml:"region" json:"region"`
	AccessKeyID     string        `yaml:"accessKeyID" json:"accessKeyID"`
	AccessSecretKey string        `yaml:"accessSecretKey" json:"accessSecretKey"`
	Timeout         time.Duration `yaml:"timeout" json:"timeout"`
}

// AppendTable append table
func AppendTable(t Tabler) {
	tables = append(tables, t)
}

// Init init
func (e *Database) Init() {
	if e.Timeout == 0 {
		// set default timeout
		e.Timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(
		context.TODO(),
		e.Timeout)
	defer cancel()

	optFns := make([]func(*config.LoadOptions) error, 0)
	if e.Region != "" {
		optFns = append(optFns, config.WithRegion(e.Region))
	}
	if e.AccessKeyID != "" && e.AccessSecretKey != "" {
		optFns = append(optFns,
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					e.AccessKeyID,
					e.AccessSecretKey, "")))
	}

	defaultConfig, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		log.Fatalf("unable to load SDK config, %s\n", err.Error())
	}
	DB = dynamodb.NewFromConfig(defaultConfig)
	for i := range tables {
		tables[i].Make()
	}
}
