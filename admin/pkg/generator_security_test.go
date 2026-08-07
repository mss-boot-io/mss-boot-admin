package pkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRejectsRenderedPathEscape(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "parent traversal", value: "../../outside"},
		{name: "absolute path", value: "/tmp/outside"},
		{name: "windows separator", value: `..\outside`},
		{name: "git metadata", value: ".git"},
		{name: "drive prefix", value: "C:outside"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			writeGeneratorTestFile(t, filepath.Join(source, "{{.Name}}"), "secret template content")
			parent := t.TempDir()
			destination := filepath.Join(parent, "destination")
			outside := filepath.Join(parent, "outside")
			err := Generate(&TemplateConfig{
				TemplateLocal: source,
				Destination:   destination,
				Params:        map[string]string{"Name": test.value},
			})
			if err == nil || !strings.Contains(err.Error(), "unsafe") {
				t.Fatalf("Generate() error = %v, want unsafe rendered path rejection", err)
			}
			if _, statErr := os.Stat(outside); !os.IsNotExist(statErr) {
				t.Fatalf("outside path was created or touched: %v", statErr)
			}
		})
	}
}

func TestGenerateRejectsRepositorySymlinkBeforeReading(t *testing.T) {
	source := t.TempDir()
	outside := filepath.Join(t.TempDir(), "host-secret.txt")
	writeGeneratorTestFile(t, outside, "host-secret-must-not-be-copied")
	if err := os.Symlink(outside, filepath.Join(source, "leak.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "destination")
	err := Generate(&TemplateConfig{TemplateLocal: source, Destination: destination})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Generate() error = %v, want symlink rejection", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(destination, "leak.txt")); readErr == nil {
		t.Fatalf("generator copied symlink target: %q", data)
	}
}

func TestGenerateNormalNestedTemplate(t *testing.T) {
	source := t.TempDir()
	writeGeneratorTestFile(
		t,
		filepath.Join(source, "service", "{{.Package}}", "hello.txt"),
		"hello {{.Name}}",
	)
	destination := filepath.Join(t.TempDir(), "destination")
	err := Generate(&TemplateConfig{
		TemplateLocal: source,
		Service:       "service",
		Destination:   destination,
		Params: map[string]string{
			"Package": "example",
			"Name":    "world",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "example", "hello.txt"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("generated content = %q, want %q", data, "hello world")
	}
}

func TestGenerateReturnsTemplateParseErrorsWithoutPanic(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
	}{
		{name: "filename", filename: "{{", content: "valid"},
		{name: "content", filename: "valid.txt", content: "{{"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			writeGeneratorTestFile(t, filepath.Join(source, test.filename), test.content)
			err := Generate(&TemplateConfig{
				TemplateLocal: source,
				Destination:   filepath.Join(t.TempDir(), "destination"),
			})
			if err == nil || !strings.Contains(err.Error(), "parse template") {
				t.Fatalf("Generate() error = %v, want template parse error", err)
			}
		})
	}
}

func writeGeneratorTestFile(t *testing.T, filename, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
