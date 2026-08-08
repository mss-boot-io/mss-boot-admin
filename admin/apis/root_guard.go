package apis

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"gorm.io/gorm"
)

// requireRootManagement is a second, handler-adjacent boundary for operations
// that define or assign authority. Legacy Casbin rows may still contain a
// grant for these routes; those grants can never turn a delegated
// administrator into an authority issuer.
func requireRootManagement(c *gin.Context) {
	verify := middleware.GetVerify(c)
	if verify == nil {
		c.Abort()
		response.Make(c).Err(http.StatusUnauthorized)
		return
	}
	if !verify.Root() {
		c.Abort()
		response.Make(c).Err(http.StatusForbidden)
		return
	}
	c.Next()
}

// protectRootUserLifecycle prevents the generic user controller from
// disabling, reassigning, or deleting a root principal. Root identities may
// still manage their own profile through the dedicated self-service endpoint
// and another root may perform the explicit password-reset operation.
func protectRootUserLifecycle(c *gin.Context) {
	// The legacy generic controller shares one Control action for POST and PUT.
	// Creation has no existing target to protect; root-only assignment is
	// enforced by the preceding requireRootManagement handler.
	if c.Request != nil && c.Request.Method == http.MethodPost {
		c.Next()
		return
	}
	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		c.Abort()
		response.Make(c).Err(http.StatusUnprocessableEntity)
		return
	}

	var target models.User
	err := center.Default.GetDB(c, &models.User{}).
		Preload("Role").
		First(&target, "id = ?", userID).Error
	if err != nil {
		c.Abort()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Make(c).Err(http.StatusNotFound)
			return
		}
		api := response.Make(c)
		api.AddError(err).Log.Error("load protected user lifecycle target")
		api.Err(http.StatusInternalServerError)
		return
	}
	if target.Role != nil && target.Role.Root {
		c.Abort()
		response.Make(c).Err(http.StatusForbidden)
		return
	}
	c.Next()
}
