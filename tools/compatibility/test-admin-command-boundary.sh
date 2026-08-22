#!/usr/bin/env bash
set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
admin_root="${repository_root}/admin"
framework_root="${repository_root}/mss-boot"
work_root=$(mktemp -d "${TMPDIR:-/tmp}/mss-admin-command-boundary.XXXXXX")
trap 'rm -rf -- "$work_root"' EXIT HUP INT TERM

write_go_mod() {
  local directory=$1
  cat >"${directory}/go.mod" <<EOF
module example.com/mss-admin-command-boundary

go 1.26.6

require github.com/mss-boot-io/mss-boot-admin/admin v0.0.0

replace github.com/mss-boot-io/mss-boot-admin/admin => ${admin_root}
replace github.com/mss-boot-io/mss-boot-admin/mss-boot => ${framework_root}
EOF
}

positive_dir="${work_root}/guarded"
mkdir -p -- "$positive_dir"
write_go_mod "$positive_dir"
cat >"${positive_dir}/main.go" <<'EOF'
package main

import (
	"context"
	"log"

	adminapp "github.com/mss-boot-io/mss-boot-admin/admin/app"
)

func main() {
	for _, args := range [][]string{
		{"--help"},
		{"server", "--help"},
		{"migrate", "--help"},
	} {
		application, err := adminapp.New()
		if err != nil {
			log.Fatal(err)
		}
		if err := application.ExecuteArgsContext(context.Background(), args); err != nil {
			log.Fatal(err)
		}
	}
}
EOF
(
  cd -- "$positive_dir"
  export GOWORK=off
  go mod tidy
  go test ./...
  go vet ./...
  go build -trimpath -o guarded-admin .
  ./guarded-admin >/dev/null
)

compatibility_dir="${work_root}/deprecated-compatibility"
mkdir -p -- "$compatibility_dir"
write_go_mod "$compatibility_dir"
cat >"${compatibility_dir}/main.go" <<'EOF'
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	adminapp "github.com/mss-boot-io/mss-boot-admin/admin/app"
	admincmd "github.com/mss-boot-io/mss-boot-admin/admin/cmd"
	"github.com/mss-boot-io/mss-boot-admin/admin/cmd/migrate"
	"github.com/mss-boot-io/mss-boot-admin/admin/cmd/server"
)

func requireError(label string, err, target error) {
	if !errors.Is(err, target) {
		panic(fmt.Sprintf("%s error = %v, want %v", label, err, target))
	}
}

func requireInertCommand(label string, command *cobra.Command, target error) {
	if command == nil {
		panic(label + " returned a nil compatibility command")
	}
	if children := command.Commands(); len(children) != 0 {
		panic(fmt.Sprintf("%s exposed %d executable child commands", label, len(children)))
	}
	command.SetArgs(nil)
	requireError(label, command.ExecuteContext(context.Background()), target)
}

func main() {
	application, err := adminapp.New()
	if err != nil {
		panic(err)
	}
	applicationCommand, err := application.Command()
	requireError("app.Command", err, adminapp.ErrCommandTreeUnavailable)
	requireInertCommand(
		"app.Command execution",
		applicationCommand,
		adminapp.ErrCommandTreeUnavailable,
	)

	requireInertCommand("cmd.New", admincmd.New(nil), admincmd.ErrDirectCommandAccess)
	requireError(
		"cmd.ExecuteContext",
		admincmd.ExecuteContext(context.Background(), nil),
		admincmd.ErrDirectCommandAccess,
	)
	requireError("cmd.Execute", admincmd.Execute(), admincmd.ErrDirectCommandAccess)
	requireInertCommand(
		"server.NewCommand",
		server.NewCommand(nil),
		server.ErrDirectCommandAccess,
	)
	if server.StartCmd == nil {
		panic("server.StartCmd did not preserve its v1 source-compatible type")
	}
	requireInertCommand(
		"migrate.NewCommand",
		migrate.NewCommand(nil),
		migrate.ErrDirectCommandAccess,
	)
	requireError("migrate.Run", migrate.Run(), migrate.ErrDirectCommandAccess)
	requireError(
		"migrate.RunContext",
		migrate.RunContext(context.Background()),
		migrate.ErrDirectCommandAccess,
	)
	if migrate.StartCmd == nil {
		panic("migrate.StartCmd did not preserve its v1 source-compatible type")
	}
}
EOF
(
  cd -- "$compatibility_dir"
  export GOWORK=off
  go mod tidy
  go test ./...
  go vet ./...
  go build -trimpath -o deprecated-admin-compatibility .
  ./deprecated-admin-compatibility
)

expect_compile_failure() {
  local case_name=$1
  local expected=$2
  local source=$3
  local case_dir="${work_root}/${case_name}"
  local log_file="${case_dir}/build.log"
  mkdir -p -- "$case_dir"
  write_go_mod "$case_dir"
  printf '%s\n' "$source" >"${case_dir}/main.go"
  if (
    cd -- "$case_dir"
    GOWORK=off GOFLAGS=-mod=mod go build ./...
  ) >"$log_file" 2>&1; then
    printf 'expected external command-boundary compilation to fail: %s\n' "$case_name" >&2
    exit 1
  fi
  if ! grep -F -- "$expected" "$log_file" >/dev/null; then
    printf 'unexpected failure for %s; wanted %q:\n' "$case_name" "$expected" >&2
    cat -- "$log_file" >&2
    exit 1
  fi
}

expect_compile_failure \
  internal-command-tree \
  'use of internal package github.com/mss-boot-io/mss-boot-admin/admin/internal/cmd not allowed' \
  'package main

import admincmd "github.com/mss-boot-io/mss-boot-admin/admin/internal/cmd"

func main() { _ = admincmd.New }'

printf 'external Admin command boundary passed: guarded execution, fail-closed v1 compatibility, and internal-package rejection\n'
