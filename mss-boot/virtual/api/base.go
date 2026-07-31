package api

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/26 01:15:07
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/26 01:15:07
 */

import (
	"github.com/gin-gonic/gin"

	"github.com/mss-boot-io/mss-boot/pkg/response"
)

// DefaultKey default key
var DefaultKey = "id"

// Base controller
type Base struct{}

// Path http path
func (e *Base) Path() string {
	return ""
}

// Handlers middlewares
func (e *Base) Handlers() gin.HandlersChain {
	return nil
}

// GetAction get action
func (e *Base) GetAction(_ string) response.Action {
	return nil
}

// Other handler
func (e *Base) Other(_ *gin.RouterGroup) {}

// GetKey get key
func (e *Base) GetKey() string {
	return DefaultKey
}
