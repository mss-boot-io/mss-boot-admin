package k8s

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/k8s"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/5/23 20:53:03
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/5/23 20:53:03
 */

type Get struct {
	opts *Options
}

func (*Get) String() string {
	return "get"
}

func NewGet(opts ...Option) *Get {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return &Get{
		opts: o,
	}
}

func (e *Get) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		e.get(c, e.opts.Key)
	}
	chain := gin.HandlersChain{h}
	if e.opts.getHandlers != nil {
		chain = append(e.opts.getHandlers, chain...)
	}
	if e.opts.Handlers != nil {
		chain = append(e.opts.handlers, chain...)
	}
	return chain
}

func (e *Get) get(c *gin.Context, key string) {
	api := response.Make(c)
	m := pkg.DeepCopy(e.opts.Model)
	namespace := c.Param("namespace")
	name := c.Param(key)

	if e.opts.BeforeGet != nil {
		if err := e.opts.BeforeGet(c, k8s.ClientSet, m); err != nil {
			api.AddError(err).Log.Error("BeforeGet error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	object, err := getResource(c, e.opts.ResourceType, namespace, name)
	if err != nil {
		if status, ok := err.(errors.APIStatus); ok {
			if status.Status().Reason != metav1.StatusReasonNotFound {
				api.Err(http.StatusNotFound)
			}
		}
		api.AddError(err).Log.Error("getResource error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if e.opts.AfterGet != nil {
		if err = e.opts.AfterGet(c, k8s.ClientSet, object); err != nil {
			api.AddError(err).Log.Error("AfterGet error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.OK(object)

}

func getResource(ctx context.Context, resourceType ResourceType, namespace, name string) (any, error) {
	switch resourceType {
	case Deployment:
		return k8s.ClientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	case StatefulSet:
		return k8s.ClientSet.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	case DaemonSet:
		return k8s.ClientSet.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	case Job:
		return k8s.ClientSet.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	case CronJob:
		return k8s.ClientSet.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	case Pod:
		return k8s.ClientSet.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	case Service:
		return k8s.ClientSet.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	case ConfigMap:
		return k8s.ClientSet.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	case Secret:
		return k8s.ClientSet.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	case Ingress:
		return k8s.ClientSet.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	case ResourceQuota:
		return k8s.ClientSet.CoreV1().ResourceQuotas(namespace).Get(ctx, name, metav1.GetOptions{})
	case LimitRange:
		return k8s.ClientSet.CoreV1().LimitRanges(namespace).Get(ctx, name, metav1.GetOptions{})
	case PersistentVolume:
		return k8s.ClientSet.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
	case PersistentVolumeClaim:
		return k8s.ClientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	case Namespace:
		return k8s.ClientSet.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	case StorageClass:
		return k8s.ClientSet.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	case IngressClass:
		return k8s.ClientSet.NetworkingV1().IngressClasses().Get(ctx, name, metav1.GetOptions{})
	}
	return nil, fmt.Errorf("not support resource type")
}
