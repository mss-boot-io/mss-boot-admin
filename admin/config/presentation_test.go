package config

import (
	"slices"
	"testing"

	adminpresentation "github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"gopkg.in/yaml.v3"
)

func TestBasePresentationActivatesExactlyFoundationCorePages(t *testing.T) {
	data, err := FS.ReadFile("application.yml")
	if err != nil {
		t.Fatalf("read base configuration: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode base configuration: %v", err)
	}

	wantActivePages := []string{
		"user.list",
		"role.list",
		"menu.list",
		"department.list",
		"post.list",
		"task.list",
		"notice.list",
		"language.list",
		"option.list",
		"system-config.list",
		"online-session.list",
		"log.login",
		"log.audit",
		"log.runtime",
	}
	if cfg.Presentation.AdoptionMode != adminpresentation.AdoptionActive {
		t.Fatalf("base presentation adoption mode = %q, want %q", cfg.Presentation.AdoptionMode, adminpresentation.AdoptionActive)
	}
	if !slices.Equal(cfg.Presentation.ActivePages, wantActivePages) {
		t.Fatalf("base presentation active pages = %#v, want exact Foundation core inventory %#v", cfg.Presentation.ActivePages, wantActivePages)
	}
}
