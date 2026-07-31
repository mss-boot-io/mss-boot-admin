package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/virtual/action"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/18 09:09:44
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/18 09:09:44
 */

// Virtual controller
type Virtual struct {
	Base
	options  Options
	Provider *action.Base
}

// NewVirtual new virtual controller
func NewVirtual(provider *action.Base, options ...Option) *Virtual {
	v := &Virtual{
		Provider: provider,
		options:  DefaultOptions(),
	}
	for i := range options {
		options[i](&v.options)
	}
	return v
}

func (v *Virtual) GetProvider() fmt.Stringer {
	return v.options.modelProvider
}

// Path http path
func (v *Virtual) Path() string {
	return ":" + action.PathKey
}

// Handlers middlewares
func (v *Virtual) Handlers() gin.HandlersChain {
	if v.options.auth {
		return gin.HandlersChain{response.AuthHandler}
	}
	return nil
}

// GetAction get action
func (v *Virtual) GetAction(key string) response.Action {
	return v.getAction(key)
}

func (v *Virtual) getAction(key string) response.Action {
	switch key {
	case response.Get:
		return action.NewGet(v.Provider)
	case response.Create:
		return action.NewCreate(v.Provider)
	case response.Update:
		return action.NewUpdate(v.Provider)
	case response.Delete:
		return action.NewDelete(v.Provider)
	case response.Search:
		return action.NewSearch(v.Provider)
	default:
		return nil
	}
}
