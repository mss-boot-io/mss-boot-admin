package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bootpkg "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
)

// methodNotAllowed keeps legacy mutation URLs side-effect free while clients
// migrate to their POST replacements.
func methodNotAllowed(ctx *gin.Context) {
	message := http.StatusText(http.StatusMethodNotAllowed)
	ctx.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
		"code":         http.StatusMethodNotAllowed,
		"status":       "error",
		"msg":          message,
		"errorMessage": message,
		"traceId":      bootpkg.GenerateMsgIDFromContext(ctx),
	})
}
