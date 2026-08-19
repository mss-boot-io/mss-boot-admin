package generator

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

const (
	layoutFoundation = "foundation"
	layoutThinHost   = "thin-host"
)

// targetLayout resolves every generated location and import root from the
// target project contract. It is deliberately private so public callers keep
// using the stable generator Options API.
type targetLayout struct {
	Kind                    string
	TargetModule            string
	AdminModule             string
	FrameworkModule         string
	AdminWebPackage         string
	BackendDir              string
	ModulesDir              string
	FrontendDir             string
	GeneratedDir            string
	BusinessRoutesFile      string
	FrontendPagesDir        string
	FrontendE2EDir          string
	DocumentationModulesDir string
	SpecificationsDir       string
	LegacyMigrationDir      string
}

func resolveTargetLayout(document *project.ProjectDocument) (targetLayout, error) {
	if document == nil {
		return targetLayout{
			Kind:                    layoutFoundation,
			TargetModule:            "github.com/mss-boot-io/mss-boot-admin/admin",
			AdminModule:             "github.com/mss-boot-io/mss-boot-admin/admin",
			FrameworkModule:         "github.com/mss-boot-io/mss-boot-admin/mss-boot",
			AdminWebPackage:         "@mss-boot-io/admin-web",
			BackendDir:              "admin",
			ModulesDir:              "admin/modules",
			FrontendDir:             "web/antd-v6",
			GeneratedDir:            "web/antd-v6/src/generated",
			BusinessRoutesFile:      "web/antd-v6/config/routes.generated.ts",
			FrontendPagesDir:        "web/antd-v6/src/pages/generated",
			FrontendE2EDir:          "web/antd-v6/e2e/generated",
			DocumentationModulesDir: "docs/docs/modules",
			SpecificationsDir:       ".mss",
			LegacyMigrationDir:      "admin/cmd/migrate/migration",
		}, nil
	}

	layout := document.Spec.RepositoryLayout
	kind := strings.TrimSpace(layout["kind"])
	if kind == "" {
		kind = layoutFoundation
	}
	if kind != layoutFoundation && kind != layoutThinHost {
		return targetLayout{}, fmt.Errorf("unsupported project repository layout kind %q", kind)
	}
	result := targetLayout{
		Kind:               kind,
		TargetModule:       strings.TrimSpace(document.Spec.Backend.Module),
		AdminModule:        strings.TrimSpace(document.Spec.Distribution.Backend.Module),
		FrameworkModule:    strings.TrimSpace(document.Spec.Backend.FrameworkModule),
		AdminWebPackage:    strings.TrimSpace(document.Spec.Distribution.Frontend.Package),
		BackendDir:         strings.TrimSpace(layout["backend"]),
		ModulesDir:         strings.TrimSpace(layout["modules"]),
		FrontendDir:        strings.TrimSpace(layout["frontend"]),
		GeneratedDir:       strings.TrimSpace(layout["generated"]),
		BusinessRoutesFile: strings.TrimSpace(layout["businessRoutes"]),
		SpecificationsDir:  strings.TrimSpace(layout["specifications"]),
	}
	if result.AdminModule == "" && kind == layoutFoundation {
		result.AdminModule = result.TargetModule
	}
	if result.SpecificationsDir == "" {
		result.SpecificationsDir = ".mss"
	}
	if result.BackendDir == "" {
		result.BackendDir = "."
	}
	if result.GeneratedDir == "" && result.FrontendDir != "" {
		result.GeneratedDir = filepath.ToSlash(filepath.Join(result.FrontendDir, "src", "generated"))
	}
	if result.BusinessRoutesFile == "" && result.FrontendDir != "" {
		result.BusinessRoutesFile = filepath.ToSlash(filepath.Join(result.FrontendDir, "config", "routes.generated.ts"))
	}
	result.FrontendPagesDir = filepath.ToSlash(filepath.Join(result.FrontendDir, "src", "pages", "generated"))
	result.FrontendE2EDir = filepath.ToSlash(filepath.Join(result.FrontendDir, "e2e", "generated"))
	documentation := strings.TrimSpace(layout["documentation"])
	if documentation != "" {
		if kind == layoutFoundation {
			result.DocumentationModulesDir = filepath.ToSlash(filepath.Join(documentation, "docs", "modules"))
		} else {
			result.DocumentationModulesDir = filepath.ToSlash(filepath.Join(documentation, "modules"))
		}
	}
	if kind == layoutFoundation {
		backend := strings.TrimSpace(layout["backend"])
		result.LegacyMigrationDir = filepath.ToSlash(filepath.Join(backend, "cmd", "migrate", "migration"))
	}

	values := map[string]string{
		"backend module":      result.TargetModule,
		"Admin module":        result.AdminModule,
		"framework module":    result.FrameworkModule,
		"Admin Web package":   result.AdminWebPackage,
		"modules directory":   result.ModulesDir,
		"frontend directory":  result.FrontendDir,
		"generated directory": result.GeneratedDir,
		"business route file": result.BusinessRoutesFile,
	}
	var problems []string
	for label, value := range values {
		if value == "" {
			problems = append(problems, label+" is required by the project layout")
			continue
		}
		if label == "backend module" || label == "Admin module" || label == "framework module" {
			if strings.ContainsAny(value, " \\:") || !strings.Contains(value, "/") {
				problems = append(problems, label+" is invalid")
			}
			continue
		}
		if label == "Admin Web package" {
			if strings.ContainsAny(value, " \\:") {
				problems = append(problems, label+" is invalid")
			}
			continue
		}
		if err := validateLayoutPath(value); err != nil {
			problems = append(problems, label+": "+err.Error())
		}
	}
	for label, value := range map[string]string{
		"frontend pages directory": result.FrontendPagesDir,
		"frontend e2e directory":   result.FrontendE2EDir,
		"specifications directory": result.SpecificationsDir,
	} {
		if err := validateLayoutPath(value); err != nil {
			problems = append(problems, label+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		return targetLayout{}, errors.New(strings.Join(problems, "; "))
	}
	if _, err := result.moduleImportPath("validation"); err != nil {
		return targetLayout{}, err
	}
	return result, nil
}

func (l targetLayout) moduleImportPath(name string) (string, error) {
	relative, err := filepath.Rel(filepath.FromSlash(l.BackendDir), filepath.FromSlash(l.ModulesDir))
	if err != nil {
		return "", fmt.Errorf("resolve business module import path: %w", err)
	}
	relative = filepath.Clean(relative)
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("project modules directory must be inside the backend module root")
	}
	return strings.TrimSuffix(l.TargetModule, "/") + "/" + strings.Trim(filepath.ToSlash(filepath.Join(relative, name)), "/"), nil
}

func validateLayoutPath(value string) error {
	if filepath.IsAbs(value) || strings.ContainsAny(value, "\\:\x00") {
		return errors.New("must be a repository-relative confined path")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("must be a repository-relative confined path")
	}
	return nil
}
