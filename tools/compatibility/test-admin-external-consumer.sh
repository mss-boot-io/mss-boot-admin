#!/usr/bin/env bash
set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
admin_root="${repository_root}/admin"
framework_root="${repository_root}/mss-boot"
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/mss-admin-consumer.XXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT HUP INT TERM

cat >"${work_dir}/go.mod" <<EOF
module example.com/mss-admin-external-consumer

go 1.26.6

require github.com/mss-boot-io/mss-boot-admin/admin v0.0.0

replace github.com/mss-boot-io/mss-boot-admin/admin => ${admin_root}
replace github.com/mss-boot-io/mss-boot-admin/mss-boot => ${framework_root}
EOF

cat >"${work_dir}/main.go" <<'EOF'
package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	adminapp "github.com/mss-boot-io/mss-boot-admin/admin/app"
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"gorm.io/gorm"
)

type externalRecord struct{}

func (*externalRecord) TableName() string { return "external_records" }

type externalModule struct{}

func (externalModule) Name() string { return "external" }

func (externalModule) Register(registry *business.Registry) error {
	return registry.Register(business.Registration{
		Descriptor: business.Descriptor{
			Name:        "external",
			DisplayName: "External",
			Model:       new(externalRecord),
		},
		Readiness: func(context.Context, *gorm.DB) error { return nil },
		Routes: func(group *gin.RouterGroup, _ business.Runtime) error {
			group.GET("/external-probe", func(ctx *gin.Context) { ctx.Status(204) })
			return nil
		},
	})
}

func main() {
	if err := adminapp.ExecuteContext(
		context.Background(),
		adminapp.WithBusinessModules(externalModule{}),
	); err != nil {
		log.Fatal(err)
	}
}
EOF

cat >"${work_dir}/main_test.go" <<'EOF'
package main

import (
	"testing"

	adminapp "github.com/mss-boot-io/mss-boot-admin/admin/app"
)

func TestExternalApplicationConstruction(t *testing.T) {
	application, err := adminapp.New(adminapp.WithBusinessModules(externalModule{}))
	if err != nil {
		t.Fatalf("construct external Admin application: %v", err)
	}
	descriptors := application.BusinessDescriptors()
	if len(descriptors) != 1 || descriptors[0].Name != "external" {
		t.Fatalf("external descriptors = %#v", descriptors)
	}
}
EOF

cd -- "$work_dir"
export GOWORK=off
go mod tidy
go test ./...
go vet ./...
go build -trimpath -o "${work_dir}/mss-admin-consumer" .
"${work_dir}/mss-admin-consumer" --help >/dev/null

printf 'external Admin consumer passed: test, vet, build, --help (GOWORK=off)\n'

cd -- "$repository_root"
bash tools/compatibility/test-admin-command-boundary.sh
