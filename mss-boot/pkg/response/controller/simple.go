package controller

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/26 01:22:21
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/26 01:22:21
 */

import (
	"fmt"
	"strings"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions/k8s"

	"github.com/gin-gonic/gin"
	"github.com/kamva/mgm/v3"

	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions/gorm"
	mgmActions "github.com/mss-boot-io/mss-boot/pkg/response/actions/mgm"
)

// Simple controller
type Simple struct {
	Base
	options Options
}

// NewSimple new simple
func NewSimple(options ...Option) *Simple {
	s := &Simple{
		options: DefaultOptions(),
	}
	for i := range options {
		options[i](&s.options)
	}
	return s
}

// GetProvider get provider
func (e *Simple) GetProvider() fmt.Stringer {
	return e.options.modelProvider
}

// Path route path
func (e *Simple) Path() string {
	if e.options.model == nil {
		return ""
	}
	return strings.ReplaceAll(strings.ToLower(mgm.CollName(e.options.model)), "_", "-")
}

// Handlers return handlers
func (e *Simple) Handlers() gin.HandlersChain {
	return nil
}

// GetAction get action
func (e *Simple) GetAction(key string) response.Action {
	if action := e.options.getAction(key); action != nil {
		return action
	}
	switch e.options.modelProvider {
	case actions.ModelProviderMgm:
		return e.getActionMgm(key)
	case actions.ModelProviderGorm:
		return e.getActionGorm(key)
	case actions.ModelProviderK8S:
		return e.getActionK8S(key)
	default:
		return nil
	}
}

func (e *Simple) getActionMgm(key string) response.Action {
	b := mgmActions.Base{
		Model: e.options.model,
	}
	switch key {
	case response.Get:
		return mgmActions.NewGet(b, e.GetKey())
	case response.Control:
		return mgmActions.NewControl(b, e.GetKey())
	case response.Delete:
		return mgmActions.NewDelete(b, e.GetKey())
	case response.Search:
		return mgmActions.NewSearch(b, e.options.search)
	default:
		return nil
	}
}

func (e *Simple) getActionGorm(key string) response.Action {
	opts := []gorm.Option{
		gorm.WithModel(e.options.model),
		gorm.WithScope(e.options.scope),
		gorm.WithTreeField(e.options.treeField),
		gorm.WithDepth(e.options.depth),
		gorm.WithHandlers(e.options.handlers),
		gorm.WithControlHandlers(e.options.createHandlers),
		gorm.WithGetHandlers(e.options.getHandlers),
		gorm.WithDeleteHandlers(e.options.deleteHandlers),
		gorm.WithSearchHandlers(e.options.searchHandlers),
		gorm.WithBeforeGet(e.options.beforeGet),
		gorm.WithAfterGet(e.options.afterGet),
		gorm.WithBeforeCreate(e.options.beforeCreate),
		gorm.WithAfterCreate(e.options.afterCreate),
		gorm.WithBeforeUpdate(e.options.beforeUpdate),
		gorm.WithAfterUpdate(e.options.afterUpdate),
		gorm.WithBeforeDelete(e.options.beforeDelete),
		gorm.WithAfterDelete(e.options.afterDelete),
		gorm.WithBeforeSearch(e.options.beforeSearch),
		gorm.WithAfterSearch(e.options.afterSearch),
		gorm.WithKey(e.GetKey()),
		gorm.WithSearch(e.options.search),
	}
	if e.options.needAuth(key) {
		opts = append(opts, gorm.WithHandlers(gin.HandlersChain{response.AuthHandler}))
	}
	switch key {
	case response.Get:
		return gorm.NewGet(opts...)
	case response.Control:
		return gorm.NewControl(opts...)
	case response.Delete:
		return gorm.NewDelete(opts...)
	case response.Search:
		return gorm.NewSearch(opts...)
	default:
		return nil
	}
}

func (e *Simple) getActionK8S(key string) response.Action {
	opts := []k8s.Option{
		k8s.WithModel(e.options.resourceModel),
		k8s.WithResourceType(e.options.resourceType),
		k8s.WithHandlers(e.options.handlers),
		k8s.WithControlHandlers(e.options.createHandlers),
		k8s.WithGetHandlers(e.options.getHandlers),
		k8s.WithDeleteHandlers(e.options.deleteHandlers),
		k8s.WithSearchHandlers(e.options.searchHandlers),
		k8s.WithBeforeGet(e.options.resourceBeforeGet),
		k8s.WithAfterGet(e.options.resourceAfterGet),
		k8s.WithBeforeCreate(e.options.resourceBeforeCreate),
		k8s.WithAfterCreate(e.options.resourceAfterCreate),
		k8s.WithBeforeUpdate(e.options.resourceBeforeUpdate),
		k8s.WithAfterUpdate(e.options.resourceAfterUpdate),
		k8s.WithBeforeDelete(e.options.resourceBeforeDelete),
		k8s.WithAfterDelete(e.options.resourceAfterDelete),
		k8s.WithBeforeSearch(e.options.resourceBeforeSearch),
		k8s.WithAfterSearch(e.options.resourceAfterSearch),
		k8s.WithKey(e.GetKey()),
	}
	if e.options.needAuth(key) {
		opts = append(opts, k8s.WithHandlers(gin.HandlersChain{response.AuthHandler}))
	}
	switch key {
	case response.Get:
		return k8s.NewGet(opts...)
	case response.Control:
		return k8s.NewControl(opts...)
	case response.Delete:
		return k8s.NewDelete(opts...)
	case response.Search:
		return k8s.NewSearch(opts...)
	default:
		return nil
	}
}
