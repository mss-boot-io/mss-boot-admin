package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type customRouteKey struct {
	method string
	path   string
}

type discoveredCustomRoute struct {
	key              customRouteKey
	handler          string
	directMiddleware []string
	groupMiddleware  []string
	file             string
	line             int
}

type groupMiddlewareUse struct {
	position   token.Pos
	middleware []string
}

func TestCustomRouteContractsAreValid(t *testing.T) {
	t.Parallel()

	validClasses := map[CustomRouteClass]bool{
		RoutePublic:                true,
		RouteOptionalAuthenticated: true,
		RouteAuthenticatedSelf:     true,
		RouteAuthorized:            true,
	}
	seen := make(map[customRouteKey]int, len(customRouteContracts))
	for index, contract := range customRouteContracts {
		key := customRouteKey{method: contract.Method, path: contract.Path}
		if previous, exists := seen[key]; exists {
			t.Errorf("duplicate custom route contract %s %s at indexes %d and %d", key.method, key.path, previous, index)
		}
		seen[key] = index

		if !validClasses[contract.Class] {
			t.Errorf("%s %s has unsupported security class %q", key.method, key.path, contract.Class)
		}
		if contract.Method != strings.ToUpper(contract.Method) {
			t.Errorf("%s %s must use an uppercase HTTP method", key.method, key.path)
		}
		if !strings.HasPrefix(contract.Path, "/admin/api/") {
			t.Errorf("%s %s must use its fully qualified Admin API path", key.method, key.path)
		}
		if contract.Class == RouteAuthorized {
			if strings.TrimSpace(contract.Permission) == "" {
				t.Errorf("%s %s is Authorized without a permission", key.method, key.path)
			}
		} else if contract.Permission != "" {
			t.Errorf("%s %s has permission %q outside the Authorized class", key.method, key.path, contract.Permission)
		}
		if contract.RootOnly && contract.Class != RouteAuthorized {
			t.Errorf("%s %s is root-only outside the Authorized class", key.method, key.path)
		}
		if contract.Mutation && contract.Method == http.MethodGet {
			t.Errorf("state-changing route %s %s must not use GET", key.method, key.path)
		}
		if contract.Method != http.MethodGet && !contract.Mutation {
			t.Errorf("non-GET route %s %s must explicitly declare its mutation", key.method, key.path)
		}
		if contract.ConstrainedPublicGET {
			if contract.Method != http.MethodGet || contract.Mutation ||
				(contract.Class != RoutePublic && contract.Class != RouteOptionalAuthenticated) {
				t.Errorf("constrained protocol route %s %s must be an active non-mutating Public or OptionalAuthenticated GET", key.method, key.path)
			}
		}
	}
}

func TestRetiredDeveloperToolRoutesAreNotContracted(t *testing.T) {
	t.Parallel()

	retired := map[customRouteKey]bool{
		{method: http.MethodGet, path: "/admin/api/github/get-login-url"}:  true,
		{method: http.MethodPut, path: "/admin/api/model/generate-data"}:   true,
		{method: http.MethodGet, path: "/admin/api/template/get-branches"}: true,
		{method: http.MethodGet, path: "/admin/api/template/get-path"}:     true,
		{method: http.MethodGet, path: "/admin/api/template/get-params"}:   true,
		{method: http.MethodPost, path: "/admin/api/template/generate"}:    true,
	}
	for _, contract := range customRouteContracts {
		key := customRouteKey{method: contract.Method, path: contract.Path}
		if retired[key] {
			t.Errorf("retired developer-tool route remains contracted: %s %s", key.method, key.path)
		}
	}
}

func TestCustomRouteContractsCoverOtherRegistrations(t *testing.T) {
	t.Parallel()

	discovered := discoverOtherRoutes(t)
	contracts := make(map[customRouteKey]CustomRouteContract, len(customRouteContracts))
	for _, contract := range customRouteContracts {
		key := customRouteKey{method: contract.Method, path: contract.Path}
		contracts[key] = contract
	}

	missing := make([]string, 0)
	for key, route := range discovered {
		if _, exists := contracts[key]; exists {
			continue
		}
		missing = append(missing, routeDescription(route))
	}
	sort.Strings(missing)
	for _, route := range missing {
		t.Errorf("unclassified Other route: %s", route)
	}

	stale := make([]string, 0)
	for key := range contracts {
		if _, exists := discovered[key]; exists {
			continue
		}
		stale = append(stale, key.method+" "+key.path)
	}
	sort.Strings(stale)
	for _, route := range stale {
		t.Errorf("stale custom route contract: %s", route)
	}
}

func TestCustomRouteContractsMatchRuntimeAuthentication(t *testing.T) {
	t.Parallel()

	discovered := discoverOtherRoutes(t)
	for _, contract := range customRouteContracts {
		key := customRouteKey{method: contract.Method, path: contract.Path}
		route, exists := discovered[key]
		if !exists {
			// The coverage test reports missing and stale registrations with the
			// source-oriented error that is more useful for that failure.
			continue
		}

		switch contract.Class {
		case RouteOptionalAuthenticated:
			if !containsMiddleware(route.directMiddleware, "OptionalAuth") {
				t.Errorf(
					"%s %s is classified OptionalAuthenticated, but its registration at %s:%d does not directly include middleware.OptionalAuth(); direct middleware: %s",
					key.method,
					key.path,
					route.file,
					route.line,
					formatMiddleware(route.directMiddleware),
				)
			}
		case RouteAuthenticatedSelf, RouteAuthorized:
			if !hasRequiredAuthentication(route.directMiddleware) && !hasRequiredAuthentication(route.groupMiddleware) {
				t.Errorf(
					"%s %s is classified %s, but its registration at %s:%d has no mandatory authentication; add response.AuthHandler or middleware.Auth.MiddlewareFunc() directly, or call router.Use with mandatory auth before registering the route (direct: %s; prior group Use: %s)",
					key.method,
					key.path,
					contract.Class,
					route.file,
					route.line,
					formatMiddleware(route.directMiddleware),
					formatMiddleware(route.groupMiddleware),
				)
			}
		case RoutePublic:
			// Public and constrained protocol GET routes do not require
			// authentication. A controller may still apply a group
			// middleware for a narrower deployment policy.
		}
		if contract.RootOnly && !containsMiddleware(route.directMiddleware, "requireRootManagement") {
			t.Errorf(
				"%s %s is contracted root-only, but its registration at %s:%d does not directly include requireRootManagement; direct middleware: %s",
				key.method,
				key.path,
				route.file,
				route.line,
				formatMiddleware(route.directMiddleware),
			)
		}
	}
}

func discoverOtherRoutes(t *testing.T) map[customRouteKey]discoveredCustomRoute {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve security contract test path")
	}
	apisDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "apis"))
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, apisDir, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") &&
			!strings.HasSuffix(name, "_test.go") &&
			!strings.HasPrefix(name, "_") &&
			!strings.HasPrefix(name, ".")
	}, 0)
	if err != nil {
		t.Fatalf("parse Admin API controllers: %v", err)
	}

	routes := make(map[customRouteKey]discoveredCustomRoute)
	for _, parsedPackage := range packages {
		for filename, file := range parsedPackage.Files {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Name.Name != "Other" || function.Body == nil || function.Type.Params == nil {
					continue
				}
				routerNames := routerParameterNames(function)
				if len(routerNames) == 0 {
					continue
				}
				groupUses := discoverGroupMiddlewareUses(function, routerNames)
				ast.Inspect(function.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || !isRouteMethod(selector.Sel.Name) || len(call.Args) < 2 {
						return true
					}
					receiver, ok := selector.X.(*ast.Ident)
					if !ok || !routerNames[receiver.Name] {
						return true
					}
					pathLiteral, ok := call.Args[0].(*ast.BasicLit)
					if !ok || pathLiteral.Kind != token.STRING {
						position := fset.Position(call.Pos())
						t.Errorf("Other route path must be a string literal at %s:%d", filepath.Base(filename), position.Line)
						return true
					}
					path, err := strconv.Unquote(pathLiteral.Value)
					if err != nil {
						t.Errorf("decode Other route path at %s: %v", fset.Position(pathLiteral.Pos()), err)
						return true
					}
					key := customRouteKey{method: selector.Sel.Name, path: "/admin/api" + path}
					position := fset.Position(call.Pos())
					route := discoveredCustomRoute{
						key:              key,
						handler:          expressionName(call.Args[len(call.Args)-1]),
						directMiddleware: middlewareNames(call.Args[1 : len(call.Args)-1]),
						groupMiddleware:  priorGroupMiddleware(groupUses[receiver.Name], call.Pos()),
						file:             filepath.Base(filename),
						line:             position.Line,
					}
					if previous, exists := routes[key]; exists {
						t.Errorf("duplicate Other route %s %s at %s:%d and %s:%d", key.method, key.path, previous.file, previous.line, route.file, route.line)
						return true
					}
					routes[key] = route
					return true
				})
			}
		}
	}
	return routes
}

func discoverGroupMiddlewareUses(function *ast.FuncDecl, routerNames map[string]bool) map[string][]groupMiddlewareUse {
	uses := make(map[string][]groupMiddlewareUse)
	for _, statement := range function.Body.List {
		expressionStatement, ok := statement.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expressionStatement.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Use" {
			continue
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || !routerNames[receiver.Name] {
			continue
		}
		uses[receiver.Name] = append(uses[receiver.Name], groupMiddlewareUse{
			position:   call.Pos(),
			middleware: middlewareNames(call.Args),
		})
	}
	return uses
}

func priorGroupMiddleware(uses []groupMiddlewareUse, routePosition token.Pos) []string {
	var middleware []string
	for _, use := range uses {
		// Gin snapshots the group's handlers when a route is registered, so a
		// later Use call cannot protect an already registered route.
		if use.position >= routePosition {
			continue
		}
		middleware = append(middleware, use.middleware...)
	}
	return middleware
}

func middlewareNames(expressions []ast.Expr) []string {
	names := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		name := expressionName(expression)
		if call, ok := expression.(*ast.CallExpr); ok && name == "GetMiddlewares" && hasStringArgument(call, "auth") {
			name = "GetMiddlewares(auth)"
		}
		names = append(names, name)
	}
	return names
}

func hasStringArgument(call *ast.CallExpr, expected string) bool {
	for _, argument := range call.Args {
		literal, ok := argument.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil && value == expected {
			return true
		}
	}
	return false
}

func hasRequiredAuthentication(middleware []string) bool {
	return containsMiddleware(middleware, "AuthHandler") ||
		containsMiddleware(middleware, "MiddlewareFunc") ||
		containsMiddleware(middleware, "GetMiddlewares(auth)")
}

func containsMiddleware(middleware []string, expected string) bool {
	for _, name := range middleware {
		if name == expected {
			return true
		}
	}
	return false
}

func formatMiddleware(middleware []string) string {
	if len(middleware) == 0 {
		return "none"
	}
	return strings.Join(middleware, ", ")
}

func routerParameterNames(function *ast.FuncDecl) map[string]bool {
	names := make(map[string]bool)
	for _, field := range function.Type.Params.List {
		selector, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		typeSelector, ok := selector.X.(*ast.SelectorExpr)
		if !ok || typeSelector.Sel.Name != "RouterGroup" {
			continue
		}
		for _, name := range field.Names {
			names[name.Name] = true
		}
	}
	return names
}

func isRouteMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func expressionName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.CallExpr:
		return expressionName(value.Fun)
	case *ast.FuncLit:
		return "func-literal"
	default:
		return "unknown"
	}
}

func routeDescription(route discoveredCustomRoute) string {
	return route.key.method + " " + route.key.path + " (" + route.file + ":" + strconv.Itoa(route.line) + ")"
}
