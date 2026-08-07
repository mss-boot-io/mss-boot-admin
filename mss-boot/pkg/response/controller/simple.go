package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kamva/mgm/v3"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/k8s"
	mgmActions "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/mgm"
)

// Simple dispatches standard controller actions to a configured provider.
type Simple struct {
	Base
	options Options
}

// NewSimple creates a controller.
func NewSimple(options ...Option) *Simple {
	s := &Simple{options: DefaultOptions()}
	for _, option := range options {
		if option != nil {
			option(&s.options)
		}
	}
	return s
}

// GetProvider returns the configured model provider.
func (e *Simple) GetProvider() fmt.Stringer {
	return e.options.modelProvider
}

// Path preserves the historical model-derived route for GORM and MongoDB
// controllers. Kubernetes controllers without a database model fall back to
// their resource type instead of passing nil to mgm.CollName.
func (e *Simple) Path() string {
	if e.options.model != nil {
		return normalizePath(mgm.CollName(e.options.model))
	}
	if e.options.modelProvider == actions.ModelProviderK8S {
		return normalizePath(string(e.options.resourceType))
	}
	return ""
}

// Handlers is intentionally empty because authentication can vary by action.
// Common controller middleware is attached by GetAction so it runs exactly once
// and preserves WithNoAuthAction semantics.
func (*Simple) Handlers() gin.HandlersChain {
	return nil
}

// GetAction returns an action with consistent authentication and middleware
// semantics across built-in and custom providers.
func (e *Simple) GetAction(key string) response.Action {
	if action := e.options.getAction(key); action != nil {
		return wrapAction(action, e.actionHandlers(key))
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
	base := mgmActions.Base{Model: e.options.model}
	var action response.Action
	switch key {
	case response.Get:
		action = mgmActions.NewGet(base, e.GetKey())
	case response.Control:
		action = mgmActions.NewControl(base, e.GetKey())
	case response.Delete:
		action = mgmActions.NewDelete(base, e.GetKey())
	case response.Search:
		action = mgmActions.NewSearch(base, e.options.search)
	}
	return wrapAction(action, e.actionHandlers(key))
}

func (e *Simple) getActionGorm(key string) response.Action {
	opts := []gorm.Option{
		gorm.WithModel(e.options.model),
		gorm.WithScope(e.options.scope),
		gorm.WithTreeField(e.options.treeField),
		gorm.WithDepth(e.options.depth),
		gorm.WithHandlers(e.commonHandlers(key)),
		gorm.WithControlHandlers(e.options.createHandlers),
		gorm.WithGetHandlers(e.options.getHandlers),
		gorm.WithDeleteHandlers(e.options.deleteHandlers),
		gorm.WithSearchHandlers(e.options.searchHandlers),
		gorm.WithBeforeGet(e.options.beforeGet),
		gorm.WithAfterGet(e.options.afterGet),
		gorm.WithBeforeCreate(e.options.beforeCreate),
		gorm.WithAfterCreate(e.options.afterCreate),
		gorm.WithAfterCommitCreate(e.options.afterCommitCreate),
		gorm.WithBeforeUpdate(e.options.beforeUpdate),
		gorm.WithAfterUpdate(e.options.afterUpdate),
		gorm.WithBeforeDelete(e.options.beforeDelete),
		gorm.WithAfterDelete(e.options.afterDelete),
		gorm.WithBeforeSearch(e.options.beforeSearch),
		gorm.WithAfterSearch(e.options.afterSearch),
		gorm.WithKey(e.GetKey()),
		gorm.WithSearch(e.options.search),
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
		k8s.WithHandlers(e.commonHandlers(key)),
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

func (e *Simple) commonHandlers(key string) gin.HandlersChain {
	handlers := make(gin.HandlersChain, 0, len(e.options.handlers)+1)
	if e.options.needAuth(key) {
		handlers = append(handlers, configuredAuthHandler())
	}
	handlers = append(handlers, e.options.handlers...)
	return handlers
}

func (e *Simple) actionHandlers(key string) gin.HandlersChain {
	handlers := e.commonHandlers(key)
	switch key {
	case response.Get:
		handlers = append(handlers, e.options.getHandlers...)
	case response.Control:
		handlers = append(handlers, e.options.createHandlers...)
	case response.Delete:
		handlers = append(handlers, e.options.deleteHandlers...)
	case response.Search:
		handlers = append(handlers, e.options.searchHandlers...)
	}
	return handlers
}

func configuredAuthHandler() gin.HandlerFunc {
	if response.AuthHandler != nil {
		return response.AuthHandler
	}
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success":      false,
			"errorMessage": "authentication middleware is not configured",
		})
	}
}

func wrapAction(action response.Action, handlers gin.HandlersChain) response.Action {
	if action == nil || len(handlers) == 0 {
		return action
	}
	return &actionWithHandlers{
		Action:   action,
		handlers: append(gin.HandlersChain(nil), handlers...),
	}
}

type actionWithHandlers struct {
	response.Action
	handlers gin.HandlersChain
}

func (e *actionWithHandlers) Handler() gin.HandlersChain {
	chain := append(gin.HandlersChain(nil), e.handlers...)
	return append(chain, e.Action.Handler()...)
}

func normalizePath(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "_", "-")
	return strings.ToLower(name)
}
