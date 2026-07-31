package response

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

// Default 默认返回
var Default Responses = &response{}

func checkContext(c *gin.Context) {
	if c == nil {
		slog.Error("context is nil, please check, e.g. e.Make(c) add your controller function")
		os.Exit(-1)
	}
}
