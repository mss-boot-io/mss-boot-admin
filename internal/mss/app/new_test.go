package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
)

func TestGenerateApplicationUsesEmbeddedSourceInEmptyDirectory(t *testing.T) {
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	working := t.TempDir()
	if err := os.Chdir(working); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDirectory) })

	originalVersion, originalCommit, originalTimestamp := buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp = originalVersion, originalCommit, originalTimestamp
	})
	buildinfo.Version = "v1.3.4"
	buildinfo.Commit = strings.Repeat("a", 40)
	buildinfo.Timestamp = "2026-08-25T12:34:56Z"

	rootOverride := ""
	command := &cobra.Command{}
	plan, err := generateApplication(command, &rootOverride, "", blueprint.Options{
		Destination: "generated-admin",
		Application: blueprint.Application{
			Name:        "generated-admin",
			DisplayName: "Generated Administration",
			Module:      "github.com/acme/generated-admin",
			Repository:  "acme/generated-admin",
		},
		FrontendRegistryURL: appFrontendRegistry(t),
		Write:               true,
	})
	if err != nil {
		t.Fatalf("generateApplication() error = %v", err)
	}
	if plan.Identities.Foundation.Version != "1.3.4" || plan.Destination != filepath.Join(working, "generated-admin") {
		t.Fatalf("standalone application plan = %#v", plan)
	}
	if _, err := os.Stat(filepath.Join(working, "generated-admin", "README.md")); err != nil {
		t.Fatalf("generated README: %v", err)
	}
}

func TestGenerateApplicationDefaultsToEmbeddedSourceFromFoundationSubdirectory(t *testing.T) {
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	foundationSubdirectory := filepath.Clean(filepath.Join(originalDirectory, "..", "blueprint"))
	if err := os.Chdir(foundationSubdirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDirectory) })

	setAppReleaseBuild(t)
	rootOverride := ""
	destination := filepath.Join(t.TempDir(), "subdirectory-admin")
	plan, err := generateApplication(&cobra.Command{}, &rootOverride, "", blueprint.Options{
		Destination: destination,
		Application: blueprint.Application{
			Name:        "subdirectory-admin",
			DisplayName: "Subdirectory Administration",
			Module:      "github.com/acme/subdirectory-admin",
			Repository:  "acme/subdirectory-admin",
		},
		FrontendRegistryURL: appFrontendRegistry(t),
	})
	if err != nil {
		t.Fatalf("generateApplication() from Foundation subdirectory error = %v", err)
	}
	if plan.Identities.Foundation.Commit != strings.Repeat("a", 40) || plan.Identities.Foundation.Source != ".mss/release-policy.yaml" {
		t.Fatalf("subdirectory generation did not use embedded release identity: %#v", plan.Identities.Foundation)
	}
}

func appFrontendRegistry(t *testing.T) string {
	t.Helper()
	integrity := "sha512-" + strings.Repeat("A", 86) + "=="
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"name":"@mss-boot-io/admin-web","version":"1.3.4","dist":{"integrity":%q}}`, integrity)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func setAppReleaseBuild(t *testing.T) {
	t.Helper()
	originalVersion, originalCommit, originalTimestamp := buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp = originalVersion, originalCommit, originalTimestamp
	})
	buildinfo.Version = "v1.3.4"
	buildinfo.Commit = strings.Repeat("a", 40)
	buildinfo.Timestamp = "2026-08-25T12:34:56Z"
}
