package litellmops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
)

var (
	ErrAuthenticationRequired   = errors.New("litellmops authentication required")
	ErrAuthorizationDenied      = errors.New("litellmops authorization denied")
	ErrAuthorizationUnavailable = errors.New("litellmops authorization unavailable")
)

type authorizationRoute struct {
	method string
	path   string
}

var authorizationRoutes = map[string][]authorizationRoute{
	PermissionUserList: {{method: "GET", path: "/admin/api/litellmops/users"}},
	PermissionUserRead: {
		{method: "GET", path: "/admin/api/litellmops/users/:id"},
		{method: "GET", path: "/admin/api/litellmops/users/:id/recharges"},
	},
	PermissionKeyList:  {{method: "GET", path: "/admin/api/litellmops/keys"}},
	PermissionKeyRead:  {{method: "GET", path: "/admin/api/litellmops/keys/:id"}},
	PermissionSync:     {{method: "POST", path: "/admin/api/litellmops/sync"}},
	PermissionBills:    {{method: "GET", path: "/admin/api/litellmops/bills"}},
	PermissionRecharge: {{method: "POST", path: "/admin/api/litellmops/users/:id/recharge"}},
	PermissionOrgList:  {{method: "GET", path: "/admin/api/litellmops/organizations"}},
	PermissionOrgRead:  {{method: "GET", path: "/admin/api/litellmops/organizations/:id"}},
	PermissionOrgWrite: {
		{method: "POST", path: "/admin/api/litellmops/organizations"},
		{method: "PATCH", path: "/admin/api/litellmops/organizations/:id"},
		{method: "DELETE", path: "/admin/api/litellmops/organizations/:id"},
		{method: "POST", path: "/admin/api/litellmops/organizations/:id/recharge"},
		{method: "POST", path: "/admin/api/litellmops/organizations/:id/members"},
		{method: "POST", path: "/admin/api/litellmops/organizations/:id/members/remove"},
		{method: "POST", path: "/admin/api/litellmops/organizations/:id/teams"},
		{method: "POST", path: "/admin/api/litellmops/teams/:teamId/recharge"},
		{method: "DELETE", path: "/admin/api/litellmops/teams/:teamId"},
	},
	PermissionProductRead: {{method: "GET", path: "/admin/api/litellmops/sales/products"}},
	PermissionProductWrite: {
		{method: "POST", path: "/admin/api/litellmops/sales/products"},
		{method: "PATCH", path: "/admin/api/litellmops/sales/products/:id"},
	},
	PermissionOrderRead: {
		{method: "GET", path: "/admin/api/litellmops/sales/orders"},
		{method: "GET", path: "/admin/api/litellmops/sales/orders/:id"},
	},
	PermissionOrderImport: {
		{method: "POST", path: "/admin/api/litellmops/sales/orders"},
		{method: "POST", path: "/admin/api/litellmops/sales/orders/import"},
	},
	PermissionOrderVerify: {
		{method: "POST", path: "/admin/api/litellmops/sales/orders/:id/verify"},
		{method: "POST", path: "/admin/api/litellmops/sales/orders/:id/match"},
	},
	PermissionOrderApprove:      {{method: "POST", path: "/admin/api/litellmops/sales/orders/:id/approve"}},
	PermissionOrderExecute:      {{method: "POST", path: "/admin/api/litellmops/sales/orders/:id/execute"}},
	PermissionOrderReconcile:    {{method: "POST", path: "/admin/api/litellmops/sales/orders/:id/reconcile"}},
	PermissionOrderRefundReview: {{method: "POST", path: "/admin/api/litellmops/sales/orders/:id/refund-review"}},
	PermissionUserWrite: {
		{method: "POST", path: "/admin/api/litellmops/users"},
		{method: "POST", path: "/admin/api/litellmops/users/invite"},
		{method: "PATCH", path: "/admin/api/litellmops/users/:id"},
		{method: "POST", path: "/admin/api/litellmops/users/:id/block"},
		{method: "POST", path: "/admin/api/litellmops/users/:id/unblock"},
		{method: "DELETE", path: "/admin/api/litellmops/users/:id"},
	},
	PermissionKeyIssue: {
		{method: "POST", path: "/admin/api/litellmops/keys"},
		{method: "POST", path: "/admin/api/litellmops/keys/:id/rotate"},
	},
	PermissionKeyWrite: {
		{method: "PATCH", path: "/admin/api/litellmops/keys/:id"},
		{method: "POST", path: "/admin/api/litellmops/keys/:id/reset-spend"},
	},
	PermissionKeyRevoke: {
		{method: "POST", path: "/admin/api/litellmops/keys/:id/block"},
		{method: "POST", path: "/admin/api/litellmops/keys/:id/unblock"},
		{method: "DELETE", path: "/admin/api/litellmops/keys/:id"},
	},
	PermissionGatewayRead: {
		{method: "GET", path: "/admin/api/litellmops/gateway/models"},
		{method: "GET", path: "/admin/api/litellmops/gateway/health"},
	},
	PermissionGatewayWrite: {
		{method: "POST", path: "/admin/api/litellmops/gateway/models/:id/block"},
		{method: "POST", path: "/admin/api/litellmops/gateway/models/:id/unblock"},
	},
}

// AdminAuthorizer adapts the module permission contract to the Admin
// database-backed Casbin policy, mirroring the generated module pattern.
type AdminAuthorizer struct {
	database  business.RequestDatabase
	principal business.PrincipalResolver
}

// NewAdminAuthorizer requires the request-scoped Admin database and the
// canonical principal resolver installed by the Admin authentication middleware.
func NewAdminAuthorizer(
	database business.RequestDatabase,
	principal business.PrincipalResolver,
) (*AdminAuthorizer, error) {
	if database == nil {
		return nil, errors.New("litellmops authorization database provider is required")
	}
	if principal == nil {
		return nil, errors.New("litellmops principal resolver is required")
	}
	return &AdminAuthorizer{database: database, principal: principal}, nil
}

// Authorize validates the permission-to-route binding, requires an
// authenticated Admin principal, then reads the canonical Casbin table.
func (authorizer *AdminAuthorizer) Authorize(ctx *gin.Context, permission string) error {
	if authorizer == nil || authorizer.database == nil || authorizer.principal == nil {
		return ErrAuthorizationUnavailable
	}
	routes, declared := authorizationRoutes[permission]
	if !declared || ctx == nil || ctx.Request == nil {
		return ErrAuthorizationDenied
	}
	matched := authorizationRoute{}
	found := false
	for _, route := range routes {
		if ctx.Request.Method == route.method && ctx.FullPath() == route.path {
			matched = route
			found = true
			break
		}
	}
	if !found {
		return ErrAuthorizationDenied
	}
	principal := authorizer.principal(ctx)
	if principal == nil || strings.TrimSpace(principal.GetRoleID()) == "" {
		return ErrAuthenticationRequired
	}
	if principal.Root() {
		return nil
	}
	db, ok := authorizer.database(ctx.Request.Context())
	if !ok || db == nil {
		return ErrAuthorizationUnavailable
	}
	var count int64
	if err := db.WithContext(ctx.Request.Context()).Model(&models.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p",
		principal.GetRoleID(),
		adminpkg.APIAccessType.String(),
		matched.path,
		matched.method,
	).Count(&count).Error; err != nil {
		return fmt.Errorf("%w: read Admin policy", ErrAuthorizationUnavailable)
	}
	if count == 0 {
		return ErrAuthorizationDenied
	}
	return nil
}
