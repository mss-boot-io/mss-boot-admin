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
// route. Permission is required only for RouteAuthorized. RootOnly records an
// additional handler-adjacent authority/target boundary that a stale Casbin
// row cannot bypass. Mutation describes application state, rather than the
// HTTP method alone. ConstrainedPublicGET is reserved for protocol callbacks
// that must remain GET endpoints and perform their own state/nonce validation.
type CustomRouteContract struct {
	Method               string
	Path                 string
	Class                CustomRouteClass
	Permission           string
	RootOnly             bool
	Mutation             bool
	ConstrainedPublicGET bool
}

var customRouteContracts = []CustomRouteContract{
	// Application configuration.
	{Method: http.MethodGet, Path: "/admin/api/app-configs/:group", Class: RouteAuthorized, Permission: "config:read"},
	{Method: http.MethodPut, Path: "/admin/api/app-configs/:group", Class: RouteAuthorized, Permission: "config:write", Mutation: true},
	{Method: http.MethodDelete, Path: "/admin/api/app-configs/theme", Class: RouteAuthorized, Permission: "config:write", Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/app-configs/profile", Class: RouteOptionalAuthenticated},

	// Audit evidence.
	{Method: http.MethodGet, Path: "/admin/api/audit-logs/login", Class: RouteAuthorized, Permission: "audit:read"},
	{Method: http.MethodGet, Path: "/admin/api/audit-logs/operation", Class: RouteAuthorized, Permission: "audit:read"},

	// Organization directory.
	{Method: http.MethodGet, Path: "/admin/api/departments", Class: RouteAuthorized, Permission: "department:read"},
	{Method: http.MethodGet, Path: "/admin/api/posts", Class: RouteAuthorized, Permission: "post:read"},

	// Bounded operational summaries. System configuration remains root-only
	// because its full-resource routes can contain opaque credentials.
	{Method: http.MethodGet, Path: "/admin/api/tasks", Class: RouteAuthorized, Permission: "task:read"},
	{Method: http.MethodGet, Path: "/admin/api/system-configs", Class: RouteAuthorized, Permission: "config:read", RootOnly: true},

	// Public authentication and locale discovery.
	{Method: http.MethodGet, Path: "/admin/api/language/profile", Class: RoutePublic},
	{Method: http.MethodGet, Path: "/admin/api/languages/public", Class: RoutePublic},

	// Language management. Read and mutation permissions remain independently
	// assignable; public locale discovery above does not grant management access.
	{Method: http.MethodGet, Path: "/admin/api/languages", Class: RouteAuthorized, Permission: "language:read"},
	{Method: http.MethodGet, Path: "/admin/api/languages/:id", Class: RouteAuthorized, Permission: "language:read"},
	{Method: http.MethodPost, Path: "/admin/api/languages", Class: RouteAuthorized, Permission: "language:create", Mutation: true},
	{Method: http.MethodPut, Path: "/admin/api/languages/:id", Class: RouteAuthorized, Permission: "language:update", Mutation: true},

	// Option management. Runtime dictionaries are read through the same bounded
	// resources, while create/update/delete remain separately assignable.
	{Method: http.MethodGet, Path: "/admin/api/options", Class: RouteAuthorized, Permission: "option:read"},
	{Method: http.MethodGet, Path: "/admin/api/options/:id", Class: RouteAuthorized, Permission: "option:read"},
	{Method: http.MethodPost, Path: "/admin/api/options", Class: RouteAuthorized, Permission: "option:create", Mutation: true},
	{Method: http.MethodPut, Path: "/admin/api/options/:id", Class: RouteAuthorized, Permission: "option:update", Mutation: true},
	{Method: http.MethodDelete, Path: "/admin/api/options/:id", Class: RouteAuthorized, Permission: "option:delete", Mutation: true},

	// Runtime logs and monitoring.
	{Method: http.MethodGet, Path: "/admin/api/logs", Class: RouteAuthorized, Permission: "log:read"},
	{Method: http.MethodGet, Path: "/admin/api/logs/files", Class: RouteAuthorized, Permission: "log:read"},
	{Method: http.MethodGet, Path: "/admin/api/logs/export", Class: RouteAuthorized, Permission: "log:export"},
	{Method: http.MethodGet, Path: "/admin/api/monitor", Class: RouteAuthorized, Permission: "monitor:read"},

	// Menu and role administration.
	{Method: http.MethodGet, Path: "/admin/api/menu/tree", Class: RouteAuthorized, Permission: "menu:read"},
	{Method: http.MethodGet, Path: "/admin/api/menu/authorize", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPut, Path: "/admin/api/menu/authorize/:roleID", Class: RouteAuthorized, Permission: "menu:authorize", RootOnly: true, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/menu/api/:id", Class: RouteAuthorized, Permission: "menu:read"},
	{Method: http.MethodPost, Path: "/admin/api/menu/bind-api", Class: RouteAuthorized, Permission: "menu:bind-api", RootOnly: true, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/menus", Class: RouteAuthorized, Permission: "menu:read"},
	{Method: http.MethodPost, Path: "/admin/api/role/authorize/:roleID", Class: RouteAuthorized, Permission: "role:authorize", RootOnly: true, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/role/authorize/:roleID", Class: RouteAuthorized, Permission: "role:authorize"},

	// Current-user notices and preferences.
	{Method: http.MethodGet, Path: "/admin/api/notice/unread", Class: RouteOptionalAuthenticated},
	{Method: http.MethodPut, Path: "/admin/api/notice/read/:id", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/notice/read/:id", Class: RouteAuthenticatedSelf},
	{Method: http.MethodGet, Path: "/admin/api/user-configs/:group", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPut, Path: "/admin/api/user-configs/:group", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodDelete, Path: "/admin/api/user-configs/theme", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/user-configs/profile", Class: RouteOptionalAuthenticated},

	// Session and websocket operations.
	{Method: http.MethodGet, Path: "/admin/api/online-sessions", Class: RouteAuthorized, Permission: "session:read", RootOnly: true},
	{Method: http.MethodGet, Path: "/admin/api/online-sessions/:id", Class: RouteAuthorized, Permission: "session:read", RootOnly: true},
	{Method: http.MethodDelete, Path: "/admin/api/online-sessions/:id", Class: RouteAuthorized, Permission: "session:revoke", RootOnly: true, Mutation: true},
	{Method: http.MethodDelete, Path: "/admin/api/online-sessions/user/:userID", Class: RouteAuthorized, Permission: "session:revoke", RootOnly: true, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/online-sessions/logout", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/ws/tickets", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/ws/connect", Class: RoutePublic, ConstrainedPublicGET: true},
	{Method: http.MethodGet, Path: "/admin/api/ws/online", Class: RouteAuthorized, Permission: "session:read", RootOnly: true},

	// Statistics and storage.
	{Method: http.MethodGet, Path: "/admin/api/statistics/:name", Class: RouteAuthorized, Permission: "statistics:read"},
	{Method: http.MethodPost, Path: "/admin/api/storage/upload", Class: RouteAuthorized, Permission: "storage:upload", Mutation: true},

	// Task administration.
	{Method: http.MethodPost, Path: "/admin/api/tasks/:id/actions/:operate", Class: RouteAuthorized, Permission: "task:operate", Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/task/func-list", Class: RouteAuthorized, Permission: "task:read"},

	// Authentication, account recovery, and current-user profile.
	{Method: http.MethodPost, Path: "/admin/api/user/session/login", Class: RoutePublic, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/session/refresh-token", Class: RoutePublic, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/auth-cookie/clear", Class: RoutePublic, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/reset-password", Class: RouteOptionalAuthenticated, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/fakeCaptcha", Class: RoutePublic, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/user/userInfo", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPut, Path: "/admin/api/user/:userID/password-reset", Class: RouteAuthorized, Permission: "user:password-reset", Mutation: true},
	{Method: http.MethodPut, Path: "/admin/api/user/userInfo", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/avatar", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodGet, Path: "/admin/api/user/oauth2", Class: RouteAuthenticatedSelf},
	{Method: http.MethodGet, Path: "/admin/api/user/security", Class: RouteAuthenticatedSelf},
	{Method: http.MethodPost, Path: "/admin/api/user/security/reauthenticate", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodPut, Path: "/admin/api/user/security/password", Class: RouteAuthenticatedSelf, Mutation: true},
	{Method: http.MethodPost, Path: "/admin/api/user/session/oauth2/authorize", Class: RouteOptionalAuthenticated, Mutation: true},
	{Method: http.MethodDelete, Path: "/admin/api/user/oauth2/:provider", Class: RouteAuthenticatedSelf, Mutation: true},
	{
		Method:   http.MethodPost,
		Path:     "/admin/api/user/session/:provider/callback",
		Class:    RouteOptionalAuthenticated,
		Mutation: true,
	},

	// Personal access tokens.
	{Method: http.MethodPost, Path: "/admin/api/user-auth-tokens", Class: RouteAuthenticatedSelf, Mutation: true},
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
