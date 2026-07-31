package controller

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/26 11:24:55
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/26 11:24:55
 */

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions/k8s"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"k8s.io/client-go/kubernetes"
)

// Option set options
type Option func(*Options)

// Options options
type Options struct {
	actions        []response.Action
	search         response.Searcher
	model          actions.Model
	auth           bool
	noAuthAction   []string
	depth          int
	treeField      string
	modelProvider  fmt.Stringer
	scope          func(ctx *gin.Context, table schema.Tabler) func(db *gorm.DB) *gorm.DB
	beforeCreate   func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	beforeUpdate   func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	afterCreate    func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	afterUpdate    func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	beforeGet      func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	afterGet       func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	beforeDelete   func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	afterDelete    func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	beforeSearch   func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	afterSearch    func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error
	handlers       gin.HandlersChain
	createHandlers gin.HandlersChain
	updateHandlers gin.HandlersChain
	getHandlers    gin.HandlersChain
	deleteHandlers gin.HandlersChain
	searchHandlers gin.HandlersChain
	// k8s action option
	resourceType         k8s.ResourceType
	resourceModel        any
	resourceBeforeCreate func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceAfterCreate  func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceBeforeUpdate func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceAfterUpdate  func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceBeforeGet    func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceAfterGet     func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceBeforeDelete func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceAfterDelete  func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceBeforeSearch func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
	resourceAfterSearch  func(ctx *gin.Context, db *kubernetes.Clientset, m any) error
}

func (o *Options) needAuth(name string) bool {
	if !o.auth {
		return false
	}
	for i := range o.noAuthAction {
		if o.noAuthAction[i] == name {
			return false
		}
	}
	return true
}

// getAction get action
func (o *Options) getAction(key string) response.Action {
	for i := range o.actions {
		if o.actions[i].String() == key {
			return o.actions[i]
		}
	}
	return nil
}

// DefaultOptions make default options
func DefaultOptions() Options {
	return Options{
		actions:       make([]response.Action, 0),
		modelProvider: actions.ModelProviderGorm,
	}
}

// WithAction set action
func WithAction(action response.Action) Option {
	return func(o *Options) {
		if o.actions == nil {
			o.actions = make([]response.Action, 0)
		}
		o.actions = append(o.actions, action)
	}
}

// WithSearch set search
func WithSearch(search response.Searcher) Option {
	return func(o *Options) {
		o.search = search
	}
}

// WithModel set model
func WithModel(m actions.Model) Option {
	return func(o *Options) {
		o.model = m
	}
}

// WithAuth set auth
func WithAuth(auth bool) Option {
	return func(o *Options) {
		o.auth = auth
	}
}

// WithNoAuthAction set no auth action names
func WithNoAuthAction(names ...string) Option {
	return func(o *Options) {
		o.noAuthAction = names
	}
}

// WithModelProvider set model provider
func WithModelProvider(provider actions.ModelProvider) Option {
	return func(o *Options) {
		o.modelProvider = provider
	}
}

// WithScope set scope
func WithScope(scope func(ctx *gin.Context, table schema.Tabler) func(db *gorm.DB) *gorm.DB) Option {
	return func(o *Options) {
		o.scope = scope
	}
}

// WithDepth set depth
func WithDepth(depth int) Option {
	return func(o *Options) {
		o.depth = depth
	}
}

// WithTreeField set tree field
func WithTreeField(treeField string) Option {
	return func(o *Options) {
		o.treeField = treeField
	}
}

func WithBeforeCreate(beforeCreate func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.beforeCreate = beforeCreate
	}
}

func WithAfterCreate(afterCreate func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.afterCreate = afterCreate
	}
}

func WithBeforeUpdate(beforeUpdate func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.beforeUpdate = beforeUpdate
	}
}

func WithAfterUpdate(afterUpdate func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.afterUpdate = afterUpdate
	}
}

func WithBeforeGet(beforeGet func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.beforeGet = beforeGet
	}
}

func WithAfterGet(afterGet func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.afterGet = afterGet
	}
}

func WithBeforeDelete(beforeDelete func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.beforeDelete = beforeDelete
	}
}

func WithAfterDelete(afterDelete func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.afterDelete = afterDelete
	}
}

func WithBeforeSearch(beforeSearch func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.beforeSearch = beforeSearch
	}
}

func WithAfterSearch(afterSearch func(ctx *gin.Context, db *gorm.DB, m schema.Tabler) error) Option {
	return func(o *Options) {
		o.afterSearch = afterSearch
	}
}

func WithHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.handlers = handlers
	}
}

func WithCreateHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.createHandlers = handlers
	}
}

func WithUpdateHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.updateHandlers = handlers
	}
}

func WithGetHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.getHandlers = handlers
	}
}

func WithDeleteHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.deleteHandlers = handlers
	}
}

func WithSearchHandlers(handlers gin.HandlersChain) Option {
	return func(o *Options) {
		o.searchHandlers = handlers
	}
}

func WithResourceModel(model any) Option {
	return func(o *Options) {
		o.resourceModel = model
	}
}

func WithResourceType(resourceType k8s.ResourceType) Option {
	return func(o *Options) {
		o.resourceType = resourceType
	}
}

func WithResourceBeforeCreate(beforeCreate func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceBeforeCreate = beforeCreate
	}
}

func WithResourceAfterCreate(afterCreate func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceAfterCreate = afterCreate
	}
}

func WithResourceBeforeUpdate(beforeUpdate func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceBeforeUpdate = beforeUpdate
	}
}

func WithResourceAfterUpdate(afterUpdate func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceAfterUpdate = afterUpdate
	}
}

func WithResourceBeforeGet(beforeGet func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceBeforeGet = beforeGet
	}
}

func WithResourceAfterGet(afterGet func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceAfterGet = afterGet
	}
}

func WithResourceBeforeDelete(beforeDelete func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceBeforeDelete = beforeDelete
	}
}

func WithResourceAfterDelete(afterDelete func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceAfterDelete = afterDelete
	}
}

func WithResourceBeforeSearch(beforeSearch func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceBeforeSearch = beforeSearch
	}
}

func WithResourceAfterSearch(afterSearch func(ctx *gin.Context, db *kubernetes.Clientset, m any) error) Option {
	return func(o *Options) {
		o.resourceAfterSearch = afterSearch
	}
}
