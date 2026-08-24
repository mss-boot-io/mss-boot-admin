package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func newUnifiedSpecCommand(rootOverride *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "spec",
		Short: "Create and validate machine-readable Feature and AdminModule specifications",
	}
	command.AddCommand(newUnifiedSpecValidateCommand(rootOverride))
	command.AddCommand(newUnifiedSpecInitCommand(rootOverride))
	return command
}

func newUnifiedSpecValidateCommand(rootOverride *string) *cobra.Command {
	var format string
	var normalized bool
	command := &cobra.Command{
		Use:   "validate <spec.yaml> [more-specs...]",
		Short: "Auto-detect and validate Feature or AdminModule specifications",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			type result struct {
				Path       string         `json:"path"`
				APIVersion string         `json:"apiVersion,omitempty"`
				Kind       string         `json:"kind,omitempty"`
				Name       string         `json:"name,omitempty"`
				Valid      bool           `json:"valid"`
				Summary    map[string]any `json:"summary,omitempty"`
				Issues     []spec.Issue   `json:"issues,omitempty"`
				Error      string         `json:"error,omitempty"`
				Normalized string         `json:"normalized,omitempty"`
			}

			results := make([]result, 0, len(args))
			allValid := true
			for _, argument := range args {
				path, pathErr := repositoryInputPath(projectContext.Root, argument)
				entry := result{Path: argument, Valid: false}
				if pathErr != nil {
					entry.Error = pathErr.Error()
					results = append(results, entry)
					allValid = false
					continue
				}
				entry.Path = relativePath(projectContext.Root, path)
				document, validateErr := spec.ValidateFile(path)
				if validateErr != nil {
					allValid = false
					var moduleValidation *spec.ValidationError
					if errors.As(validateErr, &moduleValidation) {
						entry.Issues = moduleValidation.Issues
					} else {
						entry.Error = validateErr.Error()
					}
					results = append(results, entry)
					continue
				}
				document.Path = entry.Path
				entry.APIVersion = document.APIVersion
				entry.Kind = document.Kind
				entry.Name = document.Name
				entry.Valid = true
				entry.Summary = document.Summary
				if normalized {
					data, marshalErr := yaml.Marshal(document.Document)
					if marshalErr != nil {
						return marshalErr
					}
					entry.Normalized = string(data)
				}
				results = append(results, entry)
			}

			if err := writeUnifiedSpecResults(cmd.OutOrStdout(), results, format); err != nil {
				return err
			}
			if !allValid {
				return errors.New("one or more specifications are invalid")
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.Flags().BoolVar(&normalized, "normalized", false, "include normalized YAML in the result")
	return command
}

func newUnifiedSpecInitCommand(rootOverride *string) *cobra.Command {
	var kind string
	var outputPath string
	var displayName string
	var owner string
	var moduleName string
	var write bool
	command := &cobra.Command{
		Use:   "init <name>",
		Short: "Render a semantically valid Feature or AdminModule starter specification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectContext, err := loadProject(*rootOverride)
			if err != nil {
				return err
			}
			kind = strings.ToLower(strings.TrimSpace(kind))
			var data []byte
			switch kind {
			case "module", "admin-module":
				if strings.TrimSpace(moduleName) != "" {
					return errors.New("--module is supported only for Feature specifications")
				}
				kind = "module"
				data, err = spec.RenderModuleTemplate(args[0], displayName)
			case "feature":
				data, err = spec.RenderFeatureTemplateForModule(args[0], moduleName, displayName, owner)
			default:
				return fmt.Errorf("unsupported specification kind %q", kind)
			}
			if err != nil {
				return err
			}
			if !write {
				return writeLine(cmd.OutOrStdout(), data)
			}

			if outputPath == "" {
				directory := "modules"
				if kind == "feature" {
					directory = "features"
				}
				outputPath = filepath.Join(".mss", directory, args[0]+".yaml")
			}
			target, err := repositoryOutputPath(projectContext.Root, outputPath)
			if err != nil {
				return err
			}
			if _, err := os.Lstat(target); err == nil {
				return fmt.Errorf("refusing to overwrite existing path %s", relativePath(projectContext.Root, target))
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0o644); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s specification %s\n", kind, relativePath(projectContext.Root, target))
			return err
		},
	}
	command.Flags().StringVar(&kind, "kind", "module", "specification kind: module or feature")
	command.Flags().StringVar(&outputPath, "output", "", "repository-relative output path")
	command.Flags().StringVar(&displayName, "display-name", "", "human-readable display name")
	command.Flags().StringVar(&owner, "owner", "product-engineering", "Feature owner; ignored for modules")
	command.Flags().StringVar(&moduleName, "module", "", "primary AdminModule name for a Feature; defaults to the Feature name")
	command.Flags().BoolVar(&write, "write", false, "write the specification; default is stdout dry-run")
	return command
}

func writeUnifiedSpecResults(writer interface{ Write([]byte) (int, error) }, results any, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		return writeLine(writer, data)
	case "text":
		data, err := json.Marshal(results)
		if err != nil {
			return err
		}
		var decoded []struct {
			Path   string       `json:"path"`
			Kind   string       `json:"kind"`
			Name   string       `json:"name"`
			Valid  bool         `json:"valid"`
			Issues []spec.Issue `json:"issues"`
			Error  string       `json:"error"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		for _, entry := range decoded {
			if entry.Valid {
				if _, err := fmt.Fprintf(writer, "PASS %s %s (%s)\n", entry.Kind, entry.Path, entry.Name); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(writer, "FAIL %s\n", entry.Path); err != nil {
				return err
			}
			if entry.Error != "" {
				if _, err := fmt.Fprintf(writer, "  - %s\n", entry.Error); err != nil {
					return err
				}
			}
			for _, issue := range entry.Issues {
				if _, err := fmt.Fprintf(writer, "  - %s [%s]: %s\n", issue.Path, issue.Code, issue.Message); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func repositoryInputPath(root, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("specification path is required")
	}
	if filepath.IsAbs(value) {
		return "", errors.New("absolute specification paths are not allowed")
	}
	path := filepath.Join(root, filepath.Clean(filepath.FromSlash(value)))
	if err := ensureRepositoryPath(root, path); err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("specification path must be a regular non-symlink file")
	}
	return path, nil
}

func repositoryOutputPath(root, value string) (string, error) {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) {
		return "", errors.New("output path must be repository-relative")
	}
	path := filepath.Join(root, filepath.Clean(filepath.FromSlash(value)))
	if err := ensureRepositoryPath(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func ensureRepositoryPath(root, path string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("path escapes repository root")
	}
	return nil
}
