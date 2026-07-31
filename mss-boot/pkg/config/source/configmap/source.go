package configmap

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/4/26 17:19:21
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/4/26 17:19:21
 */

// Source is a k8s configmap source
type Source struct {
	opt *source.Options
}

func (s *Source) GetExtend() source.Scheme {
	return s.opt.Extend
}

// Open a file for reading
func (s *Source) Open(_ string) (fs.File, error) {
	return nil, errors.New("method Get not implemented")
}

// ReadFile read file
func (s *Source) ReadFile(name string) (rb []byte, err error) {
	ctx, cancel := context.WithTimeout(context.TODO(), s.opt.Timeout)
	defer cancel()
	cm, err := s.opt.Clientset.CoreV1().
		ConfigMaps(s.opt.Namespace).
		Get(ctx, s.opt.Configmap, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for i := range source.Extends {
		if v, ok := cm.Data[name+"."+string(source.Extends[i])]; ok {
			s.opt.Extend = source.Extends[i]
			return []byte(v), nil
		}
	}
	return nil, err
}

// Watch configmap
func (s *Source) Watch(c source.Entity, unm func([]byte, any) error) error {
	watcher, err := s.opt.Clientset.CoreV1().
		ConfigMaps(s.opt.Namespace).
		Watch(context.TODO(), metav1.ListOptions{
			FieldSelector: "metadata.name=" + s.opt.Configmap,
		})
	if err != nil {
		return err
	}
	go func(sc *Source, cfg source.Entity, w watch.Interface, decoder func([]byte, any) error) {
		defer w.Stop()
		for event := range w.ResultChan() {
			if event.Type == watch.Modified {
				cm, ok := event.Object.(*corev1.ConfigMap)
				if !ok {
					continue
				}
				for i := range source.Extends {
					if v, ok := cm.Data[sc.opt.Name+"."+string(source.Extends[i])]; ok {
						if err = decoder([]byte(v), cfg); err != nil {
							slog.Error("Failed to decode config", slog.Any("error", err))
							continue
						}
					}
					if v, ok := cm.Data[sc.opt.Name+"-"+pkg.GetStage()+"."+string(source.Extends[i])]; ok {
						if err = decoder([]byte(v), cfg); err != nil {
							slog.Error("Failed to decode config", slog.Any("error", err))
						}
					}
				}
			}
			cfg.OnChange()
		}
	}(s, c, watcher, unm)
	return nil
}

func (s *Source) getClientset() error {
	if s.opt.Clientset == nil {
		var err error
		var config *rest.Config
		if s.opt.KubeconfigPath == "" && s.opt.Kubeconfig == "" {
			config, err = rest.InClusterConfig()
			if err != nil {
				return err
			}
		} else {
			var apiConfig *clientcmdapi.Config
			if s.opt.Kubeconfig != "" {
				apiConfig, err = clientcmd.Load([]byte(s.opt.Kubeconfig))
				if err != nil {
					return err
				}
			} else {
				if s.opt.KubeconfigPath == "" {
					s.opt.KubeconfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
				}
				apiConfig, err = clientcmd.LoadFromFile(s.opt.KubeconfigPath)
				if err != nil {
					return err
				}
			}
			config, err = clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
			if err != nil {
				return err
			}
		}
		s.opt.Clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
	}
	return nil
}

func New(options ...source.Option) (*Source, error) {
	s := &Source{
		opt: source.DefaultOptions(),
	}
	for _, opt := range options {
		opt(s.opt)
	}
	err := s.getClientset()
	if err != nil {
		return nil, err
	}
	if s.opt.Namespace == "" {
		s.opt.Namespace = "default"
	}
	return s, nil
}
