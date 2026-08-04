package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/spf13/cast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/4/24 22:31:57
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/4/24 22:31:57
 */

type Secret struct {
	EnvPrefix string            `yaml:"envPrefix" json:"envPrefix"`
	Keys      []string          `yaml:"keys" json:"keys"`
	AWS       *AWSSecretManager `yaml:"aws" json:"aws"`
	K8S       *K8SSecretManager `yaml:"k8s" json:"k8s"`
}

type AWSSecretManager struct {
	SecretName      string `yaml:"secretName" json:"secretName"`
	Region          string `yaml:"region" json:"region"`
	AccessKeyID     string `yaml:"accessKeyID" json:"accessKeyID"`
	SecretAccessKey string `yaml:"secretAccessKey" json:"secretAccessKey"`
	VersionStage    string `yaml:"versionStage" json:"versionStage"`
	client          *secretsmanager.Client
}

type K8SSecretManager struct {
	Namespace      string `yaml:"namespace" json:"namespace"`
	SecretName     string `yaml:"secretName" json:"secretName"`
	KubeConfig     string `yaml:"kubeConfig" json:"kubeConfig"`
	KubeConfigPath string `yaml:"kubeConfigPath" json:"kubeConfigPath"`
	clientSet      *kubernetes.Clientset
}

func (s *Secret) Init() {
	if s.K8S != nil {
		var err error
		var config *rest.Config
		if s.K8S.KubeConfig == "" && s.K8S.KubeConfigPath == "" {
			// inCluster
			config, err = rest.InClusterConfig()
			if err != nil {
				slog.Error("cfg init failed", slog.Any("err", err))
				os.Exit(-1)
			}
		} else {
			var apiConfig *clientcmdapi.Config
			// outCluster
			if s.K8S.KubeConfig != "" {
				apiConfig, err = clientcmd.Load([]byte(s.K8S.KubeConfig))
			} else {
				if s.K8S.KubeConfigPath == "" {
					s.K8S.KubeConfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
				}
				apiConfig, err = clientcmd.LoadFromFile(s.K8S.KubeConfigPath)
			}
			if err != nil {
				slog.Error("cfg init failed", slog.Any("err", err))
				os.Exit(-1)
			}
			config, err = clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
			if err != nil {
				slog.Error("cfg init failed", slog.Any("err", err))
				os.Exit(-1)
			}
		}
		s.K8S.clientSet, err = kubernetes.NewForConfig(config)
		if err != nil {
			slog.Error("cfg init failed", slog.Any("err", err))
			os.Exit(-1)
		}
		if s.K8S.Namespace == "" {
			s.K8S.Namespace = "default"
		}
		secret, err := s.K8S.clientSet.CoreV1().Secrets(s.K8S.Namespace).
			Get(context.TODO(), s.K8S.SecretName, metav1.GetOptions{})
		if err != nil {
			slog.Error("cfg init failed", slog.Any("err", err))
			os.Exit(-1)
		}
		for i := range s.Keys {
			if v, ok := secret.Data[s.Keys[i]]; ok {
				err = s.set(s.Keys[i], v)
				if err != nil {
					slog.Error("cfg init failed", slog.Any("err", err))
					os.Exit(-1)
				}
			}
		}
		return
	}
	if s.AWS != nil {
		if s.EnvPrefix == "" {
			s.EnvPrefix = "aws_"
		}
		if s.AWS.Region == "" || s.AWS.AccessKeyID == "" || s.AWS.SecretAccessKey == "" {

			opts := make([]func(*awsConfig.LoadOptions) error, 0)
			if s.AWS.Region != "" {
				opts = append(opts, awsConfig.WithRegion(s.AWS.Region))
			}
			config, err := awsConfig.LoadDefaultConfig(context.TODO(), opts...)
			if err != nil {
				slog.Error("cfg init failed", slog.Any("err", err))
				os.Exit(-1)
			}
			s.AWS.client = secretsmanager.NewFromConfig(config)
		} else {
			s.AWS.client = secretsmanager.New(secretsmanager.Options{
				Region: s.AWS.Region,
				Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     s.AWS.AccessKeyID,
						SecretAccessKey: s.AWS.SecretAccessKey,
					}, nil
				}),
			})
		}
		if s.AWS.VersionStage == "" {
			s.AWS.VersionStage = "AWSCURRENT"
		}
		result, err := s.AWS.client.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
			SecretId:     aws.String(s.AWS.SecretName),
			VersionStage: aws.String(s.AWS.VersionStage),
		})
		if err != nil {
			slog.Error("cfg init failed", slog.Any("err", err))
			os.Exit(-1)
		}
		data := make(map[string]any)
		err = json.Unmarshal([]byte(*result.SecretString), &data)
		if err != nil {
			slog.Error("cfg init failed", slog.Any("err", err))
			os.Exit(-1)
		}
		for i := range s.Keys {
			if v, ok := data[s.Keys[i]]; ok {
				err = s.set(s.Keys[i], v)
				if err != nil {
					slog.Error("cfg init failed", slog.Any("err", err))
					os.Exit(-1)
				}
			}
		}
		return
	}
}

func (s *Secret) set(key string, v any) error {
	key = fmt.Sprintf("%s%s", s.EnvPrefix, key)
	key = strings.ToLower(key)
	err := os.Setenv(key, cast.ToString(v))
	if err != nil {
		return err
	}
	return os.Setenv(strings.ToUpper(key), cast.ToString(v))
}

// Get value by key
func (s *Secret) Get(key string) (any, error) {
	if s.AWS != nil {
		result, err := s.AWS.client.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
			SecretId:     aws.String(s.AWS.SecretName),
			VersionStage: aws.String(s.AWS.VersionStage),
		})
		if err != nil {
			return nil, err
		}
		data := make(map[string]any)
		err = json.Unmarshal([]byte(*result.SecretString), &data)
		if err != nil {
			return nil, err
		}
		if v, ok := data[key]; ok {
			_ = s.set(key, v)
			return v, nil
		}
		return nil, nil
	}
	if s.K8S != nil {
		secret, err := s.K8S.clientSet.CoreV1().Secrets(s.K8S.Namespace).
			Get(context.TODO(), s.K8S.SecretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if v, ok := secret.Data[key]; ok {
			_ = s.set(key, v)
			return v, nil
		}
		return nil, nil
	}
	return nil, nil
}
