package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"

	"github.com/mss-boot-io/mss-boot-admin/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/8/22 16:59:33
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/8/22 16:59:33
 */

type Clusters []*Cluster

func (e Clusters) Init() {
	for i := range e {
		e[i].Init()
	}
}

func (e Clusters) GetDynamicClient(name string) *dynamic.DynamicClient {
	for i := range e {
		if e[i].Name == name {
			return e[i].GetDynamicClient()
		}
	}
	return nil
}

func (e Clusters) GetClientSet(name string) *kubernetes.Clientset {
	for i := range e {
		if e[i].Name == name {
			return e[i].GetClientSet()
		}
	}
	return nil
}

func (e Clusters) GetConfig(name string) *rest.Config {
	for i := range e {
		if e[i].Name == name {
			return e[i].GetConfig()
		}
	}
	return nil
}

type Cluster struct {
	Name           string      `yaml:"name" json:"name"`
	KubeConfig     string      `yaml:"kubeConfig" json:"kubeConfig"`
	KubeConfigPath string      `yaml:"kubeConfigPath" json:"kubeConfigPath"`
	EKS            *EKSCluster `yaml:"eks" json:"eks"`
	clientSet      *kubernetes.Clientset
	config         *rest.Config
	dynamicClient  *dynamic.DynamicClient
}

func (e *Cluster) GetDynamicClient() *dynamic.DynamicClient {
	return e.dynamicClient
}

func (e *Cluster) GetClientSet() *kubernetes.Clientset {
	return e.clientSet
}

func (e *Cluster) GetConfig() *rest.Config {
	return e.config
}

func (e *Cluster) Init() {
	var err error
	if e.EKS != nil {
		e.KubeConfig, err = e.EKS.GetKubeconfig()
		if err != nil {
			slog.Error("Failed to get kubeconfig", "err", err)
			os.Exit(-1)
		}
		if e.Name == "" {
			e.Name = e.EKS.Name
		}
	}
	if e.KubeConfigPath == "" && e.KubeConfig == "" {
		e.config, err = rest.InClusterConfig()
		if err != nil {
			slog.Error("Failed to get in cluster config", "err", err)
			os.Exit(-1)
		}
	} else {
		var apiConfig *clientcmdapi.Config
		if e.KubeConfig != "" {
			apiConfig, err = clientcmd.Load([]byte(e.KubeConfig))
			if err != nil {
				slog.Error("Failed to load kube config", "err", err)
				os.Exit(-1)
			}
		} else {
			if e.KubeConfigPath == "" {
				e.KubeConfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
			}
			apiConfig, err = clientcmd.LoadFromFile(e.KubeConfigPath)
			if err != nil {
				slog.Error("Failed to load kube config", "err", err)
				os.Exit(-1)
			}
		}
		// 创建一个 rest.Config 对象
		e.config, err = clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			slog.Error("Failed to create rest config", "err", err)
			os.Exit(-1)
		}
	}
	e.clientSet, err = kubernetes.NewForConfig(e.config)
	if err != nil {
		slog.Error("Failed to create clientset", "err", err)
		os.Exit(-1)
	}
	e.dynamicClient, err = dynamic.NewForConfig(e.config)
	if err != nil {
		slog.Error("Failed to create dynamic client", "err", err)
		os.Exit(-1)
	}
}

type EKSCluster struct {
	Name   string `yaml:"name" json:"name"`
	Region string `yaml:"region" json:"region"`
}

func (e *EKSCluster) GetKubeconfig() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(e.Region))
	if err != nil {
		return "", err
	}

	svc := eks.NewFromConfig(cfg)

	return pkg.FetchEKSKubeconfig(ctx, svc, e.Name)
}
