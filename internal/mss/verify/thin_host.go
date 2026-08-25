package verify

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/command"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/generator"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
	"gopkg.in/yaml.v3"
)

// ValidateThinHostStructure exposes the complete read-only Thin Host contract
// to doctor without running external build commands.
func ValidateThinHostStructure(ctx *project.Context) error {
	result := validateThinHostStructure(ctx)
	if result.ExitCode != 0 {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

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
	modulesImportPath := ""
	if confinedRepositoryDirectory(backend) && confinedRepositoryPath(modules) {
		relativeModules, err := filepath.Rel(filepath.FromSlash(backend), filepath.FromSlash(modules))
		if err != nil || !confinedRepositoryPath(filepath.ToSlash(relativeModules)) {
			problems = append(problems, "repositoryLayout.modules must be inside repositoryLayout.backend")
		} else {
			modulesImportPath = strings.TrimSuffix(ctx.Project.Spec.Backend.Module, "/") + "/" + filepath.ToSlash(relativeModules)
		}
	}

	required := []string{
		"README.md",
		".mss/project.yaml",
		".mss/lock.yaml",
		".mss/blueprint-manifest.json",
		".mss/dev.yaml",
		joinRepositoryPath(backend, "go.mod"),
		joinRepositoryPath(backend, "go.sum"),
		joinRepositoryPath(backend, "cmd/server/main.go"),
		joinRepositoryPath(modules, "registry.go"),
		joinRepositoryPath(modules, "all/generated.go"),
		joinRepositoryPath(modules, "custom/modules.go"),
		joinRepositoryPath(frontend, "package.json"),
		joinRepositoryPath(frontend, "pnpm-lock.yaml"),
		joinRepositoryPath(frontend, ".npmrc"),
		joinRepositoryPath(frontend, "tsconfig.json"),
		joinRepositoryPath(frontend, "config/config.ts"),
		joinRepositoryPath(frontend, "config/business-routes.ts"),
		joinRepositoryPath(frontend, "mss-admin.config.ts"),
		businessRoutes,
		joinRepositoryPath(frontend, "src/app.tsx"),
		joinRepositoryPath(frontend, "src/access.ts"),
		joinRepositoryPath(frontend, "src/route-registrations.ts"),
		joinRepositoryPath(frontend, "src/business/routes.config.ts"),
		joinRepositoryPath(frontend, "src/business/route-registrations.ts"),
		joinRepositoryPath(frontend, "src/business/locales/zh-CN.ts"),
		joinRepositoryPath(frontend, "src/business/locales/en-US.ts"),
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
		if _, err := readThinHostFile(ctx.Root, relative); err != nil {
			problems = append(problems, err.Error())
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
	if confinedRepositoryPath(goModRelative) {
		if data, err := readThinHostFile(ctx.Root, goModRelative); err != nil {
			problems = append(problems, err.Error())
		} else {
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
	}
	goSumRelative := joinRepositoryPath(backend, "go.sum")
	if confinedRepositoryPath(goSumRelative) {
		if data, err := readThinHostFile(ctx.Root, goSumRelative); err != nil {
			problems = append(problems, err.Error())
		} else {
			problems = append(problems, distributionGoSumProblems(data, distribution.Backend.Module, frameworkModule, distribution.Backend.Version)...)
		}
	}

	packagePath := joinRepositoryPath(frontend, "package.json")
	if confinedRepositoryPath(packagePath) {
		if data, err := readThinHostFile(ctx.Root, packagePath); err != nil {
			problems = append(problems, err.Error())
		} else {
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
	}
	lockPath := joinRepositoryPath(frontend, "pnpm-lock.yaml")
	if confinedRepositoryPath(lockPath) {
		if data, err := readThinHostFile(ctx.Root, lockPath); err != nil {
			problems = append(problems, err.Error())
		} else {
			problems = append(problems, frozenFrontendLockProblems(data, distribution.Frontend.Package, distribution.Frontend.Version)...)
		}
	}

	npmrcPath := joinRepositoryPath(frontend, ".npmrc")
	if confinedRepositoryPath(npmrcPath) {
		if data, err := readThinHostFile(ctx.Root, npmrcPath); err != nil {
			problems = append(problems, err.Error())
		} else {
			expected := "registry=https://registry.npmjs.org/\n" +
				"save-exact=true"
			if strings.TrimSpace(string(data)) != expected {
				problems = append(problems,
					npmrcPath+" must select the public npm registry with exact dependency pins and no credentials",
				)
			}
		}
	}

	requiredFragments := map[string][]string{
		joinRepositoryPath(backend, "cmd/server/main.go"): {
			distribution.Backend.Module + "/app",
			modulesImportPath,
			"modules.Modules()",
		},
		joinRepositoryPath(modules, "registry.go"): {
			distribution.Backend.Module + "/business",
			modulesImportPath + "/all",
			modulesImportPath + "/custom",
			"append(all.Modules(), custom.Modules()...)",
		},
		joinRepositoryPath(modules, "all/generated.go"): {
			distribution.Backend.Module + "/business",
		},
		joinRepositoryPath(frontend, "config/config.ts"): {
			distribution.Frontend.Package + "/business",
			"./business-routes",
			"routeRegistrations: './src/route-registrations.ts'",
			"useUtoopack: true",
		},
		joinRepositoryPath(frontend, "config/business-routes.ts"): {
			distribution.Frontend.Package + "/business",
			"../src/business/routes.config",
			"./business-routes.generated",
			"...generatedBusinessRoutes",
			"...customBusinessRoutes",
		},
		joinRepositoryPath(frontend, "src/route-registrations.ts"): {
			distribution.Frontend.Package + "/runtime",
			"./generated/routes",
			"./business/route-registrations",
			"duplicate business UI route path",
			"duplicate business server route path",
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
			"../business/locales/zh-CN",
			"...coreMessages",
			"...generatedMessages",
			"...customMessages",
		},
		joinRepositoryPath(frontend, "src/locales/en-US.ts"): {
			distribution.Frontend.Package + "/runtime/locales/en-US",
			"../generated/locales/en-US",
			"../business/locales/en-US",
			"...coreMessages",
			"...generatedMessages",
			"...customMessages",
		},
	}
	for relative, fragments := range requiredFragments {
		if !confinedRepositoryPath(relative) {
			continue
		}
		data, err := readThinHostFile(ctx.Root, relative)
		if err != nil {
			problems = append(problems, err.Error())
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

func validateThinHostGeneratedModules(ctx *project.Context) command.Result {
	started := time.Now()
	result := command.Result{
		ID:          "thin-host-generated-drift",
		Description: "verify every Thin Host AdminModule projection is current",
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
		result.Stdout = "layout foundation: Thin Host generation drift is not applicable\n"
		result.Duration = time.Since(started)
		return result
	}
	specifications := strings.TrimSpace(ctx.Project.Spec.RepositoryLayout["specifications"])
	if specifications == "" {
		specifications = ".mss"
	}
	if !confinedRepositoryPath(specifications) {
		result.ExitCode = 1
		result.Error = "project specifications directory must be repository-relative and confined"
		result.Duration = time.Since(started)
		return result
	}
	pattern := filepath.Join(ctx.Root, filepath.FromSlash(specifications), "modules", "*.yaml")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		result.ExitCode = 1
		result.Error = err.Error()
		result.Duration = time.Since(started)
		return result
	}
	sort.Strings(paths)
	sourceSpecifications := make(map[string]struct{}, len(paths))
	var output strings.Builder
	for _, path := range paths {
		module, loadErr := spec.LoadModule(path)
		if loadErr != nil {
			result.ExitCode = 1
			result.Error = loadErr.Error()
			break
		}
		relative, relErr := filepath.Rel(ctx.Root, path)
		if relErr != nil || !confinedRepositoryPath(filepath.ToSlash(relative)) {
			result.ExitCode = 1
			result.Error = "module specification path is not repository-confined: " + path
			break
		}
		module.SourcePath = filepath.ToSlash(relative)
		sourceSpecifications[module.SourcePath] = struct{}{}
		_, generateErr := generator.Generate(module, generator.Options{
			Root:           ctx.Root,
			Check:          true,
			FrontendTarget: spec.FrontendTargetAntDV6,
			Project:        &ctx.Project,
		})
		if generateErr != nil {
			result.ExitCode = 1
			result.Error = fmt.Sprintf("%s: %v", filepath.ToSlash(relative), generateErr)
			break
		}
		fmt.Fprintf(&output, "generated projections current: %s (%s)\n", filepath.ToSlash(relative), module.Metadata.Name)
	}
	if result.ExitCode == 0 {
		orphans, orphanErr := orphanedThinHostGeneratedSources(ctx.Root, specifications, sourceSpecifications)
		if orphanErr != nil {
			result.ExitCode = 1
			result.Error = orphanErr.Error()
		} else if len(orphans) > 0 {
			result.ExitCode = 1
			result.Error = "generated output references missing AdminModule specifications: " + strings.Join(orphans, ", ")
		}
	}
	if len(paths) == 0 {
		output.WriteString("no Thin Host AdminModule specifications declared\n")
	}
	result.Stdout = output.String()
	result.Duration = time.Since(started)
	return result
}

func orphanedThinHostGeneratedSources(root, specifications string, current map[string]struct{}) ([]string, error) {
	modulePrefix := filepath.ToSlash(filepath.Join(specifications, "modules")) + "/"
	orphans := make(map[string]struct{})
	skipDirectories := map[string]bool{
		".git": true, "node_modules": true, "vendor": true, "dist": true,
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skipDirectories[entry.Name()] {
				return fs.SkipDir
			}
			if path != root {
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				switch filepath.ToSlash(relative) {
				case ".mss/run", ".mss/logs", ".mss/reports":
					return fs.SkipDir
				}
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) > 4096 {
			data = data[:4096]
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "Code generated by mss") {
				continue
			}
			from := strings.Index(line, " from ")
			end := strings.Index(line, ". DO NOT EDIT")
			if from < 0 || end <= from+len(" from ") {
				continue
			}
			source := filepath.ToSlash(strings.TrimSpace(line[from+len(" from ") : end]))
			_, exists := current[source]
			if !exists || !strings.HasPrefix(source, modulePrefix) || filepath.Ext(source) != ".yaml" {
				relative, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				orphans[filepath.ToSlash(relative)+" -> "+source] = struct{}{}
			}
			break
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect generated Thin Host ownership: %w", err)
	}
	result := make([]string, 0, len(orphans))
	for orphan := range orphans {
		result = append(result, orphan)
	}
	sort.Strings(result)
	return result, nil
}

func distributionGoSumProblems(data []byte, adminModule, frameworkModule, version string) []string {
	required := map[string]bool{
		adminModule + " " + version:                 false,
		adminModule + " " + version + "/go.mod":     false,
		frameworkModule + " " + version:             false,
		frameworkModule + " " + version + "/go.mod": false,
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		key := fields[0] + " " + fields[1]
		if _, exists := required[key]; !exists {
			continue
		}
		required[key] = validGoChecksum(fields[2])
	}
	problems := make([]string, 0)
	for key, valid := range required {
		if !valid {
			problems = append(problems, "go.sum must contain an exact non-placeholder checksum for "+key)
		}
	}
	sort.Strings(problems)
	return problems
}

func validGoChecksum(value string) bool {
	if !strings.HasPrefix(value, "h1:") {
		return false
	}
	digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "h1:"))
	return err == nil && len(digest) == 32
}

func frozenFrontendLockProblems(data []byte, packageName, version string) []string {
	var problems []string
	var document struct {
		LockfileVersion string `yaml:"lockfileVersion"`
		Importers       map[string]struct {
			Dependencies map[string]struct {
				Specifier string `yaml:"specifier"`
				Version   string `yaml:"version"`
			} `yaml:"dependencies"`
		} `yaml:"importers"`
		Packages map[string]struct {
			Resolution struct {
				Integrity string `yaml:"integrity"`
			} `yaml:"resolution"`
		} `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return []string{"pnpm-lock.yaml must be valid YAML: " + err.Error()}
	}
	if document.LockfileVersion != "9.0" {
		problems = append(problems, "pnpm-lock.yaml must use lockfileVersion 9.0")
	}
	dependency, hasDependency := document.Importers["."].Dependencies[packageName]
	if !hasDependency || dependency.Specifier != version ||
		(dependency.Version != version && !strings.HasPrefix(dependency.Version, version+"(")) {
		problems = append(problems, "pnpm-lock.yaml must pin frontend specifier "+packageName+"@"+version)
	}
	packageEntry, hasPackage := document.Packages[packageName+"@"+version]
	if !hasPackage {
		problems = append(problems, "pnpm-lock.yaml must contain the frozen package snapshot for "+packageName+"@"+version)
	} else if !validPNPMIntegrity(packageEntry.Resolution.Integrity) {
		problems = append(problems, "pnpm-lock.yaml must contain a sha512 tarball integrity for "+packageName+"@"+version)
	}
	return problems
}

func validPNPMIntegrity(value string) bool {
	if !strings.HasPrefix(value, "sha512-") {
		return false
	}
	digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "sha512-"))
	return err == nil && len(digest) == 64
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
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return nil, fmt.Errorf("read required Thin Host file %s: %w", relative, err)
	}
	return data, nil
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
