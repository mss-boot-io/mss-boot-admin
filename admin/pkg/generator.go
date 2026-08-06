/*
 * @Author: lwnmengjing
 * @Date: 2021/12/16 7:39 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/12/16 7:39 下午
 */

package pkg

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/zealic/xignore"
)

// Generator generate operator
type Generator struct {
	SubPath                  string
	TemplatePath             string
	DestinationPath          string
	Cfg                      interface{}
	TemplateIgnoreDirs       []string
	TemplateIgnoreFiles      []string
	TemplateParseIgnoreDirs  []string
	TemplateParseIgnoreFiles []string
	sourceRoot               string
	destinationRoot          string
}

// Generate example
//
//	func Generate(url, destinationPath string, cfg interface{}, githubConfig *GithubConfig, accessToken string) error {
//		templatePath := filepath.Base(url)
func Generate(c *TemplateConfig) (err error) {
	if c == nil {
		return errors.New("template config is required")
	}
	var templatePath string
	if c.TemplateLocal != "" {
		templatePath = c.TemplateLocal
	} else {
		templatePath = filepath.Base(c.TemplateUrl)
	}
	if c.TemplateLocalSubPath != "" {
		templatePath = filepath.Join(templatePath, c.TemplateLocalSubPath)
	}
	templatePath, err = filepath.Abs(templatePath)
	if err != nil {
		return fmt.Errorf("resolve template root: %w", err)
	}
	subPath := filepath.Clean(filepath.Join(templatePath, c.Service))
	if !pathWithinRoot(templatePath, subPath, true) {
		return errors.New("template service path escapes the template root")
	}
	destinationPath, err := filepath.Abs(c.Destination)
	if err != nil {
		return fmt.Errorf("resolve generator destination: %w", err)
	}
	if err := ValidateGeneratorRepositoryTree(subPath); err != nil {
		return err
	}

	if !c.CreateRepo {
		c.Github = nil
	}

	t := &Generator{
		SubPath:                  c.Service,
		TemplatePath:             templatePath,
		DestinationPath:          c.Destination,
		Cfg:                      c.Params,
		TemplateIgnoreDirs:       make([]string, 0),
		TemplateIgnoreFiles:      make([]string, 0),
		TemplateParseIgnoreDirs:  make([]string, 0),
		TemplateParseIgnoreFiles: make([]string, 0),
		sourceRoot:               subPath,
		destinationRoot:          filepath.Clean(destinationPath),
	}

	{
		templateResultIgnore, err := xignore.DirMatches(templatePath, &xignore.MatchesOptions{
			Ignorefile: TemplateIgnore,
			Nested:     true, // Handle nested ignorefile
		})
		if err != nil && err != os.ErrNotExist {
			log.Println(err)
			return err
		}
		if templateResultIgnore != nil {
			t.TemplateIgnoreDirs = templateResultIgnore.MatchedDirs
			t.TemplateIgnoreFiles = templateResultIgnore.MatchedFiles
		}
		templateParseResultIgnore, err := xignore.DirMatches(templatePath, &xignore.MatchesOptions{
			Ignorefile: TemplateParseIgnore,
			Nested:     true,
		})
		if err != nil && err != os.ErrNotExist {
			log.Println(err)
			return err
		}
		if templateParseResultIgnore != nil {
			t.TemplateParseIgnoreDirs = templateParseResultIgnore.MatchedDirs
			t.TemplateParseIgnoreFiles = templateParseResultIgnore.MatchedFiles
		}
		_ = os.RemoveAll(filepath.Join(templatePath, TemplateParseIgnore))
	}

	{
		templateResultIgnore, err := xignore.DirMatches(subPath, &xignore.MatchesOptions{
			Ignorefile: TemplateIgnore,
			Nested:     true, // Handle nested ignorefile
		})
		if err != nil && err != os.ErrNotExist {
			log.Println(err)
			return err
		}
		if templateResultIgnore != nil {

			for i := range templateResultIgnore.MatchedDirs {
				t.TemplateIgnoreDirs = append(t.TemplateIgnoreDirs,
					strings.Join(strings.Split(templateResultIgnore.MatchedDirs[i], "/")[1:], "/"))
			}
			for i := range templateResultIgnore.MatchedDirs {
				t.TemplateIgnoreFiles = append(t.TemplateIgnoreFiles,
					strings.Join(strings.Split(templateResultIgnore.MatchedDirs[i], "/")[1:], "/"))
			}
			//t.TemplateIgnoreDirs = templateResultIgnore.MatchedDirs
			//t.TemplateIgnoreFiles = templateResultIgnore.MatchedFiles
		}
		//_ = os.RemoveAll(filepath.Join(templatePath, TemplateIgnore))
		templateParseResultIgnore, err := xignore.DirMatches(subPath, &xignore.MatchesOptions{
			Ignorefile: TemplateParseIgnore,
			Nested:     true,
		})
		if err != nil && err != os.ErrNotExist {
			log.Println(err)
			return err
		}
		if templateParseResultIgnore != nil {
			t.TemplateParseIgnoreDirs = append(t.TemplateParseIgnoreDirs, templateParseResultIgnore.MatchedDirs...)
			t.TemplateParseIgnoreFiles = append(t.TemplateParseIgnoreFiles, templateParseResultIgnore.MatchedFiles...)
		}
		_ = os.RemoveAll(filepath.Join(subPath, TemplateParseIgnore))
	}

	if err = t.Traverse(); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// Traverse traverse all dir
func (e *Generator) Traverse() error {
	if e == nil || e.sourceRoot == "" || e.destinationRoot == "" {
		return errors.New("generator roots are not initialized")
	}
	return filepath.WalkDir(e.sourceRoot, e.TraverseFunc)
}

// TraverseFunc traverse callback
func (e *Generator) TraverseFunc(path string, f os.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if f == nil {
		return errors.New("template entry is missing")
	}
	if f.Type()&os.ModeSymlink != 0 {
		return fmt.Errorf("template symlink is not allowed: %s", filepath.Base(path))
	}
	if f.IsDir() && strings.EqualFold(f.Name(), ".git") {
		return filepath.SkipDir
	}
	if !f.IsDir() && !f.Type().IsRegular() {
		return fmt.Errorf("template entry is not a regular file: %s", filepath.Base(path))
	}
	switch filepath.Base(path) {
	case TemplateIgnore, TemplateParseIgnore:
		return nil
	}
	// template ignore
	if len(e.TemplateIgnoreDirs) > 0 {
		for i := range e.TemplateIgnoreDirs {
			if f.IsDir() &&
				(strings.Index(path, filepath.Join(e.TemplatePath, e.TemplateIgnoreDirs[i])) == 0 ||
					strings.Index(path, filepath.Join(e.TemplatePath, e.SubPath, e.TemplateIgnoreDirs[i])) == 0) {
				return filepath.SkipDir
			}
		}
	}
	if len(e.TemplateIgnoreFiles) > 0 {
		for i := range e.TemplateIgnoreFiles {
			if filepath.Join(e.TemplatePath, e.TemplateIgnoreFiles[i]) == path ||
				filepath.Join(e.TemplatePath, e.SubPath, e.TemplateIgnoreFiles[i]) == path {
				return nil
			}
		}
	}
	templatePath := path
	destinationPath, err := e.renderDestinationPath(path)
	if err != nil {
		return err
	}

	if f.IsDir() {
		// dir
		if !PathExist(destinationPath) {
			return PathCreate(destinationPath)
		}
		return nil
	}
	var parseIgnore bool
	// template parse ignore
	if len(e.TemplateParseIgnoreDirs) > 0 {
		for i := range e.TemplateParseIgnoreDirs {
			if strings.Index(templatePath, filepath.Join(e.TemplatePath, e.TemplateParseIgnoreDirs[i])) == 0 ||
				strings.Index(templatePath, filepath.Join(e.SubPath, e.TemplatePath, e.TemplateParseIgnoreDirs[i])) == 0 {
				parseIgnore = true
			}
		}
	}
	if !parseIgnore && len(e.TemplateParseIgnoreFiles) > 0 {
		for i := range e.TemplateParseIgnoreFiles {
			if filepath.Join(e.TemplatePath, e.TemplateParseIgnoreFiles[i]) == templatePath ||
				filepath.Join(e.SubPath, e.TemplatePath, e.TemplateParseIgnoreFiles[i]) == templatePath {
				parseIgnore = true
			}
		}
	}
	if parseIgnore {
		_, err = FileCopy(templatePath, destinationPath)
		if err != nil {
			log.Println(err)
		}
		return err
	}
	var rb []byte
	if rb, err = os.ReadFile(templatePath); err != nil {
		log.Println(err)
		return err
	}
	buffer := bytes.Buffer{}
	t, err := template.New(destinationPath + "[file]").Parse(string(rb))
	if err != nil {
		return fmt.Errorf("parse template file %s: %w", filepath.Base(templatePath), err)
	}
	if err = t.Execute(&buffer, e.Cfg); err != nil {
		log.Printf("path %s parse error\n", templatePath)
		log.Println(err)
		return err
	}
	fi, err := f.Info()
	if err != nil {
		log.Println(err)
		return err
	}
	// create file
	err = FileOpen(buffer, destinationPath, fi.Mode())
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (e *Generator) renderDestinationPath(sourcePath string) (string, error) {
	relative, err := filepath.Rel(e.sourceRoot, sourcePath)
	if err != nil || !pathWithinRoot(e.sourceRoot, sourcePath, true) {
		return "", errors.New("template source path escapes its root")
	}
	if relative == "." {
		return e.destinationRoot, nil
	}

	components := strings.Split(filepath.ToSlash(relative), "/")
	rendered := make([]string, 0, len(components))
	for _, component := range components {
		pathTemplate, parseErr := template.New("path-component").Parse(component)
		if parseErr != nil {
			return "", fmt.Errorf("parse template path component: %w", parseErr)
		}
		var output bytes.Buffer
		if executeErr := pathTemplate.Execute(&output, e.Cfg); executeErr != nil {
			return "", fmt.Errorf("render template path component: %w", executeErr)
		}
		name := output.String()
		if name == "" || name == "." || name == ".." || strings.EqualFold(name, ".git") ||
			strings.ContainsAny(name, "/\\\x00") || strings.Contains(name, ":") {
			return "", errors.New("rendered template path component is unsafe")
		}
		rendered = append(rendered, name)
	}

	destination := filepath.Join(append([]string{e.destinationRoot}, rendered...)...)
	if !pathWithinRoot(e.destinationRoot, destination, false) {
		return "", errors.New("rendered template path escapes the destination")
	}
	return destination, nil
}

func pathWithinRoot(root, candidate string, allowRoot bool) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return allowRoot || relative != "."
}

// ValidateGeneratorRepositoryTree rejects symlinks and special filesystem
// entries before parsing or generation. Otherwise a hostile Git repository
// could make the server read a host file and push its contents elsewhere.
func ValidateGeneratorRepositoryTree(root string) error {
	return filepath.WalkDir(root, func(entryPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry == nil {
			return errors.New("repository entry is missing")
		}
		if entryPath != root && entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("repository symlink is not allowed: %s", filepath.Base(entryPath))
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("repository entry is not a regular file: %s", filepath.Base(entryPath))
		}
		return nil
	})
}
