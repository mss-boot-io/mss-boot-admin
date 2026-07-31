package k8s

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/k8s"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/5/23 20:47:20
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/5/23 20:47:20
 */

type Delete struct {
	opts *Options
}

// NewDelete new delete action
func NewDelete(opts ...Option) *Delete {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return &Delete{
		opts: o,
	}
}

func (*Delete) String() string {
	return "delete"
}

func (e *Delete) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		api := response.Make(c)
		v := c.Param(e.opts.Key)
		if v == "" {
			api.Err(http.StatusUnprocessableEntity)
			return
		}
		e.delete(c, v)
	}
	chain := gin.HandlersChain{h}
	if e.opts.deleteHandlers != nil {
		chain = append(e.opts.deleteHandlers, chain...)
	}
	if e.opts.Handlers != nil {
		chain = append(e.opts.Handlers, chain...)
	}
	return chain
}

func (e *Delete) delete(c *gin.Context, id string) {
	api := response.Make(c)
	if len(id) == 0 {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	object, err := getResource(c, e.opts.ResourceType, c.Param("namespace"), id)
	if err != nil {
		api.AddError(err).Log.Error("getResource error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if e.opts.BeforeDelete != nil {
		if err = e.opts.BeforeDelete(c, k8s.ClientSet, object); err != nil {
			api.AddError(err).Log.Error("BeforeDelete error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	if err = deleteResource(c, e.opts.ResourceType, c.Param("namespace"), id); err != nil {
		api.AddError(err).Log.Error("deleteResource error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if e.opts.AfterDelete != nil {
		if err = e.opts.AfterDelete(c, k8s.ClientSet, e.opts.Model); err != nil {
			api.AddError(err).Log.Error("AfterDelete error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.OK(nil)
}

func deleteResource(c *gin.Context, resourceType ResourceType, namespace, name string) error {
	switch resourceType {
	case Deployment:
		return k8s.ClientSet.AppsV1().Deployments(namespace).Delete(c, name, metav1.DeleteOptions{})
	case Service:
		return k8s.ClientSet.CoreV1().Services(namespace).Delete(c, name, metav1.DeleteOptions{})
	case StatefulSet:
		return k8s.ClientSet.AppsV1().StatefulSets(namespace).Delete(c, name, metav1.DeleteOptions{})
	case DaemonSet:
		return k8s.ClientSet.AppsV1().DaemonSets(namespace).Delete(c, name, metav1.DeleteOptions{})
	case Job:
		return k8s.ClientSet.BatchV1().Jobs(namespace).Delete(c, name, metav1.DeleteOptions{})
	case CronJob:
		return k8s.ClientSet.BatchV1().CronJobs(namespace).Delete(c, name, metav1.DeleteOptions{})
	case Pod:
		return k8s.ClientSet.CoreV1().Pods(namespace).Delete(c, name, metav1.DeleteOptions{})
	case ConfigMap:
		return k8s.ClientSet.CoreV1().ConfigMaps(namespace).Delete(c, name, metav1.DeleteOptions{})
	case Secret:
		return k8s.ClientSet.CoreV1().Secrets(namespace).Delete(c, name, metav1.DeleteOptions{})
	case Ingress:
		return k8s.ClientSet.NetworkingV1().Ingresses(namespace).Delete(c, name, metav1.DeleteOptions{})
	case ResourceQuota:
		return k8s.ClientSet.CoreV1().ResourceQuotas(namespace).Delete(c, name, metav1.DeleteOptions{})
	case LimitRange:
		return k8s.ClientSet.CoreV1().LimitRanges(namespace).Delete(c, name, metav1.DeleteOptions{})
	case PersistentVolume:
		return k8s.ClientSet.CoreV1().PersistentVolumes().Delete(c, name, metav1.DeleteOptions{})
	case PersistentVolumeClaim:
		return k8s.ClientSet.CoreV1().PersistentVolumeClaims(namespace).Delete(c, name, metav1.DeleteOptions{})
	case Namespace:
		return k8s.ClientSet.CoreV1().Namespaces().Delete(c, name, metav1.DeleteOptions{})
	case StorageClass:
		return k8s.ClientSet.StorageV1().StorageClasses().Delete(c, name, metav1.DeleteOptions{})
	case IngressClass:
		return k8s.ClientSet.NetworkingV1().IngressClasses().Delete(c, name, metav1.DeleteOptions{})
	}
	return errors.New("not support resource type")
}
