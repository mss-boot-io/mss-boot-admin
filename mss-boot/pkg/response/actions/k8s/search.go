package k8s

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/k8s"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/5/23 22:04:28
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/5/23 22:04:28
 */

// Search action fixme: not support pagination
type Search struct {
	opts *Options
}

func (*Search) String() string {
	return "search"
}

func NewSearch(opts ...Option) *Search {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return &Search{
		opts: o,
	}
}

func (e *Search) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.opts.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		e.search(c)
	}
	chain := gin.HandlersChain{h}
	if e.opts.searchHandlers != nil {
		chain = append(chain, e.opts.searchHandlers...)
	}
	if e.opts.Handlers != nil {
		chain = append(chain, e.opts.Handlers...)
	}
	return chain
}

func (e *Search) search(c *gin.Context) {
	req := &actions.Pagination{}
	api := response.Make(c).Bind(req)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	m := pkg.DeepCopy(e.opts.Model)
	namespace := c.Param("namespace")
	if namespace == "" {
		api.Err(http.StatusUnprocessableEntity, "namespace is required")
		return
	}
	if e.opts.BeforeSearch != nil {
		if err := e.opts.BeforeSearch(c, k8s.ClientSet, m); err != nil {
			api.AddError(err).Log.Error("BeforeSearch error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	var filter string
	labels, ok := c.GetQueryMap("labels")
	if ok && len(labels) > 0 {
		labelSelector := metav1.LabelSelector{
			MatchLabels: labels,
		}
		filter = metav1.FormatLabelSelector(&labelSelector)
	}
	list, count, err := searchResource(c, e.opts.ResourceType, namespace, filter)
	if err != nil {
		api.AddError(err).Log.Error("searchResource error")
		api.Err(http.StatusInternalServerError)
		return
	}

	if e.opts.AfterSearch != nil {
		if err = e.opts.AfterSearch(c, k8s.ClientSet, m); err != nil {
			api.AddError(err).Log.Error("AfterSearch error")
			api.Err(http.StatusInternalServerError)
			return
		}
	}
	api.PageOK(list, count, 0, 99999999)
}

func searchResource(c context.Context,
	resourceType ResourceType,
	namespace, filter string) (any, int64, error) {
	var count int
	var result any
	listOption := metav1.ListOptions{
		LabelSelector: filter,
	}
	switch resourceType {
	case Deployment:
		list, err := k8s.ClientSet.AppsV1().Deployments(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case StatefulSet:
		list, err := k8s.ClientSet.AppsV1().StatefulSets(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case DaemonSet:
		list, err := k8s.ClientSet.AppsV1().DaemonSets(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case ConfigMap:
		list, err := k8s.ClientSet.CoreV1().ConfigMaps(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case Secret:
		list, err := k8s.ClientSet.CoreV1().Secrets(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case Service:
		list, err := k8s.ClientSet.CoreV1().Services(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case Pod:
		list, err := k8s.ClientSet.CoreV1().Pods(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case Job:
		list, err := k8s.ClientSet.BatchV1().Jobs(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case CronJob:
		list, err := k8s.ClientSet.BatchV1().CronJobs(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case Ingress:
		list, err := k8s.ClientSet.NetworkingV1().Ingresses(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case ResourceQuota:
		list, err := k8s.ClientSet.CoreV1().ResourceQuotas(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case LimitRange:
		list, err := k8s.ClientSet.CoreV1().LimitRanges(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case PersistentVolume:
		list, err := k8s.ClientSet.CoreV1().PersistentVolumes().List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case PersistentVolumeClaim:
		list, err := k8s.ClientSet.CoreV1().PersistentVolumeClaims(namespace).List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case Namespace:
		list, err := k8s.ClientSet.CoreV1().Namespaces().List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case StorageClass:
		list, err := k8s.ClientSet.StorageV1().StorageClasses().List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	case IngressClass:
		list, err := k8s.ClientSet.NetworkingV1().IngressClasses().List(c, listOption)
		if err != nil {
			return nil, 0, err
		}
		count = len(list.Items)
		result = list.Items
	default:
		return nil, 0, errors.New("not support resource type")
	}
	return result, int64(count), nil
}
