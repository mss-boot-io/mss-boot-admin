package middleware

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var Middlewares = &sync.Map{}

// GetMiddlewares get middlewares by keys
func GetMiddlewares(keys ...string) gin.HandlersChain {
	var mws gin.HandlersChain
	for _, key := range keys {
		if v, ok := Middlewares.Load(key); ok {
			mws = append(mws, v.(gin.HandlerFunc))
		}
	}
	return mws
}
