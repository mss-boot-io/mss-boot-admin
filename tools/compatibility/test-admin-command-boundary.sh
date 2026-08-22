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

expect_compile_failure \
  public-root-command \
  'undefined: admincmd.New' \
  'package main

import admincmd "github.com/mss-boot-io/mss-boot-admin/admin/cmd"

func main() { _ = admincmd.New }'

expect_compile_failure \
  public-server-command \
  'undefined: server.NewCommand' \
  'package main

import server "github.com/mss-boot-io/mss-boot-admin/admin/cmd/server"

func main() { _ = server.NewCommand }'

expect_compile_failure \
  public-migrate-command \
  'undefined: migrate.RunContext' \
  'package main

import migrate "github.com/mss-boot-io/mss-boot-admin/admin/cmd/migrate"

func main() { _ = migrate.RunContext }'

printf 'external Admin command boundary passed: guarded help and four negative compilation contracts\n'
