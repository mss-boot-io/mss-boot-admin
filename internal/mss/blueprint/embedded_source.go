package blueprint

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	distribution "github.com/mss-boot-io/mss-boot-admin"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
)

const embeddedBlueprintPath = ".mss/blueprints/management-system.yaml"

type embeddedFoundation struct {
	Blueprint    *Document
	BlueprintSHA string
	Identity     FoundationIdentity
	Files        []blueprintSourceFile
}

// GenerateEmbedded plans or writes a Thin Host from the immutable source in a
// release-built mss binary. The working directory is only a destination base;
// it is not inspected as a Foundation checkout and need not contain Git data.
func GenerateEmbedded(ctx context.Context, workingDirectory string, options Options) (Plan, error) {
	base, err := filepath.Abs(workingDirectory)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve standalone working directory: %w", err)
	}
	options.Application = normalizeApplication(options.Application)
	if err := ValidateApplication(options.Application); err != nil {
		return Plan{}, err
	}
	source, err := loadEmbeddedFoundation(options.Blueprint)
	if err != nil {
		return Plan{}, err
	}
	frontendPackage, err := resolveFrontendPackageForSource(ctx, options.FrontendRegistryURL, source.Blueprint, source.Files)
	if err != nil {
		return Plan{}, err
	}
	destination, err := resolveEmbeddedDestination(base, source.Blueprint, options.Application.Name, options.Destination)
	if err != nil {
		return Plan{}, err
	}
	files, manifest, err := buildDesiredFromSource(
		source.Blueprint,
		source.BlueprintSHA,
		source.Identity,
		source.Files,
		options.Application,
		frontendPackage,
	)
	if err != nil {
		return Plan{}, err
	}
	plan, err := planDestination(destination, source.Blueprint, options.Application, manifest, files, !options.Write)
	if err != nil {
		return plan, err
	}
	if !options.Write {
		return plan, nil
	}
	if !plan.Success {
		return plan, errors.New("destination contains conflicting files; no files were written")
	}
	if err := writeGeneratedSnapshot(ctx, destination, source.Blueprint, plan, files, options.InitializeGit); err != nil {
		return plan, err
	}
	plan.DryRun = false
	return plan, nil
}

func loadEmbeddedFoundation(requestedName string) (embeddedFoundation, error) {
	if strings.TrimSpace(requestedName) == "" {
		requestedName = "management-system"
	}
	if requestedName != "management-system" {
		return embeddedFoundation{}, fmt.Errorf("embedded mss release contains only the management-system blueprint, not %q", requestedName)
	}
	provenance, err := buildinfo.ReleaseProvenance()
	if err != nil {
		return embeddedFoundation{}, fmt.Errorf("embedded Distribution source is unavailable: %w; install an official release-built mss binary or use a clean Foundation contributor checkout", err)
	}
	source := distribution.EmbeddedFS()
	blueprintData, err := fs.ReadFile(source, embeddedBlueprintPath)
	if err != nil {
		return embeddedFoundation{}, fmt.Errorf("read embedded blueprint: %w", err)
	}
	blueprint, err := decodeEmbeddedDocument(source, embeddedBlueprintPath, blueprintData)
	if err != nil {
		return embeddedFoundation{}, fmt.Errorf("validate embedded blueprint: %w", err)
	}
	version := strings.TrimPrefix(provenance.Version, "v")
	if !validSemanticVersion(version) {
		return embeddedFoundation{}, fmt.Errorf("embedded mss release version %q is not semantic", provenance.Version)
	}
	expectedDistribution := "v" + version
	if blueprint.Spec.Distribution.Version != expectedDistribution ||
		blueprint.Spec.Distribution.Backend.Version != expectedDistribution ||
		blueprint.Spec.Distribution.Frontend.Version != version {
		return embeddedFoundation{}, fmt.Errorf(
			"embedded blueprint Distribution %s is incompatible with mss %s; install the matching official mss release",
			blueprint.Spec.Distribution.Version,
			provenance.Version,
		)
	}

	templateRoot := strings.TrimSuffix(normalizedPath(blueprint.Spec.TemplateRoot), "/")
	files := make([]blueprintSourceFile, 0)
	err = fs.WalkDir(source, templateRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("embedded application source contains a non-regular file: %s", path)
		}
		data, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		output := templateOutputPath(strings.TrimPrefix(path, templateRoot+"/"))
		if !safeRelativePath(output) {
			return fmt.Errorf("embedded application source produced unsafe output path %q", output)
		}
		if blueprint.Excluded(output) || output == blueprint.Spec.ManifestPath || output == blueprint.Spec.LockPath {
			return nil
		}
		files = append(files, blueprintSourceFile{
			SourcePath: path,
			OutputPath: output,
			Data:       data,
			Mode:       0o644,
		})
		return nil
	})
	if err != nil {
		return embeddedFoundation{}, fmt.Errorf("read embedded application source: %w", err)
	}
	return embeddedFoundation{
		Blueprint:    blueprint,
		BlueprintSHA: digest(blueprintData),
		Identity: FoundationIdentity{
			Repository: provenance.Repository,
			Version:    version,
			Commit:     provenance.Commit,
			Timestamp:  provenance.Timestamp,
			Channel:    "stable",
			Source:     ".mss/release-policy.yaml",
		},
		Files: files,
	}, nil
}

func resolveEmbeddedDestination(base string, blueprint *Document, name, requested string) (string, error) {
	base = filepath.Clean(base)
	var destination string
	if strings.TrimSpace(requested) == "" {
		destination = filepath.Join(base, filepath.FromSlash(blueprint.Spec.DefaultOutputDirectory), name)
	} else if filepath.IsAbs(requested) {
		destination = filepath.Clean(requested)
	} else {
		destination = filepath.Join(base, filepath.FromSlash(requested))
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return "", fmt.Errorf("resolve application destination: %w", err)
	}
	return absolute, nil
}
