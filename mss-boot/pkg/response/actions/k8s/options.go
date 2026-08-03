package k8s

import (
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/5/23 17:42:01
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/5/23 17:42:01
 */

type ResourceType string

const (
	Deployment            ResourceType = "deployment"
	Service               ResourceType = "service"
	Pod                   ResourceType = "pod"
	ConfigMap             ResourceType = "configmap"
	Secret                ResourceType = "secret"
	StatefulSet           ResourceType = "statefulset"
	Job                   ResourceType = "job"
	CronJob               ResourceType = "cronjob"
	DaemonSet             ResourceType = "daemonset"
	Ingress               ResourceType = "ingress"
	ResourceQuota         ResourceType = "resourcequota"
	LimitRange            ResourceType = "limitrange"
	PersistentVolume      ResourceType = "persistentvolume"
	PersistentVolumeClaim ResourceType = "persistentvolumeclaim"
	Namespace             ResourceType = "namespace"
	StorageClass          ResourceType = "storageclass"
	IngressClass          ResourceType = "ingressclass"
)

type ActionHook func(ctx *gin.Context, db *kubernetes.Clientset, m any) error

type Option func(*Options)

type Options struct {
	ResourceType    ResourceType
	Model           any
	Handlers        gin.HandlersChain
	Key             string
	BeforeCreate    ActionHook
	AfterCreate     ActionHook
	BeforeUpdate    ActionHook
	AfterUpdate     ActionHook
	BeforeGet       ActionHook
	AfterGet        ActionHook
	BeforeDelete    ActionHook
	AfterDelete     ActionHook
	BeforeSearch    ActionHook
	AfterSearch     ActionHook
	handlers        gin.HandlersChain
	controlHandlers gin.HandlersChain
	getHandlers     gin.HandlersChain
	deleteHandlers  gin.HandlersChain
	searchHandlers  gin.HandlersChain
}

// WithResourceType set resource type
func WithResourceType(rt ResourceType) Option {
	return func(o *Options) {
		o.ResourceType = rt
	}
}

// WithModel set model
func WithModel(m any) Option {
	return func(o *Options) {
		o.Model = m
	}
}

// WithHandlers set handlers
func WithHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		if len(o.Handlers) == 0 {
			o.Handlers = handlers
			return
		}
		o.Handlers = append(o.Handlers, handlers...)
	}
}

// WithKey set key
func WithKey(key string) Option {
	return func(o *Options) {
		o.Key = key
	}
}

// WithBeforeCreate set before create hook
func WithBeforeCreate(hook ActionHook) Option {
	return func(o *Options) {
		o.BeforeCreate = hook
	}
}

// WithAfterCreate set after create hook
func WithAfterCreate(hook ActionHook) Option {
	return func(o *Options) {
		o.AfterCreate = hook
	}
}

// WithBeforeUpdate set before update hook
func WithBeforeUpdate(hook ActionHook) Option {
	return func(o *Options) {
		o.BeforeUpdate = hook
	}
}

// WithAfterUpdate set after update hook
func WithAfterUpdate(hook ActionHook) Option {
	return func(o *Options) {
		o.AfterUpdate = hook
	}
}

// WithBeforeGet set before get hook
func WithBeforeGet(hook ActionHook) Option {
	return func(o *Options) {
		o.BeforeGet = hook
	}
}

// WithAfterGet set after get hook
func WithAfterGet(hook ActionHook) Option {
	return func(o *Options) {
		o.AfterGet = hook
	}
}

// WithBeforeDelete set before delete hook
func WithBeforeDelete(hook ActionHook) Option {
	return func(o *Options) {
		o.BeforeDelete = hook
	}
}

// WithAfterDelete set after delete hook
func WithAfterDelete(hook ActionHook) Option {
	return func(o *Options) {
		o.AfterDelete = hook
	}
}

// WithBeforeSearch set before search hook
func WithBeforeSearch(hook ActionHook) Option {
	return func(o *Options) {
		o.BeforeSearch = hook
	}
}

// WithAfterSearch set after search hook
func WithAfterSearch(hook ActionHook) Option {
	return func(o *Options) {
		o.AfterSearch = hook
	}
}

// WithControlHandlers set control handlers
func WithControlHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.controlHandlers = handlers
	}
}

// WithGetHandlers set get handlers
func WithGetHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.getHandlers = handlers
	}
}

// WithDeleteHandlers set delete handlers
func WithDeleteHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.deleteHandlers = handlers
	}
}

// WithSearchHandlers set search handlers
func WithSearchHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.searchHandlers = handlers
	}
}
