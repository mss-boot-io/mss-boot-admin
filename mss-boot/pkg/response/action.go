package response

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/25 20:05:14
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/25 20:05:14
 */

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

const (
	// Get action
	Get = "get"
	// Base action
	Base = "base"
	// Delete action
	Delete = "delete"
	// Search action
	Search = "search"
	// Control action
	Control = "control"
	// Create action
	Create = "create"
	// Update action
	Update = "update"
)

// Action interface
type Action interface {
	fmt.Stringer
	Handler() gin.HandlersChain
}

// Searcher search interface
type Searcher interface {
	GetPage() int64
	GetPageSize() int64
}
