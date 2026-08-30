package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestUserListCorePresentationCompilesClosedFoundationBinding(t *testing.T) {
	data, sourcePath := readUserListCorePresentationSource(t)
	document, err := ParseCorePagePresentation(data, sourcePath)
	if err != nil {
		t.Fatalf("ParseCorePagePresentation() error = %v", err)
	}
	manifest, err := document.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation() error = %v", err)
	}
	if manifest.PageKey != "user.list" || manifest.DefinitionVersion != "2" {
		t.Fatalf("core identity = %q/%q", manifest.PageKey, manifest.DefinitionVersion)
	}
	canonical, err := manifest.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
	digest := sha256.Sum256(canonical)
	wantHash := "sha256:" + hex.EncodeToString(digest[:])
	if manifest.DefinitionHash != wantHash {
		t.Fatalf("definition hash = %q, want %q", manifest.DefinitionHash, wantHash)
	}
	if len(manifest.DataSources) != 1 {
		t.Fatalf("data sources = %#v", manifest.DataSources)
	}
	dataSource := manifest.DataSources[0]
	if dataSource.ID != "user.list" || !slices.Equal(dataSource.RequiredPermissions, []string{"/users"}) {
		t.Fatalf("compiled user data source = %#v", dataSource)
	}
	if !slices.Equal(dataSource.PageSizeOptions, []int{20, 50, 100}) || dataSource.MaxPageSize != 100 || dataSource.MaxSortFields != 0 {
		t.Fatalf("compiled user data-source limits = %#v", dataSource)
	}
	if len(manifest.DefaultPresentation.List.DefaultSort) != 0 {
		t.Fatalf("user core page unexpectedly sorts: %#v", manifest.DefaultPresentation.List.DefaultSort)
	}
	wantList := []string{"username", "name", "email", "roleName", "organization", "status"}
	wantListComponents := []string{"user-identity", "text", "text", "user-role", "user-organization", "status-tag"}
	wantListWidths := map[string]int{"username": 210, "roleName": 150, "status": 120}
	for index, field := range manifest.DefaultPresentation.List.Columns {
		if index >= len(wantList) || field.Field != wantList[index] || field.Component != wantListComponents[index] {
			t.Fatalf("list column %d = %#v", index, field)
		}
		if want, fixed := wantListWidths[field.Field]; fixed {
			if field.Width == nil || *field.Width != want {
				t.Fatalf("list column %s width = %#v, want %d", field.Field, field.Width, want)
			}
		} else if field.Width != nil {
			t.Fatalf("list column %s unexpectedly fixes width %d", field.Field, *field.Width)
		}
	}
	if len(manifest.DefaultPresentation.List.Columns) != len(wantList) {
		t.Fatalf("list columns = %#v", manifest.DefaultPresentation.List.Columns)
	}
	wantSearch := []string{"name", "status"}
	wantSearchComponents := []string{"input", "status-filter"}
	for index, field := range manifest.DefaultPresentation.Search.Fields {
		if index >= len(wantSearch) || field.Field != wantSearch[index] || field.Component != wantSearchComponents[index] {
			t.Fatalf("search field %d = %#v", index, field)
		}
	}
	if len(manifest.DefaultPresentation.Search.Fields) != len(wantSearch) {
		t.Fatalf("search fields = %#v", manifest.DefaultPresentation.Search.Fields)
	}
	if len(manifest.DefaultPresentation.Form.Fields) != 0 || len(manifest.DefaultPresentation.Detail.Fields) != 0 ||
		len(manifest.Actions) != 0 || len(manifest.DefaultPresentation.Actions) != 0 {
		t.Fatalf("core user page exposed form/detail/actions: %#v", manifest.DefaultPresentation)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal(manifest) error = %v", err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"password", "confirmPassword", "roleID", "departmentID", "postID", "root",
		"token", "session", "oauth", "permissionAction", "method", "url", "import", "guard",
	} {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			t.Errorf("normalized core manifest contains forbidden %q: %s", forbidden, raw)
		}
	}
	if !strings.Contains(string(raw), `"requiredPermissions":["/users"]`) {
		t.Fatalf("compiled Foundation permission is missing: %s", raw)
	}
}

func TestCorePresentationSourceRejectsAuthorityAndExecutableProperties(t *testing.T) {
	data, sourcePath := readUserListCorePresentationSource(t)
	for _, property := range []string{"permission", "route", "url", "method", "import", "guard"} {
		t.Run(property, func(t *testing.T) {
			mutated := strings.Replace(string(data), "  binding: administration.users\n", "  binding: administration.users\n  "+property+": forbidden\n", 1)
			if _, err := ParseCorePagePresentation([]byte(mutated), sourcePath); err == nil || !strings.Contains(err.Error(), "field "+property+" not found") {
				t.Fatalf("ParseCorePagePresentation(%s) error = %v", property, err)
			}
		})
	}
}

func TestCorePresentationSourceRejectsSensitiveAndRelationshipFields(t *testing.T) {
	data, sourcePath := readUserListCorePresentationSource(t)
	for _, field := range []string{
		"password", "confirmPassword", "roleID", "departmentID", "postID", "root",
		"accessToken", "sessionID", "oauthProvider",
	} {
		t.Run(field, func(t *testing.T) {
			mutated := strings.Replace(string(data), "fields: [name, status]", "fields: ["+field+", status]", 1)
			_, err := ParseCorePagePresentation([]byte(mutated), sourcePath)
			validation, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("ParseCorePagePresentation(%s) error = %T %v", field, err, err)
			}
			if !hasSpecIssue(validation.Issues, "sensitive-field-forbidden") {
				t.Fatalf("ParseCorePagePresentation(%s) issues = %#v", field, validation.Issues)
			}
		})
	}
}

func TestCorePresentationSourceRequiresExplicitEmptyNonListSurfaces(t *testing.T) {
	data, sourcePath := readUserListCorePresentationSource(t)
	for _, property := range []string{"form", "detail", "actions"} {
		t.Run(property, func(t *testing.T) {
			mutated := strings.Replace(string(data), "  "+property+": []\n", "", 1)
			_, err := ParseCorePagePresentation([]byte(mutated), sourcePath)
			validation, ok := err.(*ValidationError)
			if !ok || !hasSpecIssue(validation.Issues, "required") {
				t.Fatalf("ParseCorePagePresentation(missing %s) error = %T %v", property, err, err)
			}
		})
	}
}

func hasSpecIssue(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func readUserListCorePresentationSource(t *testing.T) ([]byte, string) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve core presentation test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	sourcePath := filepath.ToSlash(filepath.Join(".mss", "core-pages", "user-list.yaml"))
	data, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(sourcePath)))
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	return data, sourcePath
}
