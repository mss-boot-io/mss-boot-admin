package app

import (
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestUpgradeApplicationDefersRootModuleIdentityToManifest(t *testing.T) {
	context := &project.Context{
		Project: project.ProjectDocument{
			Metadata: project.Metadata{
				Name:        "customer-admin",
				DisplayName: "Customer Administration",
				Repository:  "acme/customer-admin",
			},
			Spec: project.ProjectSpec{
				Backend: project.BackendSpec{
					Module: "github.com/acme/customer-admin/admin",
				},
			},
		},
	}

	application := upgradeApplication(context)
	if application.Name != "customer-admin" ||
		application.DisplayName != "Customer Administration" ||
		application.Repository != "acme/customer-admin" {
		t.Fatalf("upgrade application = %#v", application)
	}
	if application.Module != "" {
		t.Fatalf("upgrade must not use nested backend module as root identity: %q", application.Module)
	}
}

func TestUpgradeApplicationHandlesNilContext(t *testing.T) {
	if application := upgradeApplication(nil); application.Name != "" || application.Module != "" {
		t.Fatalf("nil project context produced %#v", application)
	}
}
