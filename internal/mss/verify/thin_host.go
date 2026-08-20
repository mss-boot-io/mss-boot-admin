package verify

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func validateThinHostStructure(ctx *project.Context) command.Result {
	started := time.Now()
	result := command.Result{
		ID:          "thin-host-structure",
		Description: "validate the Thin Host boundary and Distribution dependencies",
		StartedAt:   started.UTC(),
		ExitCode:    0,
	}
	if ctx == nil {
		result.ExitCode = 1
		result.Error = "project context is required"
		result.Duration = time.Since(started)
		return result
	}
	result.Directory = ctx.Root
	if ctx.LayoutKind() != "thin-host" {
		result.Stdout = "layout foundation: Thin Host boundary is not applicable\n"
		result.Duration = time.Since(started)
		return result
	}

	distribution := ctx.Project.Spec.Distribution
	layout := ctx.Project.Spec.RepositoryLayout
	backend := strings.TrimSpace(layout["backend"])
	frontend := strings.TrimSpace(layout["frontend"])
	modules := strings.TrimSpace(layout["modules"])
	generated := strings.TrimSpace(layout["generated"])
	businessRoutes := strings.TrimSpace(layout["businessRoutes"])
	frameworkModule := strings.TrimSpace(ctx.Project.Spec.Backend.FrameworkModule)

	var problems []string
	problems = append(problems, distribution.Validate()...)
	if !confinedRepositoryDirectory(backend) {
		problems = append(problems, "repositoryLayout.backend must be a repository-relative confined directory")
	}
	for label, relative := range map[string]string{
		"repositoryLayout.frontend":       frontend,
		"repositoryLayout.modules":        modules,
		"repositoryLayout.generated":      generated,
		"repositoryLayout.businessRoutes": businessRoutes,
	} {
		if !confinedRepositoryPath(relative) {
			problems = append(problems, label+" must be a repository-relative confined path")
		}
	}
	if frameworkModule == "" || strings.ContainsAny(frameworkModule, " \\:") {
		problems = append(problems, "project spec.backend.frameworkModule is invalid")
	}

	required := []string{
		".mss/project.yaml",
		".mss/lock.yaml",
		".mss/blueprint-manifest.json",
		joinRepositoryPath(backend, "go.mod"),
		joinRepositoryPath(backend, "cmd/server/main.go"),
		joinRepositoryPath(modules, "all/generated.go"),
		joinRepositoryPath(frontend, "package.json"),
		joinRepositoryPath(frontend, ".npmrc"),
		joinRepositoryPath(frontend, "tsconfig.json"),
		joinRepositoryPath(frontend, "config/config.ts"),
		joinRepositoryPath(frontend, "mss-admin.config.ts"),
		businessRoutes,
		joinRepositoryPath(frontend, "src/app.tsx"),
		joinRepositoryPath(frontend, "src/access.ts"),
		joinRepositoryPath(frontend, "src/locales/zh-CN.ts"),
		joinRepositoryPath(frontend, "src/locales/en-US.ts"),
		joinRepositoryPath(generated, "routes.ts"),
		joinRepositoryPath(generated, "locales/zh-CN.ts"),
		joinRepositoryPath(generated, "locales/en-US.ts"),
	}
	for _, relative := range required {
		if !confinedRepositoryPath(relative) {
			continue
		}
		if problem := regularFileProblem(ctx.Root, relative); problem != "" {
			problems = append(problems, problem)
		}
	}

	forbidden := []string{
		"mss-boot",
		"cmd/mss",
		"internal/mss",
		"templates/application",
		"templates/module",
		".mss/release-policy.yaml",
		"docs/package.json",
		"tools/release",
		joinRepositoryPath(backend, "apis"),
		joinRepositoryPath(backend, "models"),
		joinRepositoryPath(backend, "service"),
		joinRepositoryPath(backend, "router"),
		joinRepositoryPath(backend, "middleware"),
		joinRepositoryPath(backend, "center"),
		joinRepositoryPath(frontend, "src/modules"),
		joinRepositoryPath(frontend, "src/shared"),
	}
	for _, relative := range forbidden {
		if !confinedRepositoryPath(relative) {
			continue
		}
		if _, err := os.Lstat(filepath.Join(ctx.Root, filepath.FromSlash(relative))); err == nil {
			problems = append(problems, "Thin Host must not contain Foundation core path "+relative)
		} else if !os.IsNotExist(err) {
			problems = append(problems, fmt.Sprintf("inspect forbidden Thin Host path %s: %v", relative, err))
		}
	}

	goModRelative := joinRepositoryPath(backend, "go.mod")
	goModPath := filepath.Join(ctx.Root, filepath.FromSlash(goModRelative))
	if data, err := os.ReadFile(goModPath); err == nil {
		if actual := goModModule(data); actual != strings.TrimSpace(ctx.Project.Spec.Backend.Module) {
			problems = append(problems, fmt.Sprintf(
				"%s module must equal project backend module %s (found %q)",
				goModRelative,
				ctx.Project.Spec.Backend.Module,
				actual,
			))
		}
		if !goModRequires(data, distribution.Backend.Module, distribution.Backend.Version) {
			problems = append(problems, fmt.Sprintf(
				"go.mod must require Distribution backend %s %s",
				distribution.Backend.Module,
				distribution.Backend.Version,
			))
		}
		if frameworkModule != "" && !goModRequires(data, frameworkModule, distribution.Backend.Version) {
			problems = append(problems, fmt.Sprintf(
				"go.mod must require Distribution framework %s %s",
				frameworkModule,
				distribution.Backend.Version,
			))
		}
	}

	packagePath := joinRepositoryPath(frontend, "package.json")
	if data, err := readThinHostFile(ctx.Root, packagePath); err == nil {
		var packageDocument struct {
			Dependencies map[string]string `json:"dependencies"`
			Scripts      map[string]string `json:"scripts"`
		}
		if err := json.Unmarshal(data, &packageDocument); err != nil {
			problems = append(problems, packagePath+": invalid JSON: "+err.Error())
		} else {
			if actual := packageDocument.Dependencies[distribution.Frontend.Package]; actual != distribution.Frontend.Version {
				problems = append(problems, fmt.Sprintf(
					"%s must depend on Distribution frontend %s@%s (found %q)",
					packagePath,
					distribution.Frontend.Package,
					distribution.Frontend.Version,
					actual,
				))
			}
			for name, expected := range map[string]string{
				"dev":   "mss-admin-web dev",
				"lint":  "mss-admin-web lint",
				"test":  "mss-admin-web test",
				"build": "mss-admin-web build",
			} {
				if packageDocument.Scripts[name] != expected {
					problems = append(problems, fmt.Sprintf("%s script %q must equal %q", packagePath, name, expected))
				}
			}
		}
	}

	npmrcPath := joinRepositoryPath(frontend, ".npmrc")
	if data, err := readThinHostFile(ctx.Root, npmrcPath); err == nil {
		expected := "@mss-boot-io:registry=https://npm.pkg.github.com\n" +
			"//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}"
		if strings.TrimSpace(string(data)) != expected {
			problems = append(problems,
				npmrcPath+" must contain only the GitHub Packages scope and NODE_AUTH_TOKEN placeholder",
			)
		}
	}

	requiredFragments := map[string][]string{
		joinRepositoryPath(backend, "cmd/server/main.go"): {
			distribution.Backend.Module + "/app",
			ctx.Project.Spec.Backend.Module + "/internal/modules/all",
		},
		joinRepositoryPath(modules, "all/generated.go"): {
			distribution.Backend.Module + "/business",
		},
		joinRepositoryPath(frontend, "config/config.ts"): {
			distribution.Frontend.Package + "/business",
			"businessRoutes",
			"routeRegistrations: './src/generated/routes.ts'",
			"useUtoopack: true",
		},
		joinRepositoryPath(frontend, "mss-admin.config.ts"): {
			"export { default } from './config/config'",
		},
		joinRepositoryPath(frontend, "src/app.tsx"): {
			"getInitialState",
			"layout",
			"request",
			"innerProvider",
			distribution.Frontend.Package + "/runtime/app",
		},
		joinRepositoryPath(frontend, "src/access.ts"): {
			distribution.Frontend.Package + "/runtime/access",
		},
		joinRepositoryPath(frontend, "src/locales/zh-CN.ts"): {
			distribution.Frontend.Package + "/runtime/locales/zh-CN",
			"../generated/locales/zh-CN",
		},
		joinRepositoryPath(frontend, "src/locales/en-US.ts"): {
			distribution.Frontend.Package + "/runtime/locales/en-US",
			"../generated/locales/en-US",
		},
	}
	for relative, fragments := range requiredFragments {
		if !confinedRepositoryPath(relative) {
			continue
		}
		data, err := readThinHostFile(ctx.Root, relative)
		if err != nil {
			continue
		}
		for _, fragment := range fragments {
			if fragment != "" && !strings.Contains(string(data), fragment) {
				problems = append(problems, fmt.Sprintf("%s must contain Thin Host glue %q", relative, fragment))
			}
		}
	}

	if confinedRepositoryPath(generated) {
		problems = append(problems, generatedFrontendImportProblems(ctx.Root, generated)...)
	}

	sort.Strings(problems)
	problems = compactStrings(problems)
	if len(problems) > 0 {
		result.ExitCode = 1
		result.Error = strings.Join(problems, "; ")
	} else {
		result.Stdout = fmt.Sprintf(
			"Thin Host boundary valid: %s@%s; backend %s@%s; frontend %s@%s; required glue present; Foundation core source absent\n",
			distribution.Name,
			distribution.Version,
			distribution.Backend.Module,
			distribution.Backend.Version,
			distribution.Frontend.Package,
			distribution.Frontend.Version,
		)
	}
	result.Duration = time.Since(started)
	return result
}

func confinedRepositoryPath(relative string) bool {
	relative = strings.TrimSpace(relative)
	if relative == "" || filepath.IsAbs(relative) || strings.ContainsAny(relative, "\\:\x00") {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func confinedRepositoryDirectory(relative string) bool {
	relative = strings.TrimSpace(relative)
	if relative == "" || filepath.IsAbs(relative) || strings.ContainsAny(relative, "\\:\x00") {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func joinRepositoryPath(base, child string) string {
	if strings.TrimSpace(base) == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(filepath.FromSlash(base), filepath.FromSlash(child)))
}

func regularFileProblem(root, relative string) string {
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return fmt.Sprintf("required Thin Host file %s: %v", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "required Thin Host path " + relative + " must be a regular non-symlink file"
	}
	return ""
}

func readThinHostFile(root, relative string) ([]byte, error) {
	if problem := regularFileProblem(root, relative); problem != "" {
		return nil, fmt.Errorf("%s", problem)
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
}

func goModRequires(data []byte, module, version string) bool {
	module = strings.TrimSpace(module)
	version = strings.TrimSpace(version)
	if module == "" || version == "" {
		return false
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	inBlock := false
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "//", 2)[0])
		switch line {
		case "require (":
			inBlock = true
			continue
		case ")":
			if inBlock {
				inBlock = false
				continue
			}
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		} else if !inBlock {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == module && fields[1] == version {
			return true
		}
	}
	return false
}

func goModModule(data []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "//", 2)[0])
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "module ")))
		if len(fields) == 1 {
			return fields[0]
		}
	}
	return ""
}

func generatedFrontendImportProblems(root, generated string) []string {
	var problems []string
	absolute := filepath.Join(root, filepath.FromSlash(generated))
	err := filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("generated frontend path contains symlink %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".ts" && extension != ".tsx" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, forbidden := range []string{"@/shared", "@mss-admin", "/src/shared"} {
			if strings.Contains(string(data), forbidden) {
				relative, _ := filepath.Rel(root, path)
				problems = append(problems, filepath.ToSlash(relative)+" imports private Admin Web path "+forbidden)
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		problems = append(problems, "inspect generated Thin Host frontend: "+err.Error())
	}
	return problems
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}
