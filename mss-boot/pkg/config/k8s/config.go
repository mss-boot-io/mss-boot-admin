package k8s

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/5/23 18:01:31
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/5/23 18:01:31
 */

var ClientSet *kubernetes.Clientset

type Config struct {
	KubeConfig     string `yaml:"kubeConfig" json:"kubeConfig"`
	KubeConfigPath string `yaml:"kubeConfigPath" json:"kubeConfigPath"`
	*ClusterConfig `yaml:",inline" json:",inline"`
}

type ClusterConfig struct {
	BearerToken     string          `yaml:"bearerToken" json:"bearerToken"`
	TLSClientConfig TLSClientConfig `yaml:"tlsClientConfig" json:"tlsClientConfig"`
}

type TLSClientConfig struct {
	Insecure bool   `yaml:"insecure" json:"insecure"`
	CertData string `yaml:"certData" json:"certData"`
	CertPath string `yaml:"certPath" json:"certPath"`
	KeyData  string `yaml:"keyData" json:"keyData"`
	KeyPath  string `yaml:"keyPath" json:"keyPath"`
	CaData   string `yaml:"caData" json:"caData"`
	CaPath   string `yaml:"caPath" json:"caPath"`
}

func (e *Config) Init() {
	var config *rest.Config
	var err error
	// 获取集群内部的配置
	if Stage() != "local" && e.KubeConfig == "" && e.KubeConfigPath == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			slog.Error("Failed to get in-cluster sectConfig", "err", err)
			os.Exit(-1)
		}
	} else {
		var apiConfig *clientcmdapi.Config
		if e.KubeConfig != "" {
			apiConfig, err = clientcmd.Load([]byte(e.KubeConfig))
			if err != nil {
				slog.Error("Failed to load kubeconfig", "err", err)
				os.Exit(-1)
			}
		} else {
			if e.KubeConfigPath == "" {
				e.KubeConfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
			}
			apiConfig, err = clientcmd.LoadFromFile(e.KubeConfigPath)
			if err != nil {
				slog.Error("Failed to load kubeconfig", "err", err)
				os.Exit(-1)
			}
		}
		// 创建一个 rest.Config 对象
		config, err = clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			slog.Error("Failed to create restConfig", "err", err)
			os.Exit(-1)
		}
	}
	// 创建 Kubernetes 客户端
	ClientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		slog.Error("Failed to create clientset", "err", err)
		os.Exit(-1)
	}
}

// Stage get current stage
func Stage() string {
	stage := os.Getenv("STAGE")
	if stage == "" {
		stage = os.Getenv("stage")
	}
	if stage == "" {
		stage = "local"
	}
	return strings.ToLower(stage)
}
