#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=tools/compatibility/process-groups.sh
source "${script_dir}/process-groups.sh"

usage() {
  cat <<'EOF'
Usage: test-thin-host-external-consumer.sh [options]

Generate and qualify one real external Thin Host without publishing anything.

Options:
  --foundation-root PATH  Foundation checkout (default: current Git root)
  --tarball PATH          Pre-built @mss-boot-io/admin-web tarball
  --report-dir PATH       Persist sanitized reports in this directory
  --help                  Show this help
EOF
}

foundation_root=""
tarball=""
report_dir=""
while (($#)); do
  case "$1" in
    --foundation-root)
      [[ $# -ge 2 ]] || { echo "--foundation-root requires a value" >&2; exit 2; }
      foundation_root="$2"
      shift 2
      ;;
    --tarball)
      [[ $# -ge 2 ]] || { echo "--tarball requires a value" >&2; exit 2; }
      tarball="$2"
      shift 2
      ;;
    --report-dir)
      [[ $# -ge 2 ]] || { echo "--report-dir requires a value" >&2; exit 2; }
      report_dir="$2"
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${foundation_root}" ]]; then
  foundation_root="$(git rev-parse --show-toplevel)"
fi
foundation_root="$(realpath -- "${foundation_root}")"
[[ -d "${foundation_root}/.git" || -f "${foundation_root}/.git" ]] || {
  echo "Foundation root is not a Git checkout: ${foundation_root}" >&2
  exit 1
}
[[ -f "${foundation_root}/cmd/mss/main.go" ]] || {
  echo "Foundation root does not contain cmd/mss" >&2
  exit 1
}

expected_pnpm_version='10.34.5'
if command -v corepack >/dev/null 2>&1; then
  pnpm_command=(corepack "pnpm@${expected_pnpm_version}")
elif command -v pnpm >/dev/null 2>&1; then
  pnpm_command=(pnpm)
else
  echo "corepack or pnpm ${expected_pnpm_version} is required for Thin Host qualification" >&2
  exit 1
fi
actual_pnpm_version="$("${pnpm_command[@]}" --version)"
[[ "${actual_pnpm_version}" = "${expected_pnpm_version}" ]] || {
  echo "Thin Host qualification requires pnpm ${expected_pnpm_version}; found ${actual_pnpm_version}" >&2
  exit 1
}

run_pnpm() {
  "${pnpm_command[@]}" "$@"
}

command -v flock >/dev/null 2>&1 || {
  echo "flock is required for collision-safe Thin Host runtime startup" >&2
  exit 1
}

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/mss-thin-host-consumer.XXXXXX")"
chmod 0700 -- "${work_dir}"
backend_pid=""
web_pid=""
registry_pid=""
port_start_lock_fd=""
persist_evidence="${MSS_PERSIST_EVIDENCE:-0}"
[[ "${persist_evidence}" = "0" || "${persist_evidence}" = "1" ]] || {
  echo "MSS_PERSIST_EVIDENCE must be 0 or 1" >&2
  exit 2
}

release_port_start_lock() {
  if [[ -n "${port_start_lock_fd:-}" ]]; then
    flock -u "${port_start_lock_fd}" >/dev/null 2>&1 || true
    exec {port_start_lock_fd}>&-
    port_start_lock_fd=""
  fi
}

cleanup() {
  local status=$?
  trap - EXIT HUP INT TERM
  mss_stop_process_group "${web_pid}" || true
  mss_stop_process_group "${backend_pid}" || true
  if [[ -n "${registry_pid}" ]]; then
    kill "${registry_pid}" >/dev/null 2>&1 || true
    wait "${registry_pid}" >/dev/null 2>&1 || true
  fi
  release_port_start_lock
  chmod -R u+w -- "${work_dir}" >/dev/null 2>&1 || true
  rm -rf -- "${work_dir}"
  exit "${status}"
}

trap cleanup EXIT HUP INT TERM
host_root="${work_dir}/compatibility-admin"
artifact_dir="${work_dir}/artifacts"
raw_report_dir="${artifact_dir}/raw-reports"
mkdir -p -- "${raw_report_dir}"

foundation_commit="$(git -C "${foundation_root}" rev-parse 'HEAD^{commit}')"
[[ "${foundation_commit}" =~ ^[0-9a-f]{40}$ ]] || {
  echo "Foundation commit is not a full SHA" >&2
  exit 1
}
report_is_persistent="0"

if [[ -n "${report_dir}" ]]; then
  report_dir="$(realpath -m -- "${report_dir}")"
  case "${report_dir}/" in
    "${foundation_root}/"*)
      echo "Compatibility reports must be written outside the Foundation checkout" >&2
      exit 1
      ;;
  esac
  [[ "${report_dir}" != "/" ]] || {
    echo "Compatibility reports cannot target the filesystem root" >&2
    exit 1
  }
  if [[ -d "${report_dir}" ]] && [[ -n "$(find "${report_dir}" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    echo "Compatibility report directory must be empty: ${report_dir}" >&2
    exit 1
  fi
  mkdir -p -- "${report_dir}"
  report_is_persistent="1"
elif [[ "${persist_evidence}" = "1" ]]; then
  report_dir="$(mktemp -d "${TMPDIR:-/tmp}/mss-thin-host-evidence.${foundation_commit:0:12}.XXXXXX")"
  chmod 0700 -- "${report_dir}"
  report_is_persistent="1"
else
  report_dir="${artifact_dir}/reports"
  mkdir -p -- "${report_dir}"
fi
if [[ "${report_is_persistent}" = "1" ]]; then
  printf '%s\n' "external Thin Host persistent evidence directory: ${report_dir}"
fi
source_repository="${GITHUB_REPOSITORY:-mss-boot-io/mss-boot-admin}"
distribution_version="$(
  python3 - "${foundation_root}" <<'PY'
import sys
from pathlib import Path

root = Path(sys.argv[1])
sys.path.insert(0, str(root / 'tools' / 'release'))
from check_release_policy import load_policy

print(load_policy(root / '.mss' / 'release-policy.yaml')['distributionVersion'])
PY
)"
frontend_distribution_version="${distribution_version#v}"
[[ "${frontend_distribution_version}" != "${distribution_version}" ]] || {
  echo "Distribution version must retain its v prefix" >&2
  exit 1
}

if [[ -z "${tarball}" ]]; then
  pack_dir="${artifact_dir}/package"
  package_source="${artifact_dir}/admin-web-source"
  mkdir -p -- "${pack_dir}" "${package_source}"
  git -C "${foundation_root}" -c core.autocrlf=false archive --format=tar HEAD:web/antd-v6 \
    | tar -xf - -C "${package_source}"
  (
    cd "${package_source}"
    npm pkg set \
      "version=${frontend_distribution_version}" \
      "gitHead=${foundation_commit}" \
      >/dev/null
    run_pnpm pack --pack-destination "${pack_dir}" >/dev/null
  )
  mapfile -t packed_tarballs < <(find "${pack_dir}" -maxdepth 1 -type f -name '*.tgz' -print)
  [[ ${#packed_tarballs[@]} -eq 1 ]] || {
    echo "pnpm pack must produce exactly one tarball" >&2
    exit 1
  }
  tarball="${packed_tarballs[0]}"
else
  tarball="$(realpath -- "${tarball}")"
fi
[[ -f "${tarball}" ]] || {
  echo "Admin Web tarball does not exist: ${tarball}" >&2
  exit 1
}
tar -xOf "${tarball}" package/package.json \
  | jq -e \
      --arg version "${frontend_distribution_version}" \
      --arg commit "${foundation_commit}" \
      '.name == "@mss-boot-io/admin-web" and .version == $version and .gitHead == $commit' \
      >/dev/null || {
  echo "Admin Web tarball identity does not match the Foundation Distribution" >&2
  exit 1
}

registry_ready="${work_dir}/registry-url"
registry_log="${work_dir}/registry.log"
python3 - "${tarball}" "${frontend_distribution_version}" "${registry_ready}" <<'PY' >"${registry_log}" 2>&1 &
from hashlib import sha512
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import base64
import json
from pathlib import Path
import sys
from urllib.parse import unquote, urlsplit

tarball = Path(sys.argv[1]).resolve()
version = sys.argv[2]
ready = Path(sys.argv[3]).resolve()
package_name = '@mss-boot-io/admin-web'
integrity = 'sha512-' + base64.b64encode(sha512(tarball.read_bytes()).digest()).decode('ascii')
metadata_path = f'/{package_name}/{version}'
tarball_path = f'/{package_name}/-/admin-web-{version}.tgz'

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        path = unquote(urlsplit(self.path).path)
        if path == metadata_path:
            payload = json.dumps({
                'name': package_name,
                'version': version,
                'dist': {
                    'integrity': integrity,
                    'tarball': f'http://127.0.0.1:{self.server.server_port}{tarball_path}',
                },
            }, separators=(',', ':')).encode('utf-8')
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
            return
        if path == tarball_path:
            payload = tarball.read_bytes()
            self.send_response(200)
            self.send_header('Content-Type', 'application/octet-stream')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
            return
        self.send_error(404)

    def log_message(self, _format, *_args):
        return

server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
ready.write_text(f'http://127.0.0.1:{server.server_port}\n', encoding='utf-8')
server.serve_forever()
PY
registry_pid=$!
for _ in $(seq 1 100); do
  [[ -s "${registry_ready}" ]] && break
  kill -0 "${registry_pid}" >/dev/null 2>&1 || {
    sed -n '1,120p' "${registry_log}" >&2
    echo "temporary Admin Web registry exited before readiness" >&2
    exit 1
  }
  sleep 0.05
done
[[ -s "${registry_ready}" ]] || {
  echo "temporary Admin Web registry did not become ready" >&2
  exit 1
}
registry_url=$(<"${registry_ready}")

sanitize_json_report() {
  local input="$1"
  local output="$2"
  python3 - "${input}" "${output}" "${foundation_root}" "${work_dir}" <<'PY'
import json
import os
import sys
from pathlib import Path

source = Path(sys.argv[1])
output = Path(sys.argv[2])
replacements = sorted(
    {
        (sys.argv[3], '<foundation-root>'),
        (sys.argv[4], '<temporary-workspace>'),
        *(
            (value, replacement)
            for value, replacement in (
                (os.environ.get('HOME'), '<home>'),
                (os.environ.get('RUNNER_TEMP'), '<runner-temp>'),
                (os.environ.get('RUNNER_TOOL_CACHE'), '<runner-tool-cache>'),
            )
            if value
        ),
    },
    key=lambda item: len(item[0]),
    reverse=True,
)

payload = json.loads(source.read_text(encoding='utf-8'))

def sanitize(value):
    if isinstance(value, dict):
        return {key: sanitize(nested) for key, nested in value.items()}
    if isinstance(value, list):
        return [sanitize(nested) for nested in value]
    if isinstance(value, str):
        for needle, replacement in replacements:
            value = value.replace(needle, replacement)
        return value
    return value

sanitized = sanitize(payload)
encoded = json.dumps(sanitized, ensure_ascii=False, indent=2, sort_keys=True) + '\n'
for needle, _ in replacements:
    if needle in encoded:
        raise SystemExit(f'known local path remained in sanitized report: {needle}')
output.write_text(encoded, encoding='utf-8')
PY
}

new_app_raw="${raw_report_dir}/new-app.json"
(
  cd "${foundation_root}"
  go run ./cmd/mss new app compatibility-admin \
    --display-name "Compatibility Administration" \
    --module example.com/compatibility-admin \
    --repository example/compatibility-admin \
    --destination "${host_root}" \
    --foundation "${foundation_root}" \
    --contributor-npm-registry "${registry_url}" \
    --write \
    --format json \
    > "${new_app_raw}"
)
sanitize_json_report "${new_app_raw}" "${report_dir}/new-app.json"

for required in \
  cmd/server/main.go \
  internal/modules/all \
  internal/modules/custom/modules.go \
  internal/modules/registry.go \
  web/package.json \
  web/.npmrc \
  web/tsconfig.json \
  web/mss-admin.config.ts \
  web/config/business-routes.ts \
  web/config/business-routes.generated.ts \
  web/src/business/locales/en-US.ts \
  web/src/business/locales/zh-CN.ts \
  web/src/business/route-registrations.ts \
  web/src/business/routes.config.ts \
  web/src/route-registrations.ts \
  .mss/project.yaml \
  .mss/lock.yaml; do
  [[ -e "${host_root}/${required}" ]] || {
    echo "Thin Host is missing required path: ${required}" >&2
    exit 1
  }
done

expected_npmrc='registry=https://registry.npmjs.org/
save-exact=true'
[[ "$(cat -- "${host_root}/web/.npmrc")" = "${expected_npmrc}" ]] || {
  echo "Thin Host web/.npmrc must use the public npm registry without credentials" >&2
  exit 1
}

for forbidden in \
  admin/models \
  admin/service \
  admin/router \
  admin/middleware \
  admin/center \
  mss-boot \
  web/antd-v6 \
  web/src/modules \
  web/src/shared \
  docs/package.json \
  docs/.dumi \
  cmd/mss \
  internal/mss \
  go.work \
  go.work.sum \
  templates/application \
  templates/module \
  tools/release \
  .mss/release-policy.yaml \
  .github/workflows/release.yml \
  .github/workflows/framework-release.yml \
  .github/workflows/admin-release.yml \
  .github/workflows/frontend-v6-release.yml; do
  [[ ! -e "${host_root}/${forbidden}" ]] || {
    echo "Thin Host copied forbidden Foundation source: ${forbidden}" >&2
    exit 1
  }
done

go_mod_json="$(cd "${host_root}" && GOWORK=off go mod edit -json)"
jq -e \
  --arg module 'github.com/mss-boot-io/mss-boot-admin/admin' \
  --arg framework 'github.com/mss-boot-io/mss-boot-admin/mss-boot' \
  --arg version "${distribution_version}" \
  '.Module.Path == "example.com/compatibility-admin" and
   ([.Require[] | select(.Path == $module and .Version == $version)] | length) == 1 and
   ([.Require[] | select(.Path == $framework and .Version == $version)] | length) == 1' \
  <<< "${go_mod_json}" >/dev/null
jq -e \
  --arg version "${frontend_distribution_version}" \
  '.packageManager == "pnpm@10.34.5" and
   .dependencies["@mss-boot-io/admin-web"] == $version and
   .scripts == {
     "dev": "mss-admin-web dev",
     "lint": "mss-admin-web lint",
     "test": "mss-admin-web test",
     "build": "mss-admin-web build"
   } and
   .pnpm.overrides == {
     "react": "19.2.8",
     "react-dom": "19.2.8",
     "antd": "6.6.1",
     "@ant-design/pro-components": "3.1.14-6",
     "@tanstack/react-query": "5.101.4",
     "axios": "0.33.0"
   } and
   (.pnpm | has("patchedDependencies") | not)' \
  "${host_root}/web/package.json" >/dev/null

install -d -m 0755 \
  "${host_root}/internal/modules/compatibilityprobe" \
  "${host_root}/web/src/business"

python3 - "${host_root}" <<'PY'
import sys
from pathlib import Path

root = Path(sys.argv[1])
fixtures = {
    'internal/modules/compatibilityprobe/module.go': r'''package compatibilityprobe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	probeMigrationID              migration.MigrationID = "20260825000100"
	probeAuthorizationMigrationID migration.MigrationID = "20260825000101"
	PermissionRead                                      = "compatibility-probe:read"
	probeMenuPath                                       = "/compatibility-probe"
	probePermissionPath                                 = "/compatibility-probe/permissions/read"
	probeAPIPath                                        = "/admin/api/compatibility-probe"
	probeAuthorizationResource                          = "authorization"
)

var (
	errProbeAuthenticationRequired   = errors.New("compatibility probe authentication required")
	errProbeAuthorizationDenied      = errors.New("compatibility probe authorization denied")
	errProbeAuthorizationUnavailable = errors.New("compatibility probe authorization unavailable")
)

type probeRecord struct {
	ID   uint   `gorm:"primaryKey"`
	Note string `gorm:"size:120;not null"`
}

func (probeRecord) TableName() string { return "compatibility_probes" }

type probeModule struct{}

// Module returns the handwritten module's explicit Admin composition contract.
func Module() business.Module { return probeModule{} }

func (probeModule) Name() string { return "compatibility-probe" }

func (probeModule) Register(registry *business.Registry) error {
	if registry == nil {
		return errors.New("compatibility probe business registry is required")
	}
	return registry.Register(business.Registration{
		Descriptor: business.Descriptor{
			Name:        "compatibility-probe",
			DisplayName: "Compatibility Probe",
			Description: "Handwritten Thin Host extension qualification module.",
			Version:     "v1alpha1",
			Model:       new(probeRecord),
			Permissions: []business.Permission{
				{
					Code:         PermissionRead,
					DisplayName:  "读取兼容性探针",
					Description:  "Read the handwritten Thin Host compatibility probe.",
					DefaultRoles: []string{"admin"},
				},
			},
			Menu: business.Menu{
				Path:          probeMenuPath,
				DisplayName:   "兼容性探针",
				DisplayNameEn: "Compatibility Probe",
				Icon:          "experiment",
				Order:         900,
			},
		},
		Migrations: registerMigrations,
		Readiness:  verifyReadiness,
		Routes:     registerRoutes,
	})
}

func registerMigrations(runner *migration.Migration) error {
	if runner == nil {
		return errors.New("compatibility probe migration runner is required")
	}
	if err := runner.Register(probeMigrationID, func(db *gorm.DB, version string) error {
		if db == nil {
			return errors.New("compatibility probe migration database is required")
		}
		if version != probeMigrationID.String() {
			return errors.New("compatibility probe migration version mismatch")
		}
		if !db.Migrator().HasTable(new(probeRecord)) {
			if err := db.Migrator().CreateTable(new(probeRecord)); err != nil {
				return fmt.Errorf("create compatibility probe table: %w", err)
			}
		}
		if !db.Migrator().HasTable(new(probeRecord)) {
			return errors.New("compatibility probe table is unavailable after migration")
		}
		if err := runner.CreateVersion(db, version); err != nil {
			return fmt.Errorf("record compatibility probe migration: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return runner.Register(probeAuthorizationMigrationID, func(db *gorm.DB, version string) error {
		return migrateAuthorization(db, version, runner)
	})
}

func migrateAuthorization(db *gorm.DB, version string, runner *migration.Migration) error {
	if db == nil {
		return errors.New("compatibility probe authorization migration database is required")
	}
	if version != probeAuthorizationMigrationID.String() {
		return errors.New("compatibility probe authorization migration version mismatch")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		role, err := resolveAuthorizationRootRole(tx)
		if err != nil {
			return err
		}
		menu, err := upsertAuthorizationMenu(tx, authorizationMenuSeed{
			name:       "compatibility-probe",
			path:       probeMenuPath,
			method:     http.MethodGet,
			accessType: adminpkg.MenuAccessType,
			permission: PermissionRead,
			icon:       "experiment",
			sort:       900,
		})
		if err != nil {
			return err
		}
		component, err := upsertAuthorizationMenu(tx, authorizationMenuSeed{
			name:       "compatibility-probe.read",
			path:       probePermissionPath,
			method:     http.MethodGet,
			parentID:   menu.ID,
			accessType: adminpkg.ComponentAccessType,
			permission: PermissionRead,
			hidden:     true,
		})
		if err != nil {
			return err
		}
		if _, err := upsertAuthorizationMenu(tx, authorizationMenuSeed{
			name:       "api.compatibility-probe.read",
			path:       probeAPIPath,
			method:     http.MethodGet,
			parentID:   component.ID,
			accessType: adminpkg.APIAccessType,
			permission: PermissionRead,
			hidden:     true,
		}); err != nil {
			return err
		}
		for _, grant := range []struct {
			accessType adminpkg.AccessType
			path       string
		}{
			{accessType: adminpkg.MenuAccessType, path: probeMenuPath},
			{accessType: adminpkg.ComponentAccessType, path: probePermissionPath},
			{accessType: adminpkg.APIAccessType, path: probeAPIPath},
		} {
			if err := seedAuthorizationRule(tx, role.ID, grant.accessType, grant.path, http.MethodGet); err != nil {
				return err
			}
		}
		if err := advanceAuthorizationRevision(tx, "role", role.ID); err != nil {
			return err
		}
		if err := advanceAuthorizationRevision(tx, "global", ""); err != nil {
			return err
		}
		if err := runner.CreateVersion(tx, version); err != nil {
			return fmt.Errorf("record compatibility probe authorization migration: %w", err)
		}
		return nil
	})
}

func resolveAuthorizationRootRole(tx *gorm.DB) (*models.Role, error) {
	var matches []models.Role
	if err := tx.Unscoped().Where("root = ?", true).Order("id").Limit(2).Find(&matches).Error; err != nil {
		return nil, fmt.Errorf("resolve compatibility probe root role: %w", err)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("compatibility probe requires exactly one root role; found %d", len(matches))
	}
	role := &matches[0]
	if role.DeletedAt.Valid || role.Status != enum.Enabled || !role.Root {
		return nil, errors.New("compatibility probe root role is not active")
	}
	return role, nil
}

type authorizationMenuSeed struct {
	name       string
	path       string
	method     string
	parentID   string
	accessType adminpkg.AccessType
	permission string
	icon       string
	sort       int
	hidden     bool
}

func upsertAuthorizationMenu(tx *gorm.DB, seed authorizationMenuSeed) (*models.Menu, error) {
	query := tx.Unscoped().Where(
		"type = ? AND path = ? AND method = ?",
		seed.accessType,
		seed.path,
		seed.method,
	)
	var matches []models.Menu
	if err := query.Order("id").Limit(2).Find(&matches).Error; err != nil {
		return nil, fmt.Errorf(
			"resolve compatibility probe %s %s %q: %w",
			seed.accessType,
			seed.method,
			seed.path,
			err,
		)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf(
			"compatibility probe %s %s %q is ambiguous",
			seed.accessType,
			seed.method,
			seed.path,
		)
	}
	if len(matches) == 0 {
		menu := &models.Menu{
			Name:       seed.name,
			Path:       seed.path,
			Method:     seed.method,
			ParentID:   seed.parentID,
			Icon:       seed.icon,
			Type:       seed.accessType,
			Permission: seed.permission,
			Status:     enum.Enabled,
			Sort:       seed.sort,
			HideInMenu: seed.hidden,
		}
		if err := tx.Create(menu).Error; err != nil {
			return nil, fmt.Errorf("create compatibility probe %s %q: %w", seed.accessType, seed.path, err)
		}
		return menu, nil
	}
	menu := &matches[0]
	if menu.DeletedAt.Valid {
		return nil, fmt.Errorf(
			"compatibility probe %s %s %q is soft-deleted",
			seed.accessType,
			seed.method,
			seed.path,
		)
	}
	if menu.Name != seed.name || menu.Path != seed.path || menu.Method != seed.method ||
		menu.ParentID != seed.parentID || menu.Icon != seed.icon || menu.Type != seed.accessType ||
		menu.Permission != seed.permission || menu.Status != enum.Enabled ||
		menu.Sort != seed.sort || menu.HideInMenu != seed.hidden {
		return nil, fmt.Errorf(
			"compatibility probe %s %s %q metadata does not match the owned contract",
			seed.accessType,
			seed.method,
			seed.path,
		)
	}
	return menu, nil
}

func seedAuthorizationRule(tx *gorm.DB, roleID string, accessType adminpkg.AccessType, path, method string) error {
	if strings.TrimSpace(roleID) == "" {
		return errors.New("compatibility probe authorization role ID is empty")
	}
	policy := tx.Model(&models.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p",
		roleID,
		accessType.String(),
		path,
		method,
	)
	var count int64
	if err := policy.Count(&count).Error; err != nil {
		return fmt.Errorf("count compatibility probe %s %s %s: %w", accessType, method, path, err)
	}
	if count > 1 {
		return fmt.Errorf("compatibility probe %s %s %s policy is ambiguous", accessType, method, path)
	}
	if count == 0 {
		rule := &models.CasbinRule{
			PType: "p",
			V0:    roleID,
			V1:    accessType.String(),
			V2:    path,
			V3:    method,
		}
		if err := tx.Create(rule).Error; err != nil {
			return fmt.Errorf("seed compatibility probe %s %s %s: %w", accessType, method, path, err)
		}
	}
	count = 0
	if err := policy.Count(&count).Error; err != nil {
		return fmt.Errorf("seed compatibility probe %s %s %s: %w", accessType, method, path, err)
	}
	if count != 1 {
		return fmt.Errorf("compatibility probe %s %s %s policy count = %d, want 1", accessType, method, path, count)
	}
	return nil
}

func advanceAuthorizationRevision(tx *gorm.DB, scope, ownerID string) error {
	key := &models.ConfigRevision{
		Scope:     scope,
		OwnerID:   ownerID,
		Resource:  probeAuthorizationResource,
		Revision:  0,
		UpdatedAt: time.Now(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(key).Error; err != nil {
		return fmt.Errorf("ensure compatibility probe revision %s/%s: %w", scope, ownerID, err)
	}
	var current models.ConfigRevision
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		scope,
		ownerID,
		probeAuthorizationResource,
	).Take(&current).Error; err != nil {
		return fmt.Errorf("lock compatibility probe revision %s/%s: %w", scope, ownerID, err)
	}
	if current.Revision < 0 || current.Revision == 1<<63-1 {
		return fmt.Errorf("compatibility probe revision cannot advance for %s/%s", scope, ownerID)
	}
	result := tx.Model(&models.ConfigRevision{}).Where(
		"scope = ? AND owner_id = ? AND resource = ? AND revision = ?",
		scope,
		ownerID,
		probeAuthorizationResource,
		current.Revision,
	).Updates(map[string]any{
		"revision":   current.Revision + 1,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return fmt.Errorf("advance compatibility probe revision %s/%s: %w", scope, ownerID, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("compatibility probe revision changed concurrently for %s/%s", scope, ownerID)
	}
	return nil
}

func verifyReadiness(ctx context.Context, db *gorm.DB) error {
	if ctx == nil {
		return errors.New("compatibility probe readiness context is required")
	}
	if db == nil {
		return errors.New("compatibility probe readiness database is required")
	}
	if err := business.RequireAppliedMigrations(
		ctx,
		db,
		probeMigrationID,
		probeAuthorizationMigrationID,
	); err != nil {
		return fmt.Errorf("compatibility probe migration readiness: %w", err)
	}
	readyDB := db.WithContext(ctx)
	if !readyDB.Migrator().HasTable(new(probeRecord)) {
		return errors.New("compatibility probe table readiness failed")
	}
	var metadata int64
	if err := readyDB.Unscoped().Model(&models.Menu{}).Where(
		"type = ? AND path = ? AND method = ? AND permission = ?",
		adminpkg.APIAccessType,
		probeAPIPath,
		http.MethodGet,
		PermissionRead,
	).Count(&metadata).Error; err != nil || metadata != 1 {
		return errors.New("compatibility probe API authorization metadata readiness failed")
	}
	rootRole, err := resolveAuthorizationRootRole(readyDB)
	if err != nil {
		return fmt.Errorf("compatibility probe root authorization readiness: %w", err)
	}
	var policies int64
	if err := readyDB.Model(&models.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p",
		rootRole.ID,
		adminpkg.APIAccessType.String(),
		probeAPIPath,
		http.MethodGet,
	).Count(&policies).Error; err != nil || policies != 1 {
		return errors.New("compatibility probe API policy readiness failed")
	}
	return nil
}

type probeAuthorizer struct {
	db        *gorm.DB
	principal business.PrincipalResolver
}

func newProbeAuthorizer(db *gorm.DB, principal business.PrincipalResolver) (*probeAuthorizer, error) {
	if db == nil {
		return nil, errors.New("compatibility probe authorization database is required")
	}
	if principal == nil {
		return nil, errors.New("compatibility probe principal resolver is required")
	}
	return &probeAuthorizer{db: db, principal: principal}, nil
}

func (authorizer *probeAuthorizer) Authorize(ctx *gin.Context, permission string) error {
	if authorizer == nil || authorizer.db == nil || authorizer.principal == nil {
		return errProbeAuthorizationUnavailable
	}
	if permission != PermissionRead || ctx == nil || ctx.Request == nil ||
		ctx.Request.Method != http.MethodGet || ctx.FullPath() != probeAPIPath {
		return errProbeAuthorizationDenied
	}
	principal := authorizer.principal(ctx)
	if nilInterface(principal) || strings.TrimSpace(principal.GetRoleID()) == "" {
		return errProbeAuthenticationRequired
	}
	if principal.Root() {
		return nil
	}
	var policies int64
	if err := authorizer.db.WithContext(ctx.Request.Context()).Model(&models.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p",
		principal.GetRoleID(),
		adminpkg.APIAccessType.String(),
		probeAPIPath,
		http.MethodGet,
	).Count(&policies).Error; err != nil {
		return fmt.Errorf("%w: read Admin policy", errProbeAuthorizationUnavailable)
	}
	if policies == 0 {
		return errProbeAuthorizationDenied
	}
	return nil
}

func registerRoutes(group *gin.RouterGroup, runtime business.Runtime) error {
	if group == nil {
		return errors.New("compatibility probe protected route group is required")
	}
	if runtime.RequestDatabase == nil {
		return errors.New("compatibility probe request database resolver is required")
	}
	if runtime.Principal == nil {
		return errors.New("compatibility probe principal resolver is required")
	}
	group.GET("/compatibility-probe", func(ctx *gin.Context) {
		db, ok := runtime.RequestDatabase(ctx.Request.Context())
		if !ok || db == nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
			return
		}
		authorizer, err := newProbeAuthorizer(db, runtime.Principal)
		if err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "authorization unavailable"})
			return
		}
		if err := authorizer.Authorize(ctx, PermissionRead); err != nil {
			status := http.StatusForbidden
			message := "forbidden"
			switch {
			case errors.Is(err, errProbeAuthenticationRequired):
				status = http.StatusUnauthorized
				message = "unauthorized"
			case errors.Is(err, errProbeAuthorizationUnavailable):
				status = http.StatusServiceUnavailable
				message = "authorization unavailable"
			}
			ctx.AbortWithStatusJSON(status, gin.H{"error": message})
			return
		}
		var records int64
		if err := db.WithContext(ctx.Request.Context()).Model(new(probeRecord)).Count(&records).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "probe query failed"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"module":  "compatibility-probe",
			"records": records,
		})
	})
	return nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
''',
    'internal/modules/custom/modules.go': r'''// Package custom registers handwritten business modules that are not generated
// from an AdminModule specification.
package custom

import (
	"example.com/compatibility-admin/internal/modules/compatibilityprobe"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
)

// Modules returns a fresh slice of explicitly registered custom modules.
func Modules() []business.Module {
	return []business.Module{compatibilityprobe.Module()}
}
''',
    'web/src/business/CompatibilityProbe.tsx': r'''export default function CompatibilityProbePage() {
  return (
    <section data-testid="mss-thin-host-handwritten-extension">
      <h1>Handwritten Thin Host extension</h1>
      <p>This page is owned by the downstream application.</p>
    </section>
  );
}
''',
    'web/src/business/locales/en-US.ts': r'''const messages = {
  'menu.compatibility-probe': 'Compatibility Probe',
};

export default messages;
''',
    'web/src/business/locales/zh-CN.ts': r'''const messages = {
  'menu.compatibility-probe': '兼容性探针',
};

export default messages;
''',
    'web/src/business/routes.config.ts': r'''import type { AdminBusinessRoute } from '@mss-boot-io/admin-web/business';

const businessRoutes: AdminBusinessRoute[] = [
  {
    path: '/compatibility-probe',
    component: '@/business/CompatibilityProbe',
    access: 'canAccessRoute',
    permission: '/compatibility-probe',
  },
];

export default businessRoutes;
''',
    'web/src/business/route-registrations.ts': r'''import type { RouteRegistration } from '@mss-boot-io/admin-web/runtime';

const routeRegistrations: readonly RouteRegistration[] = [
  {
    path: '/compatibility-probe',
    serverPaths: ['/compatibility-probe'],
    menuName: 'compatibility-probe',
    permission: '/compatibility-probe',
  },
];

export default routeRegistrations;
''',
    'web/src/route-registrations.load.test.ts': r'''import { describe, expect, it } from 'vitest';

import enUSMessages from './locales/en-US';
import zhCNMessages from './locales/zh-CN';
import routeRegistrations from './route-registrations';

describe('Thin Host route registration facade', () => {
  it('loads the handwritten registration through the managed facade', () => {
    expect(routeRegistrations).toContainEqual({
      path: '/compatibility-probe',
      serverPaths: ['/compatibility-probe'],
      menuName: 'compatibility-probe',
      permission: '/compatibility-probe',
    });
  });

  it('loads the handwritten menu labels through both managed locale facades', () => {
    expect(enUSMessages['menu.compatibility-probe']).toBe('Compatibility Probe');
    expect(zhCNMessages['menu.compatibility-probe']).toBe('兼容性探针');
  });
});
''',
}

for relative, content in fixtures.items():
    target = root / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding='utf-8')
PY

gofmt -w \
  "${host_root}/internal/modules/compatibilityprobe/module.go" \
  "${host_root}/internal/modules/custom/modules.go"

install -D -m 0644 \
  "${foundation_root}/.mss/modules/example-supplier.yaml" \
  "${host_root}/.mss/modules/example-supplier.yaml"

python3 - "${host_root}" "${report_dir}/thin-host-tree.json" <<'PY'
import json
import sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
output = Path(sys.argv[2])
paths = []
for path in sorted(root.rglob('*')):
    relative = path.relative_to(root).as_posix()
    if relative == '.git' or relative.startswith('.git/'):
        continue
    paths.append(relative + ('/' if path.is_dir() else ''))
output.write_text(json.dumps({'schema': 'mss.io/thin-host-tree/v1', 'paths': paths}, indent=2) + '\n')
PY

(
  cd "${foundation_root}"
  module_write_raw="${raw_report_dir}/module-write.json"
  module_check_raw="${raw_report_dir}/module-check.json"
  go run ./cmd/mss --root "${host_root}" module generate \
    .mss/modules/example-supplier.yaml \
    --write \
    --frontend-target antd-v6 \
    --format json \
    > "${module_write_raw}"
  go run ./cmd/mss --root "${host_root}" module generate \
    .mss/modules/example-supplier.yaml \
    --check \
    --frontend-target antd-v6 \
    --format json \
    > "${module_check_raw}"
)
sanitize_json_report "${raw_report_dir}/module-write.json" "${report_dir}/module-write.json"
sanitize_json_report "${raw_report_dir}/module-check.json" "${report_dir}/module-check.json"

tree_digest() {
  python3 - "$1" <<'PY'
import hashlib
import sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
digest = hashlib.sha256()
for path in sorted(item for item in root.rglob('*') if item.is_file()):
    relative = path.relative_to(root).as_posix()
    if relative == '.git' or relative.startswith('.git/'):
        continue
    digest.update(relative.encode())
    digest.update(b'\0')
    digest.update(hashlib.sha256(path.read_bytes()).digest())
print(digest.hexdigest())
PY
}

first_digest="$(tree_digest "${host_root}")"
module_second_write_raw="${raw_report_dir}/module-second-write.json"
(
  cd "${foundation_root}"
  go run ./cmd/mss --root "${host_root}" module generate \
    .mss/modules/example-supplier.yaml \
    --write \
    --frontend-target antd-v6 \
    --format json \
    > "${module_second_write_raw}"
)
sanitize_json_report \
  "${module_second_write_raw}" \
  "${report_dir}/module-second-write.json"
second_digest="$(tree_digest "${host_root}")"
[[ "${first_digest}" = "${second_digest}" ]] || {
  echo "A second module generation changed the Thin Host" >&2
  exit 1
}

handwritten_seam_paths=(
  internal/modules/custom/modules.go
  web/src/business/locales/en-US.ts
  web/src/business/locales/zh-CN.ts
  web/src/business/routes.config.ts
  web/src/business/route-registrations.ts
)

handwritten_seam_digests() {
  python3 - "${host_root}" "${handwritten_seam_paths[@]}" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
digests = {}
for relative in sys.argv[2:]:
    path = root / relative
    if not path.is_file():
        raise SystemExit(f'handwritten extension seam is missing: {relative}')
    digests[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
print(json.dumps(digests, separators=(',', ':'), sort_keys=True))
PY
}

assert_upgrade_preserves_handwritten_seams() {
  local plan_path="$1"
  local expected_dry_run="$2"
  local phase="$3"
  local relative

  jq -e \
    --argjson expected_dry_run "${expected_dry_run}" \
    '.success == true and
     .dryRun == $expected_dry_run and
     (.conflicts | length) == 0' \
    "${plan_path}" >/dev/null || {
    echo "${phase} Thin Host upgrade did not return a successful conflict-free plan" >&2
    exit 1
  }
  for relative in "${handwritten_seam_paths[@]}"; do
    jq -e \
      --arg path "${relative}" \
      '([.changes[] | select(.path == $path and .action == "preserve")] | length) == 1 and
       (.preservedFiles | index($path)) != null' \
      "${plan_path}" >/dev/null || {
      echo "${phase} Thin Host upgrade did not preserve ${relative}" >&2
      exit 1
    }
  done
}

upgrade_seam_digests_before="$(handwritten_seam_digests)"
upgrade_tree_before_plan="$(tree_digest "${host_root}")"
upgrade_plan_raw="${raw_report_dir}/handwritten-upgrade-plan.json"
upgrade_apply_raw="${raw_report_dir}/handwritten-upgrade-apply.json"
upgrade_repeat_plan_raw="${raw_report_dir}/handwritten-upgrade-repeat-plan.json"

echo "external Thin Host stage: plan handwritten extension upgrade"
(
  cd "${foundation_root}"
  go run ./cmd/mss --root "${host_root}" upgrade admin "${distribution_version}" \
    --foundation "${foundation_root}" \
    --contributor-npm-registry "${registry_url}" \
    --format json \
    > "${upgrade_plan_raw}"
)
sanitize_json_report \
  "${upgrade_plan_raw}" \
  "${report_dir}/handwritten-upgrade-plan.json"
assert_upgrade_preserves_handwritten_seams \
  "${upgrade_plan_raw}" \
  true \
  "read-only plan"
[[ "${upgrade_tree_before_plan}" = "$(tree_digest "${host_root}")" ]] || {
  echo "read-only Thin Host upgrade plan changed the downstream application" >&2
  exit 1
}

echo "external Thin Host stage: apply handwritten extension upgrade"
(
  cd "${foundation_root}"
  go run ./cmd/mss --root "${host_root}" upgrade admin "${distribution_version}" \
    --foundation "${foundation_root}" \
    --contributor-npm-registry "${registry_url}" \
    --apply \
    --yes \
    --format json \
    > "${upgrade_apply_raw}"
)
sanitize_json_report \
  "${upgrade_apply_raw}" \
  "${report_dir}/handwritten-upgrade-apply.json"
assert_upgrade_preserves_handwritten_seams \
  "${upgrade_apply_raw}" \
  false \
  "confirmed apply"
upgrade_seam_digests_after_apply="$(handwritten_seam_digests)"
[[ "${upgrade_seam_digests_before}" = "${upgrade_seam_digests_after_apply}" ]] || {
  echo "confirmed Thin Host upgrade changed a handwritten extension seam" >&2
  exit 1
}

upgrade_tree_before_repeat_plan="$(tree_digest "${host_root}")"
echo "external Thin Host stage: repeat handwritten extension upgrade plan"
(
  cd "${foundation_root}"
  go run ./cmd/mss --root "${host_root}" upgrade admin "${distribution_version}" \
    --foundation "${foundation_root}" \
    --contributor-npm-registry "${registry_url}" \
    --format json \
    > "${upgrade_repeat_plan_raw}"
)
sanitize_json_report \
  "${upgrade_repeat_plan_raw}" \
  "${report_dir}/handwritten-upgrade-repeat-plan.json"
assert_upgrade_preserves_handwritten_seams \
  "${upgrade_repeat_plan_raw}" \
  true \
  "repeat read-only plan"
jq -e \
  '([.changes[] |
      select(.action == "create" or
             .action == "update" or
             .action == "delete" or
             .action == "conflict")] | length) == 0 and
   (.conflicts | length) == 0' \
  "${upgrade_repeat_plan_raw}" >/dev/null || {
  echo "repeat Thin Host upgrade plan contains a mutating action or conflict" >&2
  exit 1
}
upgrade_seam_digests_after_repeat="$(handwritten_seam_digests)"
[[ "${upgrade_seam_digests_before}" = "${upgrade_seam_digests_after_repeat}" ]] || {
  echo "repeat Thin Host upgrade plan changed a handwritten extension seam" >&2
  exit 1
}
[[ "${upgrade_tree_before_repeat_plan}" = "$(tree_digest "${host_root}")" ]] || {
  echo "repeat read-only Thin Host upgrade plan changed the downstream application" >&2
  exit 1
}

handwritten_upgrade_summary="${artifact_dir}/handwritten-upgrade.json"
jq -n \
  --arg version "${distribution_version}" \
  --argjson digests "${upgrade_seam_digests_before}" \
  '{
    mode: "same-version-contributor-override",
    requestedDistributionVersion: $version,
    files: ($digests | to_entries | map({
      path: .key,
      sha256: .value,
      planAction: "preserve",
      applyAction: "preserve",
      repeatPlanAction: "preserve",
      byteIdentical: true
    })),
    plan: {
      dryRun: true,
      success: true,
      conflicts: 0,
      applicationByteIdentical: true
    },
    apply: {
      dryRun: false,
      confirmed: true,
      success: true,
      conflicts: 0
    },
    repeatPlan: {
      dryRun: true,
      success: true,
      mutatingChanges: 0,
      conflicts: 0,
      applicationByteIdentical: true
    },
    seamsByteIdentical: true
  }' > "${handwritten_upgrade_summary}"

admin_module='github.com/mss-boot-io/mss-boot-admin/admin'
framework_module='github.com/mss-boot-io/mss-boot-admin/mss-boot'
(
  cd "${host_root}"
  export GOWORK=off
  go mod edit -replace="${admin_module}=${foundation_root}/admin"
  go mod edit -replace="${framework_module}=${foundation_root}/mss-boot"
  go mod tidy
  go test -shuffle=on -count=1 ./...
  go vet ./...
  go build -trimpath -o "${work_dir}/compatibility-admin-server" ./cmd/server
  "${work_dir}/compatibility-admin-server" --help >/dev/null
)

package_version="$(tar -xOf "${tarball}" package/package.json | jq -er '.version')"
python3 "${foundation_root}/tools/release/verify_admin_web_package.py" \
  --tarball "${tarball}" \
  --expected-name '@mss-boot-io/admin-web' \
  --expected-version "${package_version}" \
  --source-repository "${source_repository}" \
  --source-commit "${foundation_commit}" \
  --output "${report_dir}/admin-web-package.json"

qualification_dir="${host_root}/.mss/qualification"
mkdir -p -- "${qualification_dir}"
qualified_tarball="${qualification_dir}/admin-web.tgz"
cp -- "${tarball}" "${qualified_tarball}"

web_root="${host_root}/web"
(
  cd "${web_root}"
  # This local tarball qualification never contacts either public registry.
  run_pnpm add \
    --save-exact \
    --lockfile-only \
    --ignore-scripts \
    "@mss-boot-io/admin-web@file:../.mss/qualification/admin-web.tgz"
  python3 - <<'PY'
import json
from pathlib import Path

import yaml

expected = 'file:../.mss/qualification/admin-web.tgz'
package_name = '@mss-boot-io/admin-web'
manifest = json.loads(Path('package.json').read_text(encoding='utf-8'))
if manifest.get('dependencies', {}).get(package_name) != expected:
    raise SystemExit('external host package.json does not bind Admin Web to the qualified tarball')
lock = yaml.safe_load(Path('pnpm-lock.yaml').read_text(encoding='utf-8'))
entry = lock.get('importers', {}).get('.', {}).get('dependencies', {}).get(package_name)
if not isinstance(entry, dict):
    raise SystemExit('external host lock importer has no Admin Web dependency')
resolved = entry.get('version')
if entry.get('specifier') != expected or not (
    resolved == expected or (
        isinstance(resolved, str) and resolved.startswith(expected + '(')
    )
):
    raise SystemExit(f'external host lock importer does not bind the qualified tarball: {entry!r}')
PY
  run_pnpm fetch --frozen-lockfile
  run_pnpm install --offline --frozen-lockfile --ignore-scripts
  run_pnpm run lint

  unique_route_registrations="${work_dir}/route-registrations.unique.ts"
  cp -- \
    "${web_root}/src/business/route-registrations.ts" \
    "${unique_route_registrations}"

  assert_business_route_registration_collision() {
    local collision="$1"
    local expected_error="$2"
    local collision_log="${raw_report_dir}/route-registration-${collision}.log"
    local collision_status

    python3 - "${web_root}/src/business/route-registrations.ts" "${collision}" <<'PY'
import sys
from pathlib import Path

target = Path(sys.argv[1])
collision = sys.argv[2]
if collision == 'ui-path':
    registrations = '''[
  {
    path: '/collision-ui',
    serverPaths: ['/collision-server-a'],
    menuName: 'collision-a',
  },
  {
    path: '/collision-ui',
    serverPaths: ['/collision-server-b'],
    menuName: 'collision-b',
  },
]'''
elif collision == 'server-path':
    registrations = '''[
  {
    path: '/collision-ui-a',
    serverPaths: ['/collision-server'],
    menuName: 'collision-a',
  },
  {
    path: '/collision-ui-b',
    serverPaths: ['/collision-server'],
    menuName: 'collision-b',
  },
]'''
else:
    raise SystemExit(f'unsupported route collision: {collision}')

target.write_text(
    "import type { RouteRegistration } from '@mss-boot-io/admin-web/runtime';\n\n"
    f'const routeRegistrations: readonly RouteRegistration[] = {registrations};\n\n'
    'export default routeRegistrations;\n',
    encoding='utf-8',
)
PY

    set +e
    run_pnpm run test -- src/route-registrations.load.test.ts \
      > "${collision_log}" 2>&1
    collision_status=$?
    set -e
    if ((collision_status == 0)); then
      echo "${collision} route-registration collision loaded successfully; wanted fail closed" >&2
      return 1
    fi
    if ! grep -F -- "${expected_error}" "${collision_log}" >/dev/null; then
      echo "${collision} route-registration collision failed without the canonical diagnostic" >&2
      tail -n 120 -- "${collision_log}" >&2 || true
      return 1
    fi
  }

  assert_business_route_registration_collision \
    ui-path \
    'duplicate business UI route path: /collision-ui'
  assert_business_route_registration_collision \
    server-path \
    'duplicate business server route path: /collision-server'
  cp -- \
    "${unique_route_registrations}" \
    "${web_root}/src/business/route-registrations.ts"

  admin_core_collision_config="${web_root}/vitest.admin-core-route-registry.config.mts"
  admin_core_collision_test="${web_root}/src/admin-core-route-registry.load.test.ts"
  python3 - \
    "${admin_core_collision_config}" \
    "${admin_core_collision_test}" <<'PY'
import sys
from pathlib import Path

config = Path(sys.argv[1])
test = Path(sys.argv[2])
config.write_text(
    '''import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { defineConfig } from 'vitest/config';

const require = createRequire(import.meta.url);
const packageRoot = dirname(require.resolve('@mss-boot-io/admin-web/package.json'));
const projectRoot = process.cwd();

export default defineConfig({
  resolve: {
    alias: [
      {
        find: '@mss-admin-business/routes',
        replacement: resolve(projectRoot, 'src/route-registrations.ts'),
      },
      { find: '@mss-admin-core', replacement: resolve(packageRoot, 'src') },
      { find: '@', replacement: resolve(projectRoot, 'src') },
    ],
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    include: ['src/admin-core-route-registry.load.test.ts'],
    maxWorkers: 1,
  },
});
''',
    encoding='utf-8',
)
test.write_text(
    '''import { describe, expect, it } from 'vitest';

import { routeRegistry } from '@mss-admin-core/shared/routes/registry';

describe('packaged Admin core route registry', () => {
  it('loads unique core and handwritten business registrations together', () => {
    expect(routeRegistry.get('/workplace')?.serverPaths).toEqual(['/welcome']);
    expect(routeRegistry.get('/compatibility-probe')).toMatchObject({
      menuName: 'compatibility-probe',
      permission: '/compatibility-probe',
      serverPaths: ['/compatibility-probe'],
    });
  });
});
''',
    encoding='utf-8',
)
PY

  run_pnpm exec vitest run \
    --config "${admin_core_collision_config}" \
    src/admin-core-route-registry.load.test.ts

  assert_admin_core_route_collision() {
    local collision="$1"
    local expected_error="$2"
    local collision_log="${raw_report_dir}/route-registration-admin-core-${collision}.log"
    local collision_status

    python3 - "${web_root}/src/business/route-registrations.ts" "${collision}" <<'PY'
import sys
from pathlib import Path

target = Path(sys.argv[1])
collision = sys.argv[2]
if collision == 'ui-path':
    registration = '''{
    path: '/workplace',
    serverPaths: ['/custom-workplace'],
    menuName: 'custom-workplace',
  }'''
elif collision == 'server-path':
    registration = '''{
    path: '/custom-welcome',
    serverPaths: ['/welcome'],
    menuName: 'custom-welcome',
  }'''
else:
    raise SystemExit(f'unsupported Admin core route collision: {collision}')

target.write_text(
    "import type { RouteRegistration } from '@mss-boot-io/admin-web/runtime';\n\n"
    f'const routeRegistrations: readonly RouteRegistration[] = [{registration}];\n\n'
    'export default routeRegistrations;\n',
    encoding='utf-8',
)
PY

    set +e
    run_pnpm exec vitest run \
      --config "${admin_core_collision_config}" \
      src/admin-core-route-registry.load.test.ts \
      > "${collision_log}" 2>&1
    collision_status=$?
    set -e
    if ((collision_status == 0)); then
      echo "${collision} Admin core route collision loaded successfully; wanted fail closed" >&2
      return 1
    fi
    if ! grep -F -- "${expected_error}" "${collision_log}" >/dev/null; then
      echo "${collision} Admin core route collision failed without the canonical diagnostic" >&2
      tail -n 120 -- "${collision_log}" >&2 || true
      return 1
    fi
  }

  assert_admin_core_route_collision \
    ui-path \
    '[mss-admin] duplicate UI route path "/workplace" between route registrations "workplace" (/workplace) and "custom-workplace" (/workplace).'
  assert_admin_core_route_collision \
    server-path \
    '[mss-admin] duplicate server route path "/welcome" between route registrations "workplace" (/workplace) and "custom-welcome" (/custom-welcome).'
  cp -- \
    "${unique_route_registrations}" \
    "${web_root}/src/business/route-registrations.ts"
  run_pnpm exec vitest run \
    --config "${admin_core_collision_config}" \
    src/admin-core-route-registry.load.test.ts
  rm -- "${admin_core_collision_config}" "${admin_core_collision_test}"

  run_pnpm run test
  run_pnpm run build
  grep -R -F -q -- \
    'mss-thin-host-handwritten-extension' \
    "${web_root}/dist" || {
    echo "Handwritten Thin Host page marker is missing from the production build" >&2
    exit 1
  }
  run_pnpm list --json --depth Infinity \
    > "${artifact_dir}/pnpm-tree.raw.json"
)

python3 - "${host_root}" "${artifact_dir}/pnpm-tree.raw.json" "${report_dir}/runtime-graph.json" <<'PY'
import json
import sys
from pathlib import Path

host = Path(sys.argv[1]).resolve()
tree_path = Path(sys.argv[2])
output = Path(sys.argv[3])
dist_paths = [
    path.relative_to(host).as_posix()
    for path in host.rglob('dist')
    if path.is_dir() and 'node_modules' not in path.parts
]
if dist_paths != ['web/dist']:
    raise SystemExit(f'external Thin Host must produce exactly web/dist, got {dist_paths}')

payload = json.loads(tree_path.read_text())
if not isinstance(payload, list) or len(payload) != 1:
    raise SystemExit('pnpm dependency tree must contain one external host project')
targets = {
    'react',
    'react-dom',
    'antd',
    '@ant-design/pro-components',
    '@tanstack/react-query',
    '@umijs/max',
    'umi',
}
versions = {target: set() for target in targets}

def walk(dependencies):
    if not isinstance(dependencies, dict):
        return
    for name, value in dependencies.items():
        if not isinstance(value, dict):
            continue
        version = value.get('version')
        if name in versions and isinstance(version, str):
            versions[name].add(version)
        walk(value.get('dependencies'))

walk(payload[0].get('dependencies'))
violations = {
    name: sorted(found)
    for name, found in versions.items()
    if len(found) != 1
}
if violations:
    raise SystemExit(f'Admin Web runtime packages must resolve to one version each: {violations}')
sanitized = {
    'schema': 'mss.io/admin-web-runtime-graph/v1',
    'dist': dist_paths,
    'versions': {name: sorted(found) for name, found in sorted(versions.items())},
}
output.write_text(json.dumps(sanitized, indent=2, sort_keys=True) + '\n')
tree_path.unlink()
PY

wait_for_http() {
  local label="$1"
  local url="$2"
  local pid="$3"
  local log_file="$4"
  for _ in {1..180}; do
    if curl --fail --silent --show-error --max-time 2 "${url}" >/dev/null 2>&1; then
      return 0
    fi
    if ! mss_process_group_alive "${pid}"; then
      echo "${label} exited before becoming ready" >&2
      tail -n 80 -- "${log_file}" >&2 || true
      return 1
    fi
    sleep 1
  done
  echo "${label} did not become ready at ${url}" >&2
  tail -n 80 -- "${log_file}" >&2 || true
  return 1
}

port_start_lock_path="${TMPDIR:-/tmp}/mss-thin-host-external-consumer.$(id -u).port-start.lock"
exec {port_start_lock_fd}>"${port_start_lock_path}"
if ! flock -w 600 "${port_start_lock_fd}"; then
  echo "timed out waiting 600s for Thin Host runtime port-start lock: ${port_start_lock_path}" >&2
  exit 1
fi
read -r backend_port web_port < <(python3 - <<'PY'
import socket

sockets = []
try:
    for _ in range(2):
        current = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        current.bind(('127.0.0.1', 0))
        sockets.append(current)
    print(*(current.getsockname()[1] for current in sockets))
finally:
    for current in sockets:
        current.close()
PY
)
[[ "${backend_port}" =~ ^[0-9]+$ && "${web_port}" =~ ^[0-9]+$ ]] || {
  echo "Thin Host qualification could not allocate loopback ports" >&2
  exit 1
}
[[ "${backend_port}" != "${web_port}" ]] || {
  echo "Thin Host qualification allocated the same backend and frontend port" >&2
  exit 1
}
backend_origin="http://127.0.0.1:${backend_port}"
web_origin="http://127.0.0.1:${web_port}"
printf '%s\n' "external Thin Host runtime origins: backend=${backend_origin} frontend=${web_origin}"

runtime_dir="${host_root}/runtime"
mkdir -p -- "${runtime_dir}/config" "${host_root}/.mss/run/antd-v6-e2e"
python3 - \
  "${foundation_root}/admin/config/application-e2e.yml" \
  "${runtime_dir}/config/application.yml" \
  "${backend_port}" \
  "${web_port}" <<'PY'
import sys
from pathlib import Path

source = Path(sys.argv[1])
target = Path(sys.argv[2])
backend_port = sys.argv[3]
web_port = sys.argv[4]
content = source.read_text(encoding='utf-8')
if content.count('127.0.0.1:18080') != 1:
    raise SystemExit('expected exactly one backend address in the E2E configuration')
if content.count('http://127.0.0.1:18001') != 2:
    raise SystemExit('expected exactly two frontend origins in the E2E configuration')
content = content.replace('127.0.0.1:18080', f'127.0.0.1:{backend_port}')
content = content.replace('http://127.0.0.1:18001', f'http://127.0.0.1:{web_port}')
target.write_text(content, encoding='utf-8')
PY
backend_log="${raw_report_dir}/external-backend.log"
migration_log="${raw_report_dir}/external-migrate.log"
web_log="${raw_report_dir}/external-web.log"
playwright_error_log="${raw_report_dir}/external-playwright.stderr.log"
e2e_password=${MSS_E2E_PASSWORD:-}
if [[ -z "${e2e_password}" ]]; then
  e2e_password="MssE2E-A1-$(python3 - <<'PY'
import secrets
print(secrets.token_hex(24))
PY
)"
fi

if ! (
  cd "${runtime_dir}"
  STAGE=e2e \
  CONFIG_PROVIDER=local \
  GIN_MODE=release \
  GOTOOLCHAIN=local \
  MSS_ADMIN_INITIAL_PASSWORD="${e2e_password}" \
    "${work_dir}/compatibility-admin-server" migrate \
      --username "${MSS_E2E_USERNAME:-admin}" \
      --domain "127.0.0.1:${web_port}"
) > "${migration_log}" 2>&1; then
  echo "external Thin Host migration failed" >&2
  tail -n 120 -- "${migration_log}" >&2 || true
  exit 1
fi

echo "external Thin Host stage: start backend"
mss_start_process_group \
  backend_pid \
  "${runtime_dir}" \
  "${backend_log}" \
  "${work_dir}" \
  env \
  -u MSS_ADMIN_INITIAL_PASSWORD \
  -u MSS_E2E_PASSWORD \
  STAGE=e2e \
  CONFIG_PROVIDER=local \
  GIN_MODE=release \
  GOTOOLCHAIN=local \
  "${work_dir}/compatibility-admin-server" server
wait_for_http \
  "external Thin Host backend" \
  "${backend_origin}/healthz" \
  "${backend_pid}" \
  "${backend_log}"

anonymous_probe_status="$(
  curl --silent --show-error \
    --output /dev/null \
    --write-out '%{http_code}' \
    "${backend_origin}/admin/api/compatibility-probe"
)"
[[ "${anonymous_probe_status}" = "401" ]] || {
  echo "handwritten business route anonymous status = ${anonymous_probe_status}, want 401" >&2
  exit 1
}

echo "external Thin Host stage: start frontend"
mss_start_process_group \
  web_pid \
  "${web_root}" \
  "${web_log}" \
  "${work_dir}" \
  env \
  -u MSS_ADMIN_INITIAL_PASSWORD \
  -u MSS_E2E_PASSWORD \
  BROWSER=none \
  MSS_ADMIN_API_TARGET="${backend_origin}" \
  MSS_V6_E2E=1 \
  PORT="${web_port}" \
  REACT_APP_ENV=dev \
  UMI_ENV=dev \
  MOCK=none \
  "${pnpm_command[@]}" run dev
wait_for_http \
  "external Thin Host frontend" \
  "${web_origin}/admin/api/languages/public" \
  "${web_pid}" \
  "${web_log}"
sleep 1
wait_for_http \
  "external Thin Host frontend stability check" \
  "${web_origin}/admin/api/languages/public" \
  "${web_pid}" \
  "${web_log}"
release_port_start_lock

secure_empty_file() {
  install -m 0600 /dev/null "$1"
}

probe_cookie_jar="${work_dir}/compatibility-probe-root.cookies"
probe_login_credentials="${work_dir}/compatibility-probe-root-login.json"
probe_login_response="${raw_report_dir}/compatibility-probe-root-login.json"
probe_response="${raw_report_dir}/compatibility-probe-root.json"
root_mutation_headers="${work_dir}/compatibility-probe-root.headers"
restricted_role_payload="${work_dir}/compatibility-probe-restricted-role.json"
restricted_role_response="${raw_report_dir}/compatibility-probe-restricted-role.json"
restricted_user_payload="${work_dir}/compatibility-probe-restricted-user.json"
restricted_user_response="${raw_report_dir}/compatibility-probe-restricted-user.json"
restricted_login_credentials="${work_dir}/compatibility-probe-restricted-login.json"
restricted_cookie_jar="${work_dir}/compatibility-probe-restricted.cookies"
restricted_login_response="${raw_report_dir}/compatibility-probe-restricted-login.json"
restricted_user_info_response="${raw_report_dir}/compatibility-probe-restricted-user-info.json"
restricted_probe_response="${raw_report_dir}/compatibility-probe-restricted.json"
for sensitive_file in \
  "${probe_cookie_jar}" \
  "${probe_login_credentials}" \
  "${probe_login_response}" \
  "${probe_response}" \
  "${root_mutation_headers}" \
  "${restricted_role_payload}" \
  "${restricted_role_response}" \
  "${restricted_user_payload}" \
  "${restricted_user_response}" \
  "${restricted_login_credentials}" \
  "${restricted_cookie_jar}" \
  "${restricted_login_response}" \
  "${restricted_user_info_response}" \
  "${restricted_probe_response}"; do
  secure_empty_file "${sensitive_file}"
done

printf '%s\n%s\n' \
  "${MSS_E2E_USERNAME:-admin}" \
  "${e2e_password}" \
  | jq -Rn \
    '[inputs] | {username: .[0], password: .[1]}' \
    > "${probe_login_credentials}"
curl --fail --silent --show-error \
  --cookie-jar "${probe_cookie_jar}" \
  --header 'Content-Type: application/json' \
  --header "Origin: ${web_origin}" \
  --data-binary @- \
  --output "${probe_login_response}" \
  "${backend_origin}/admin/api/user/session/login" \
  < "${probe_login_credentials}"
if ! jq -e '.code == 200 and (has("token") | not) and (has("accessToken") | not)' \
  "${probe_login_response}" >/dev/null; then
  echo "handwritten business route login did not return the session-only JSON contract" >&2
  exit 1
fi

probe_csrf_token="$(
  awk 'NF >= 7 && $6 == "mss_csrf" { value = $7 } END { print value }' \
    "${probe_cookie_jar}"
)"
[[ -n "${probe_csrf_token}" ]] || {
  echo "handwritten business route root session is missing the CSRF cookie" >&2
  exit 1
}
printf '%s\n%s\n%s\n' \
  'Content-Type: application/json' \
  "Origin: ${web_origin}" \
  "X-CSRF-Token: ${probe_csrf_token}" \
  > "${root_mutation_headers}"
unset probe_csrf_token

restricted_suffix="$(python3 - <<'PY'
import secrets
print(secrets.token_hex(5))
PY
)"
restricted_role_name="compatibility-restricted-${restricted_suffix}"
printf '%s\n%s\n%s\n' \
  "${restricted_role_name}" \
  'Thin Host compatibility authorization fixture' \
  'enabled' \
  | jq -Rn \
    '[inputs] | {name: .[0], remark: .[1], status: .[2]}' \
    > "${restricted_role_payload}"
restricted_role_create_status="$(curl --fail --silent --show-error \
  --cookie "${probe_cookie_jar}" \
  --header "@${root_mutation_headers}" \
  --data-binary @- \
  --output "${restricted_role_response}" \
  --write-out '%{http_code}' \
  "${backend_origin}/admin/api/roles" \
  < "${restricted_role_payload}")"
[[ "${restricted_role_create_status}" = "201" ]] || {
  echo "handwritten business route restricted role status = ${restricted_role_create_status}, want 201" >&2
  exit 1
}
restricted_role_id="$(jq -er '.id | select(type == "string" and length > 0)' "${restricted_role_response}")"

restricted_username="compatrestricted${restricted_suffix}"
restricted_password="MssRestricted-A1-$(python3 - <<'PY'
import secrets
print(secrets.token_hex(24))
PY
)"
printf '%s\n%s\n%s\n%s\n%s\n%s\n' \
  'Compatibility Restricted' \
  "${restricted_username}@example.test" \
  "${restricted_password}" \
  "${restricted_role_id}" \
  'enabled' \
  "${restricted_username}" \
  | jq -Rn \
    '[inputs] | {
      name: .[0],
      email: .[1],
      password: .[2],
      roleID: .[3],
      status: .[4],
      username: .[5]
    }' \
    > "${restricted_user_payload}"
restricted_user_create_status="$(curl --fail --silent --show-error \
  --cookie "${probe_cookie_jar}" \
  --header "@${root_mutation_headers}" \
  --data-binary @- \
  --output "${restricted_user_response}" \
  --write-out '%{http_code}' \
  "${backend_origin}/admin/api/users" \
  < "${restricted_user_payload}")"
[[ "${restricted_user_create_status}" = "201" ]] || {
  echo "handwritten business route restricted user status = ${restricted_user_create_status}, want 201" >&2
  exit 1
}
jq -e \
  '.id | type == "string" and length > 0' \
  "${restricted_user_response}" >/dev/null || {
  echo "handwritten business route fixture user was not created" >&2
  exit 1
}

printf '%s\n%s\n' \
  "${restricted_username}" \
  "${restricted_password}" \
  | jq -Rn \
    '[inputs] | {username: .[0], password: .[1]}' \
    > "${restricted_login_credentials}"
unset restricted_password
curl --fail --silent --show-error \
  --cookie-jar "${restricted_cookie_jar}" \
  --header 'Content-Type: application/json' \
  --header "Origin: ${web_origin}" \
  --data-binary @- \
  --output "${restricted_login_response}" \
  "${backend_origin}/admin/api/user/session/login" \
  < "${restricted_login_credentials}"
if ! jq -e '.code == 200 and (has("token") | not) and (has("accessToken") | not)' \
  "${restricted_login_response}" >/dev/null; then
  echo "restricted handwritten business route login did not return the session-only JSON contract" >&2
  exit 1
fi
curl --fail --silent --show-error \
  --cookie "${restricted_cookie_jar}" \
  --output "${restricted_user_info_response}" \
  "${backend_origin}/admin/api/user/userInfo"
jq -e \
  --arg role_id "${restricted_role_id}" \
  '.roleID == $role_id and .role.id == $role_id and .role.root == false' \
  "${restricted_user_info_response}" >/dev/null || {
  echo "restricted handwritten business route principal is not the expected non-root role" >&2
  exit 1
}
restricted_probe_status="$(
  curl --silent --show-error \
    --cookie "${restricted_cookie_jar}" \
    --output "${restricted_probe_response}" \
    --write-out '%{http_code}' \
    "${backend_origin}/admin/api/compatibility-probe"
)"
[[ "${restricted_probe_status}" = "403" ]] || {
  echo "handwritten business route restricted status = ${restricted_probe_status}, want 403" >&2
  exit 1
}

curl --fail --silent --show-error \
  --cookie "${probe_cookie_jar}" \
  --output "${probe_response}" \
  "${backend_origin}/admin/api/compatibility-probe"
if ! jq -e \
  '.module == "compatibility-probe" and .records == 0' \
  "${probe_response}" >/dev/null; then
  echo "authenticated handwritten business route did not return the probe JSON contract" >&2
  exit 1
fi

jq -n \
  --slurpfile upgrade "${handwritten_upgrade_summary}" \
  '{
    schema: "mss.io/thin-host-handwritten-extension/v1",
    backend: {
      module: "compatibility-probe",
      permission: "compatibility-probe:read",
      migration: "20260825000100",
      authorizationMigration: "20260825000101",
      authorizationMetadata: true,
      readiness: true,
      anonymousStatus: 401,
      restrictedStatus: 403,
      authorizedRootStatus: 200
    },
    frontend: {
      path: "/compatibility-probe",
      access: "canAccessRoute",
      permission: "/compatibility-probe",
      locales: ["en-US", "zh-CN"],
      productionBundleMarker: true,
      routeRegistrationLoaded: true
    },
    collisions: {
      businessInternal: {
        uiPath: "failed-closed",
        serverPath: "failed-closed"
      },
      adminCore: {
        uiPath: "/workplace",
        uiPathResult: "failed-closed",
        serverPath: "/welcome",
        serverPathResult: "failed-closed",
        source: "installed-admin-web-runtime-import"
      }
    },
    upgrade: $upgrade[0]
  }' > "${report_dir}/handwritten-extension.json"

external_e2e_raw="${raw_report_dir}/external-e2e.json"
echo "external Thin Host stage: run browser qualification"
(
  cd "${foundation_root}/web/antd-v6"
  run_pnpm exec playwright install chromium
)
set +e
(
  cd "${foundation_root}/web/antd-v6"
  CI=true \
  MSS_V6_EXTERNAL_BACKEND=1 \
  MSS_V6_EXTERNAL_SERVER=1 \
  MSS_V6_BASE_URL="${web_origin}" \
  MSS_E2E_API_URL="${web_origin}/admin/api" \
  MSS_E2E_BACKEND_API_URL="${backend_origin}/admin/api" \
  MSS_E2E_USERNAME="${MSS_E2E_USERNAME:-admin}" \
  MSS_E2E_PASSWORD="${e2e_password}" \
    run_pnpm exec playwright test \
      e2e/generated/supplier.spec.ts \
      e2e/permission.spec.ts \
      e2e/parity.spec.ts \
      --project=chromium-desktop \
      --output="${artifact_dir}/playwright-results" \
      --reporter=json
) > "${external_e2e_raw}" 2> "${playwright_error_log}"
playwright_status=$?
set -e

report_status=0
if [[ ! -s "${external_e2e_raw}" ]]; then
  echo "Playwright did not produce its JSON reporter" >&2
  report_status=1
elif ! sanitize_json_report \
  "${external_e2e_raw}" \
  "${report_dir}/external-e2e.json"; then
  echo "Playwright JSON reporter could not be sanitized" >&2
  report_status=1
fi

if [[ -f "${report_dir}/external-e2e.json" ]]; then
  if ! python3 - "${report_dir}/external-e2e.json" <<'PY'
import json
import sys
from pathlib import Path

report = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
required_files = {
    'generated/supplier.spec.ts',
    'permission.spec.ts',
    'parity.spec.ts',
}
required_titles = {
    'uses the HttpOnly session for create, detail, edit, export, and delete',
    '@parity en-US retained pages are responsive and deprecation-free',
    '@parity zh-CN retained pages are responsive and deprecation-free',
    '@parity OAuth login preserves an attempt-bound safe deep link',
    '@permission anonymous direct navigation requires a server session',
    '@permission restricted user cannot access Supplier or online-session menu, route, and API',
}
files = set()
tests = []
titles = []

def visit(suites):
    for suite in suites:
        file_name = suite.get('file')
        if isinstance(file_name, str):
            files.add(file_name)
        for spec in suite.get('specs', []):
            spec_file = spec.get('file')
            if isinstance(spec_file, str):
                files.add(spec_file)
            title = spec.get('title')
            if isinstance(title, str):
                titles.append(title)
            tests.extend(spec.get('tests', []))
        visit(suite.get('suites', []))

visit(report.get('suites', []))
if files != required_files:
    raise SystemExit(f'external E2E reporter files = {sorted(files)}, want {sorted(required_files)}')
if len(titles) != len(required_titles) or set(titles) != required_titles:
    raise SystemExit(
        f'external E2E reporter titles = {sorted(titles)}, want {sorted(required_titles)}'
    )
if not tests:
    raise SystemExit('external E2E reporter contains no tests')
projects = {test.get('projectName') for test in tests}
if projects != {'chromium-desktop'}:
    raise SystemExit(f'external E2E projects = {sorted(projects)!r}')
stats = report.get('stats', {})
for field in ('skipped', 'unexpected', 'flaky'):
    if stats.get(field) != 0:
        raise SystemExit(f'external E2E {field} count = {stats.get(field)!r}')
expected = stats.get('expected')
if not isinstance(expected, int) or expected != len(tests):
    raise SystemExit(f'external E2E expected count = {expected!r}, tests = {len(tests)}')
if report.get('errors'):
    raise SystemExit('external E2E reporter contains top-level errors')
PY
  then
    report_status=1
  fi
fi

if ((playwright_status != 0 || report_status != 0)); then
  echo "external Thin Host Playwright qualification failed" >&2
  tail -n 120 -- "${playwright_error_log}" >&2 || true
  echo "external Thin Host frontend log:" >&2
  tail -n 120 -- "${web_log}" >&2 || true
  echo "external Thin Host backend log:" >&2
  tail -n 120 -- "${backend_log}" >&2 || true
  exit 1
fi

evidence_manifest="$(python3 - "${report_dir}" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
manifest_path = root / 'evidence-manifest.json'
files = []
for path in sorted(
    item for item in root.rglob('*')
    if item.is_file() and item != manifest_path
):
    files.append({
        'path': path.relative_to(root).as_posix(),
        'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
    })
encoded = json.dumps({
    'schema': 'mss.io/thin-host-local-evidence/v1',
    'files': files,
}, separators=(',', ':'), sort_keys=True)
manifest_path.write_text(encoded + '\n', encoding='utf-8')
print(encoded)
PY
)"
printf '%s\n' "external Thin Host evidence: ${evidence_manifest}"
if [[ "${report_is_persistent}" = "1" ]]; then
  printf '%s\n' "external Thin Host persisted evidence manifest: ${report_dir}/evidence-manifest.json"
fi
printf '%s\n' \
  "external Thin Host passed: generation, handwritten backend/frontend extensions, route collision fail-closed, Supplier sync, idempotency, GOWORK=off backend, tarball frontend, one dist/runtime, external Playwright"
