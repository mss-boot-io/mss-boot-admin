package source

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"k8s.io/client-go/kubernetes"
)

func TestDefaultOptionsAndAllOptionSetters(t *testing.T) {
	options := DefaultOptions()
	if options.Provider != Local || options.Name != "application" || options.Dir != "config" || options.Timeout != 5*time.Second {
		t.Fatalf("default options = %#v", options)
	}

	s3Client := &s3.Client{}
	appConfigClient := &appconfigdata.Client{}
	clientset := &kubernetes.Clientset{}
	filesystem := fstest.MapFS{"application.yaml": &fstest.MapFile{Data: []byte("name: test")}}

	setters := []Option{
		WithPrefixHook(nil),
		WithPostfixHook(nil),
		WithDatasource("consul://127.0.0.1"),
		WithMongoDBURL(""),
		WithMongoDBName("config"),
		WithMongoDBCollection("documents"),
		WithGORMDriver("sqlite"),
		WithGORMDsn("file:test?mode=memory"),
		WithProvider(S3),
		WithDir(`nested\config`),
		WithName(`stage\application`),
		WithProjectName("admin"),
		WithRegion("us-east-1"),
		WithBucket("configuration"),
		WithTimeout(13 * time.Second),
		WithClient(s3Client),
		WithFrom(filesystem),
		WithDriver(nil),
		WithWatch(true),
		WithClientset(clientset),
		WithNamespace("production"),
		WithConfigmap("admin-config"),
		WithKubeconfig("apiVersion: v1"),
		WithKubeconfigPath("/tmp/kubeconfig"),
	}
	for _, setter := range setters {
		setter(options)
	}
	options.APPConfigDataClient = appConfigClient
	options.Extend = SchemeYaml

	if options.Provider != S3 || options.Datasource != "consul://127.0.0.1" {
		t.Fatalf("provider options = %#v", options)
	}
	if options.MongoDBURL != "mongodb://localhost:27017" || options.MongoDBName != "config" || options.MongoDBCollection != "documents" {
		t.Fatalf("MongoDB options = %#v", options)
	}
	if options.GORMDriver != "sqlite" || options.GORMDsn != "file:test?mode=memory" {
		t.Fatalf("GORM options = %#v", options)
	}
	if options.Dir != "nested/config" || options.Name != "stage/application" {
		t.Fatalf("normalized paths dir=%q name=%q", options.Dir, options.Name)
	}
	if options.ProjectName != "admin" || options.Region != "us-east-1" || options.Bucket != "configuration" || options.Timeout != 13*time.Second {
		t.Fatalf("service options = %#v", options)
	}
	if options.S3Client != s3Client || options.APPConfigDataClient != appConfigClient || options.FS == nil {
		t.Fatalf("client options = %#v", options)
	}
	if !options.Watch || options.Clientset != clientset || options.Namespace != "production" || options.Configmap != "admin-config" {
		t.Fatalf("watch/Kubernetes options = %#v", options)
	}
	if options.Kubeconfig != "apiVersion: v1" || options.KubeconfigPath != "/tmp/kubeconfig" || options.GetExtend() != SchemeYaml {
		t.Fatalf("kubeconfig/extension options = %#v", options)
	}
}

func TestMongoDBURLPreservesExplicitValue(t *testing.T) {
	options := &Options{}
	WithMongoDBURL("mongodb://db.example:27017")(options)
	if options.MongoDBURL != "mongodb://db.example:27017" {
		t.Fatalf("MongoDB URL = %q", options.MongoDBURL)
	}
}
