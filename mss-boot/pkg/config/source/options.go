package source

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/21 18:31:13
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/21 18:31:13
 */

import (
	"io/fs"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"k8s.io/client-go/kubernetes"
)

// Provider provider
type Provider string

const (
	// FS fs
	FS Provider = "fs"
	// Local file
	Local Provider = "local"
	// S3 s3
	S3 Provider = "s3"
	// MGDB mongodb
	MGDB Provider = "mgdb"
	// GORM gorm
	GORM Provider = "gorm"
	// ConfigMap k8s configmap
	ConfigMap Provider = "configmap"
	// Consul consul
	Consul Provider = "consul"
	// APPConfig aws appconfig
	APPConfig Provider = "appconfig"
)

// Extends extends
var Extends = []Scheme{SchemeYaml, SchemeYml, SchemeJSOM}

// Sourcer source interface
type Sourcer interface {
	fs.ReadFileFS
	GetExtend() Scheme
	Watch(e Entity, unm func([]byte, any) error) error
}

// Option option
type Option func(*Options)

// Options options
type Options struct {
	Provider            Provider
	Driver              Driver
	Name                string
	Extend              Scheme
	Dir                 string
	Region              string
	Bucket              string
	ProjectName         string
	Timeout             time.Duration
	S3Client            *s3.Client
	APPConfigDataClient *appconfigdata.Client
	FS                  fs.ReadFileFS
	MongoDBURL          string
	MongoDBName         string
	MongoDBCollection   string
	Datasource          string
	GORMDriver          string
	GORMDsn             string
	Watch               bool
	Namespace           string
	Configmap           string
	PrefixHook          PrefixHook
	PostfixHook         PostHook
	Clientset           *kubernetes.Clientset
	Kubeconfig          string
	KubeconfigPath      string
}

func (o *Options) GetExtend() Scheme {
	return o.Extend
}

// DefaultOptions default options
func DefaultOptions() *Options {
	return &Options{
		Provider: Local,
		Name:     "application",
		Dir:      "config",
		Timeout:  5 * time.Second,
	}
}

// WithPrefixHook set prefix hook
func WithPrefixHook(hook PrefixHook) Option {
	return func(args *Options) {
		args.PrefixHook = hook
	}
}

func WithPostfixHook(hook PostHook) Option {
	return func(args *Options) {
		args.PostfixHook = hook
	}
}

// WithDatasource set datasource
func WithDatasource(datasource string) Option {
	return func(args *Options) {
		args.Datasource = datasource
	}
}

// WithMongoDBURL set mongodb url
func WithMongoDBURL(url string) Option {
	return func(args *Options) {
		if url == "" {
			url = "mongodb://localhost:27017"
		}
		args.MongoDBURL = url
	}
}

// WithMongoDBName set mongodb name
func WithMongoDBName(name string) Option {
	return func(args *Options) {
		args.MongoDBName = name
	}
}

// WithMongoDBCollection set mongodb collection
func WithMongoDBCollection(collection string) Option {
	return func(args *Options) {
		args.MongoDBCollection = collection
	}
}

func WithGORMDriver(driver string) Option {
	return func(args *Options) {
		args.GORMDriver = driver
	}
}

func WithGORMDsn(dsn string) Option {
	return func(args *Options) {
		args.GORMDsn = dsn
	}
}

// WithProvider set provider
func WithProvider(provider Provider) Option {
	return func(args *Options) {
		args.Provider = provider
	}
}

// WithDir set dir
func WithDir(dir string) Option {
	return func(args *Options) {
		args.Dir = strings.ReplaceAll(dir, "\\", "/")
	}
}

// WithName set config name
func WithName(file string) Option {
	return func(args *Options) {
		args.Name = strings.ReplaceAll(file, "\\", "/")
	}
}

// WithProjectName set projectName
func WithProjectName(projectName string) Option {
	return func(args *Options) {
		args.ProjectName = projectName
	}
}

// WithRegion set s3 region
func WithRegion(region string) Option {
	return func(args *Options) {
		args.Region = region
	}
}

// WithBucket set s3 bucket
func WithBucket(bucket string) Option {
	return func(args *Options) {
		args.Bucket = bucket
	}
}

// WithTimeout set s3 client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(args *Options) {
		args.Timeout = timeout
	}
}

// WithClient set s3 client
func WithClient(client *s3.Client) Option {
	return func(args *Options) {
		args.S3Client = client
	}
}

// WithFrom set embed.FS
func WithFrom(fs fs.ReadFileFS) Option {
	return func(args *Options) {
		args.FS = fs
	}
}

// WithDriver set driver
func WithDriver(driver Driver) Option {
	return func(args *Options) {
		args.Driver = driver
	}
}

func WithWatch(watch bool) Option {
	return func(args *Options) {
		args.Watch = watch
	}
}

// WithClientset set k8s clientset
func WithClientset(clientset *kubernetes.Clientset) Option {
	return func(args *Options) {
		args.Clientset = clientset
	}
}

// WithNamespace set k8s namespace
func WithNamespace(namespace string) Option {
	return func(args *Options) {
		args.Namespace = namespace
	}
}

// WithConfigmap set k8s configmap name
func WithConfigmap(configmap string) Option {
	return func(args *Options) {
		args.Configmap = configmap
	}
}

// WithKubeconfig set k8s kubeconfig
func WithKubeconfig(kubeconfig string) Option {
	return func(args *Options) {
		args.Kubeconfig = kubeconfig
	}
}

// WithKubeconfigPath set k8s kubeconfig path
func WithKubeconfigPath(kubeconfigPath string) Option {
	return func(args *Options) {
		args.KubeconfigPath = kubeconfigPath
	}
}
