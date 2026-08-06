package router

import "net/http"

// CustomRouteClass is the explicit security boundary for routes registered by
// an Admin controller's Other method. The list is deliberately closed: adding
// another class requires an architecture decision and corresponding tests.
type CustomRouteClass string

const (
	RoutePublic                CustomRouteClass = "Public"
	RouteOptionalAuthenticated CustomRouteClass = "OptionalAuthenticated"
	RouteAuthenticatedSelf     CustomRouteClass = "AuthenticatedSelf"
	RouteAuthorized            CustomRouteClass = "Authorized"
)

// CustomRouteContract is the machine-readable inventory for an Admin custom
// route. Permission is required only for RouteAuthorized. Mutation describes
// application state, rather than the HTTP method alone.
//
// LegacyDenyOnly marks a retired GET route whose only valid behavior is a 405
// response. ConstrainedPublicGET is reserved for protocol callbacks that must
// remain GET endpoints and perform their own state/nonce validation.
type CustomRouteContract struct {
	Method               string
	Path                 string
	Class                CustomRouteClass
	Permission           string
	Mutation             bool
	LegacyDenyOnly       bool
	ConstrainedPublicGET bool
}

var customRouteContracts = []CustomRouteContract{
	// Application configuration.
	{Method: http.MethodGet, Path: "/admin/api/app-configs/:group", Class: RouteAuthorized, Permission: "config:read"},
	{Method: http.MethodPut, Path: "/admin/api/app-configs/:group", Class: RouteAuthorized, Permission: "config:write", Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/app-configs/profile", Class: RouteOptionalAuthenticated},

	// Audit evidence.
	{Method: http.MethodGet, Path: "/admin/api/audit-logs/login", Class: RouteAuthorized, Permission: "audit:read"},
	{Method: http.MethodGet, Path: "/admin/api/audit-logs/operation", Class: RouteAuthorized, Permission: "audit:read"},

	// Organization directory.
	{Method: http.MethodGet, Path: "/admin/api/departments", Class: RouteAuthorized, Permission: "department:read"},
	{Method: http.MethodGet, Path: "/admin/api/posts", Class: RouteAuthorized, Permission: "post:read"},

	// Public authentication and locale discovery.
	{Method: http.MethodGet, Path: "/admin/api/github/get-login-url", Class: RoutePublic, LegacyDenyOnly: true},
	{Method: http.MethodGet, Path: "/admin/api/language/profile", Class: RoutePublic},
	{Method: http.MethodGet, Path: "/admin/api/languages/public", Class: RoutePublic},

	// Runtime logs and monitoring.
	{Method: http.MethodGet, Path: "/admin/api/logs", Class: RouteAuthorized, Permission: "log:read"},
	{Method: http.MethodGet, Path: "/admin/api/logs/files", Class: RouteAuthorized, Permission: "log:read"},
	{Method: http.MethodGet, Path: "/admin/api/logs/export", Class: RouteAuthorized, Permission: "log:export"},
	{Method: http.MethodGet, Path: "/admin/api/monitor", Class: RouteAuthorized, Permission: "monitor:read"},

	// Menu and role administration.
	{Method: http.MethodGet, Path: "/admin/api/menu/tree", Class: RouteAuthorized, Permission: "menu:read"},
	{Method: http.MethodGet, Path: "/admin/api/menu/authorize", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPut, Path: "/admin/api/menu/authorize/:roleID", Class: RouteAuthorized, Permission: "menu:authorize", Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/menu/api/:id", Class: RouteAuthorized, Permission: "menu:read"},
	{Method: http.MethodPost, Path: "/admin/api/menu/bind-api", Class: RouteAuthorized, Permission: "menu:bind-api", Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/menus", Class: RouteAuthorized, Permission: "menu:read"},
	{Method: http.MethodPost, Path: "/admin/api/role/authorize/:roleID", Class: RouteAuthorized, Permission: "role:authorize", Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/role/authorize/:roleID", Class: RouteAuthorized, Permission: "role:authorize"},

	// Legacy runtime model generation.
	{Method: http.MethodPut, Path: "/admin/api/model/generate-data", Class: RouteAuthorized, Permission: "model:generate", Mutation: true},

	// Current-user notices and preferences.
	{Method: http.MethodGet, Path: "/admin/api/notice/unread", Class: RouteOptionalAuthenticated},
	{Method: http.MethodPut, Path: "/admin/api/notice/read/:id", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/notice/read/:id", Class: RouteAuthenticatedSelf},
	{Method: http.MethodGet, Path: "/admin/api/user-configs/:group", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPut, Path: "/admin/api/user-configs/:group", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/user-configs/profile", Class: RouteOptionalAuthenticated},

	// Session and websocket operations.
	{Method: http.MethodGet, Path: "/admin/api/online-sessions", Class: RouteAuthorized, Permission: "session:read"},
	{Method: http.MethodGet, Path: "/admin/api/online-sessions/:id", Class: RouteAuthorized, Permission: "session:read"},
	{Method: http.MethodDelete, Path: "/admin/api/online-sessions/:id", Class: RouteAuthorized, Permission: "session:revoke", Mutation: true},
	{Method: http.MethodDelete, Path: "/admin/api/online-sessions/user/:userID", Class: RouteAuthorized, Permission: "session:revoke", Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/online-sessions/logout", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/ws/connect", Class: RouteAuthenticatedSelf},
	{Method: http.MethodGet, Path: "/admin/api/ws/online", Class: RouteAuthorized, Permission: "session:read"},

	// Statistics and storage.
	{Method: http.MethodGet, Path: "/admin/api/statistics/:name", Class: RouteAuthorized, Permission: "statistics:read"},
	{Method: http.MethodPost, Path: "/admin/api/storage/upload", Class: RouteAuthenticatedSelf, Mutation: true},

	// Task administration. The legacy GET remains only as an explicit 405.
	{Method: http.MethodPost, Path: "/admin/api/tasks/:id/actions/:operate", Class: RouteAuthorized, Permission: "task:operate", Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/task/:operate/:id", Class: RoutePublic, LegacyDenyOnly: true},
	{Method: http.MethodGet, Path: "/admin/api/task/func-list", Class: RouteAuthorized, Permission: "task:read"},

	// Template inspection and generation. The controller authenticates its
	// complete route group; each operation still has an explicit permission.
	{Method: http.MethodGet, Path: "/admin/api/template/get-branches", Class: RouteAuthorized, Permission: "template:read"},
	{Method: http.MethodGet, Path: "/admin/api/template/get-path", Class: RouteAuthorized, Permission: "template:read"},
	{Method: http.MethodGet, Path: "/admin/api/template/get-params", Class: RouteAuthorized, Permission: "template:read"},
	{Method: http.MethodPost, Path: "/admin/api/template/generate", Class: RouteAuthorized, Permission: "template:generate", Mutation: true},

	// Authentication, account recovery, and current-user profile.
	{Method: http.MethodPost, Path: "/admin/api/user/login", Class: RoutePublic, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/reset-password", Class: RouteOptionalAuthenticated, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/fakeCaptcha", Class: RoutePublic, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/login/github", Class: RoutePublic, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/refresh-token", Class: RoutePublic, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/user/refresh-token", Class: RoutePublic, LegacyDenyOnly: true},
	{Method: http.MethodGet, Path: "/admin/api/user/userInfo", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPut, Path: "/admin/api/user/:userID/password-reset", Class: RouteAuthorized, Permission: "user:password-reset", Mutation: true},
	{Method: http.MethodPut, Path: "/admin/api/user/userInfo", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/avatar", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/user/oauth2", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPost, Path: "/admin/api/user/oauth2/authorize", Class: RouteOptionalAuthenticated, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/binding", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodDelete, Path: "/admin/api/user/unbinding", Class: RouteAuthenticatedSelf, Mutation: true},
	{
		Method:               http.MethodGet,
		Path:                 "/admin/api/user/:provider/callback",
		Class:                RouteOptionalAuthenticated,
		ConstrainedPublicGET: true,
	},

	// Personal access tokens. The two legacy action routes remain only as 405
	// responses until clients have moved to the safe mutation methods.
	{Method: http.MethodPost, Path: "/admin/api/user-auth-tokens", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/user-auth-token/generate", Class: RoutePublic, LegacyDenyOnly: true},
	{Method: http.MethodGet, Path: "/admin/api/user-auth-tokens", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPut, Path: "/admin/api/user-auth-token/:id/revoke", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodPut, Path: "/admin/api/user-auth-token/:id/refresh", Class: RouteAuthenticatedSelf, Mutation: true},
}

// CustomRouteContracts returns a copy so callers cannot mutate the repository
// contract shared by validation and documentation tooling.
func CustomRouteContracts() []CustomRouteContract {
	contracts := make([]CustomRouteContract, len(customRouteContracts))
	copy(contracts, customRouteContracts)
	return contracts
}
