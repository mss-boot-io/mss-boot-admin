package verify

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestV101ChangedProvidersDoNotExitOrDetach is the executable checkpoint
// behind the historical acceptance name. It intentionally targets the
// changed Kafka composition graph instead of claiming that every legacy
// provider in the repository already satisfies Runtime v2 lifecycle rules.
func TestV101ChangedProvidersDoNotExitOrDetach(t *testing.T) {
	root := repositoryRoot(t)
	targets := []string{
		"mss-boot/pkg/config/queue.go",
		"mss-boot/pkg/config/storage/queue/kafka.go",
		"mss-boot/pkg/config/storage/queue/casbin_watcher.go",
		"admin/config/config.go",
		"admin/cmd/server/server.go",
	}

	files := make(map[string]*ast.File, len(targets))
	for _, relative := range targets {
		path := filepath.Join(root, filepath.FromSlash(relative))
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse changed provider path %s: %v", relative, err)
		}
		files[relative] = parsed
		assertNoProcessTerminationOrTODO(t, relative, parsed)
	}

	assertCriticalFunctionsUseCallerContext(t, "mss-boot/pkg/config/queue.go", files["mss-boot/pkg/config/queue.go"], map[string]bool{
		"buildConfig": true, "buildSaramaConfig": true, "configureMSK": true, "InitContext": true,
	})
	assertCriticalFunctionsUseCallerContext(t, "mss-boot/pkg/config/storage/queue/kafka.go", files["mss-boot/pkg/config/storage/queue/kafka.go"], map[string]bool{
		"RegisterContext": true, "register": true, "constructConsumerGroup": true,
		"Start": true, "runConsumer": true, "observeConsumerErrors": true,
		"Close": true, "startClose": true, "finishClose": true,
	})
	assertCriticalFunctionsUseCallerContext(t, "mss-boot/pkg/config/storage/queue/casbin_watcher.go", files["mss-boot/pkg/config/storage/queue/casbin_watcher.go"], map[string]bool{
		"SetUpdateCallbackContext": true, "setUpdateCallback": true,
	})
	assertCriticalFunctionsUseCallerContext(t, "admin/config/config.go", files["admin/config/config.go"], map[string]bool{
		"buildOptionalQueue": true, "closeQueueAdapter": true, "closeManagedQueue": true,
		"CloseContext": true, "bindPolicyWatcher": true,
	})
	assertCriticalFunctionsUseCallerContext(t, "admin/cmd/server/server.go", files["admin/cmd/server/server.go"], map[string]bool{
		"Start": true, "drainManagedQueueErrors": true, "stopManagedQueueAfterRuntimeError": true,
	})

	assertGoStatementsOwnedByKafka(t, files["mss-boot/pkg/config/storage/queue/kafka.go"])
	assertServerDoesNotDetachManagedKafka(t, files["admin/cmd/server/server.go"])
	assertManagedQueueCompositionContracts(t, root)
}

func assertNoProcessTerminationOrTODO(t *testing.T, relative string, file *ast.File) {
	t.Helper()
	imports := importedPackagePaths(t, relative, file)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}

		packagePath := imports[owner.Name]
		forbidden := packagePath == "os" && selector.Sel.Name == "Exit"
		forbidden = forbidden || packagePath == "log" && strings.HasPrefix(selector.Sel.Name, "Fatal")
		forbidden = forbidden || packagePath == "context" && selector.Sel.Name == "TODO"
		if forbidden {
			t.Errorf("changed provider path %s contains forbidden %s.%s call", relative, packagePath, selector.Sel.Name)
		}
		return true
	})
}

func importedPackagePaths(t *testing.T, relative string, file *ast.File) map[string]string {
	t.Helper()
	result := make(map[string]string, len(file.Imports))
	for _, imported := range file.Imports {
		packagePath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatalf("decode import in %s: %v", relative, err)
		}
		name := pathpkg.Base(packagePath)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		if name == "." && (packagePath == "context" || packagePath == "log" || packagePath == "os") {
			t.Errorf("changed provider path %s must not dot-import lifecycle package %s", relative, packagePath)
			continue
		}
		result[name] = packagePath
	}
	return result
}

func assertCriticalFunctionsUseCallerContext(t *testing.T, relative string, file *ast.File, critical map[string]bool) {
	t.Helper()
	imports := importedPackagePaths(t, relative, file)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || !critical[function.Name.Name] {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			owner, ok := selector.X.(*ast.Ident)
			if ok && imports[owner.Name] == "context" && (selector.Sel.Name == "Background" || selector.Sel.Name == "TODO") {
				t.Errorf("critical lifecycle function %s in %s replaces its caller context with context.%s", function.Name.Name, relative, selector.Sel.Name)
			}
			return true
		})
	}
}

func assertGoStatementsOwnedByKafka(t *testing.T, file *ast.File) {
	t.Helper()
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.GoStmt:
				switch function.Name.Name {
				case "Start":
					literal, ok := typed.Call.Fun.(*ast.FuncLit)
					if !ok || !functionDefersSelector(literal.Body, "runners", "Done") {
						t.Errorf("Kafka Start goroutine must defer e.runners.Done")
					}
				case "startClose":
					if !callUsesReceiverSelector(typed.Call, "e", "finishClose") {
						t.Errorf("Kafka startClose may only launch the closeDone-tracked finishClose worker")
					}
				default:
					t.Errorf("Kafka goroutine is not owned by Start/startClose: function %s", function.Name.Name)
				}
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Go" {
					return true
				}
				receiver, ok := selector.X.(*ast.SelectorExpr)
				if !ok || function.Name.Name != "register" || !selectorUsesReceiver(receiver, "e", "registrations") || len(typed.Args) != 1 {
					t.Errorf("Kafka WaitGroup.Go work is not owned by the registrations group in register")
					return true
				}
				literal, ok := typed.Args[0].(*ast.FuncLit)
				if !ok || !bodyCallsReceiverSelector(literal.Body, "e", "constructConsumerGroup") {
					t.Errorf("Kafka registrations.Go must own constructConsumerGroup")
				}
			}
			return true
		})
	}
}

func assertServerDoesNotDetachManagedKafka(t *testing.T, file *ast.File) {
	t.Helper()
	foundRun := false
	foundManagedStart := false
	foundLegacyGuard := false
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		switch function.Name.Name {
		case "run":
			foundRun = true
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if _, ok := node.(*ast.GoStmt); ok {
					t.Error("server run must not start a detached queue goroutine")
				}
				return true
			})
		case "Start":
			if receiverTypeName(function) != "managedQueueRuntime" {
				continue
			}
			foundManagedStart = true
			goStatements := 0
			ast.Inspect(function.Body, func(node ast.Node) bool {
				statement, ok := node.(*ast.GoStmt)
				if !ok {
					return true
				}
				goStatements++
				literal, ok := statement.Call.Fun.(*ast.FuncLit)
				if !ok || !bodyContainsSend(literal.Body) {
					t.Error("managed queue runtime goroutine must report completion through its owned result channel")
				}
				return true
			})
			if goStatements != 1 {
				t.Errorf("managed queue runtime Start goroutines = %d, want exactly 1 tracked worker", goStatements)
			}
		case "startLegacyQueue":
			goStatements := 0
			guarded := false
			for _, statement := range function.Body.List {
				if conditional, ok := statement.(*ast.IfStmt); ok && expressionCallsFunction(conditional.Cond, "managedQueueRunnable") && bodyContainsReturn(conditional.Body) {
					guarded = true
				}
				if goStatement, ok := statement.(*ast.GoStmt); ok {
					goStatements++
					if !callUsesReceiverSelector(goStatement.Call, "adapter", "Run") {
						t.Error("legacy queue helper may only launch adapter.Run after the managed guard")
					}
				}
			}
			foundLegacyGuard = guarded && goStatements == 1
		}
	}
	if !foundRun || !foundManagedStart || !foundLegacyGuard {
		t.Errorf("Admin server lifecycle AST contract incomplete: run=%t managedStart=%t guardedLegacy=%t", foundRun, foundManagedStart, foundLegacyGuard)
	}
}

func selectorUsesReceiver(selector *ast.SelectorExpr, receiverName, fieldName string) bool {
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == receiverName && selector.Sel.Name == fieldName
}

func callUsesReceiverSelector(call *ast.CallExpr, receiverName, methodName string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == receiverName && selector.Sel.Name == methodName
}

func functionDefersSelector(body *ast.BlockStmt, receiverField, methodName string) bool {
	for _, statement := range body.List {
		deferred, ok := statement.(*ast.DeferStmt)
		if !ok {
			continue
		}
		selector, ok := deferred.Call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != methodName {
			continue
		}
		receiver, ok := selector.X.(*ast.SelectorExpr)
		if ok && selectorUsesReceiver(receiver, "e", receiverField) {
			return true
		}
	}
	return false
}

func bodyCallsReceiverSelector(body *ast.BlockStmt, receiverName, methodName string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && callUsesReceiverSelector(call, receiverName, methodName) {
			found = true
		}
		return !found
	})
	return found
}

func bodyContainsSend(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if _, ok := node.(*ast.SendStmt); ok {
			found = true
		}
		return !found
	})
	return found
}

func expressionCallsFunction(expression ast.Expr, functionName string) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == functionName {
			found = true
		}
		return !found
	})
	return found
}

func bodyContainsReturn(body *ast.BlockStmt) bool {
	for _, statement := range body.List {
		if _, ok := statement.(*ast.ReturnStmt); ok {
			return true
		}
	}
	return false
}

func receiverTypeName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) != 1 {
		return ""
	}
	typeExpression := function.Recv.List[0].Type
	if pointer, ok := typeExpression.(*ast.StarExpr); ok {
		typeExpression = pointer.X
	}
	identifier, _ := typeExpression.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

func assertManagedQueueCompositionContracts(t *testing.T, root string) {
	t.Helper()
	checks := map[string][]string{
		"mss-boot/pkg/config/storage/type.go": {
			"type ManagedAdapterQueue interface",
			"RegisterContext(context.Context",
			"Start(context.Context) error",
			"Errors() <-chan error",
			"Close(context.Context) error",
		},
		"mss-boot/pkg/config/queue.go": {
			"func (e *Queue) InitContext(",
			"MSK requires InitContext with an owner context",
		},
		"admin/config/config.go": {
			"initializer = e.Queue.InitContext",
			"return bindPolicyWatcher(ctx, adapter, enforcer)",
			"queueErr := e.closeManagedQueue(ctx)",
			"handle, leases := e.swapDatabaseHandle(nil)",
		},
		"mss-boot/pkg/config/storage/queue/casbin_watcher.go": {
			"SetUpdateCallbackContext(ctx context.Context",
			"RegisterContext(ctx",
		},
	}

	for relative, requiredTokens := range checks {
		content := readLifecycleContractFile(t, root, relative)
		for _, required := range requiredTokens {
			if !strings.Contains(content, required) {
				t.Errorf("managed queue composition %s is missing %q", relative, required)
			}
		}
	}
}

func readLifecycleContractFile(t *testing.T, root, relative string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read managed queue contract %s: %v", relative, err)
	}
	return string(content)
}
