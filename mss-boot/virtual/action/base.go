package action

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot/virtual/model"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/17 08:05:15
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/17 08:05:15
 */

var (
	PathKey = "key"
	base    = &Base{}
)

// GetBase get base
func GetBase() *Base {
	return base
}

// SetModel set model
func SetModel(key string, m *model.Model) {
	base.SetModel(key, m)
}

// Base acton
type Base struct {
	Models       map[string]*model.Model
	mutex        sync.Mutex
	TenantIDFunc func(ctx *gin.Context) (any, error)
}

// String string
func (*Base) String() string {
	return "base"
}

// Handler action handler
func (*Base) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusMethodNotAllowed)
	}
	return gin.HandlersChain{h}
}

func (b *Base) SetModel(key string, m *model.Model) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.Models == nil {
		b.Models = make(map[string]*model.Model)
	}
	b.Models[key] = m
}

func (b *Base) GetModel(ctx *gin.Context) *model.Model {
	if b.Models == nil {
		b.Models = make(map[string]*model.Model)
	}
	m, ok := b.Models[ctx.Param(PathKey)]
	if !ok {
		return nil
	}
	return m
}
