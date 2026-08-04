package response

import "github.com/gin-gonic/gin"

// Default is the process-wide response renderer retained for compatibility.
var Default Responses = &response{}

func checkContext(c *gin.Context) {
	if c == nil {
		panic("response: nil gin context")
	}
}
