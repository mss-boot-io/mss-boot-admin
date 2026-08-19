#!/usr/bin/env bash
set -euo pipefail

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

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/mss-thin-host-consumer.XXXXXX")"
backend_pid=""
web_pid=""

stop_process_group() {
  local pid="$1"
  [[ -n "${pid}" ]] || return 0
  if kill -0 -- "-${pid}" 2>/dev/null; then
    kill -TERM -- "-${pid}" 2>/dev/null || true
    for _ in {1..50}; do
      kill -0 -- "-${pid}" 2>/dev/null || break
      sleep 0.1
    done
    if kill -0 -- "-${pid}" 2>/dev/null; then
      kill -KILL -- "-${pid}" 2>/dev/null || true
    fi
  fi
  wait "${pid}" 2>/dev/null || true
}

cleanup() {
  local status=$?
  trap - EXIT HUP INT TERM
  stop_process_group "${web_pid}"
  stop_process_group "${backend_pid}"
  rm -rf -- "${work_dir}"
  exit "${status}"
}

trap cleanup EXIT HUP INT TERM
host_root="${work_dir}/compatibility-admin"
artifact_dir="${work_dir}/artifacts"
raw_report_dir="${artifact_dir}/raw-reports"
mkdir -p -- "${raw_report_dir}"

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
else
  report_dir="${artifact_dir}/reports"
  mkdir -p -- "${report_dir}"
fi

foundation_commit="$(git -C "${foundation_root}" rev-parse HEAD^{commit})"
[[ "${foundation_commit}" =~ ^[0-9a-f]{40}$ ]] || {
  echo "Foundation commit is not a full SHA" >&2
  exit 1
}
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
    --write \
    --format json \
    > "${new_app_raw}"
)
sanitize_json_report "${new_app_raw}" "${report_dir}/new-app.json"

for required in \
  cmd/server/main.go \
  internal/modules/all \
  web/package.json \
  web/tsconfig.json \
  web/mss-admin.config.ts \
  web/config/business-routes.generated.ts \
  .mss/project.yaml \
  .mss/lock.yaml; do
  [[ -e "${host_root}/${required}" ]] || {
    echo "Thin Host is missing required path: ${required}" >&2
    exit 1
  }
done

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
  .github/workflows/frontend-v6-release.yml \
  .github/workflows/release-readiness.yml; do
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
     "antd": "6.6.0",
     "@ant-design/pro-components": "3.1.14-6",
     "@tanstack/react-query": "5.101.4",
     "axios": "0.33.0"
   } and
   (.pnpm | has("patchedDependencies") | not)' \
  "${host_root}/web/package.json" >/dev/null

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

if [[ -z "${tarball}" ]]; then
  pack_dir="${artifact_dir}/package"
  package_source="${artifact_dir}/admin-web-source"
  mkdir -p -- "${pack_dir}" "${package_source}"
  git -C "${foundation_root}" archive --format=tar HEAD:web/antd-v6 \
    | tar -xf - -C "${package_source}"
  (
    cd "${package_source}"
    npm pkg set "gitHead=${foundation_commit}" >/dev/null
    corepack pnpm@10.34.5 pack --pack-destination "${pack_dir}" >/dev/null
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
  corepack pnpm@10.34.5 add \
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
  corepack pnpm@10.34.5 install --offline --frozen-lockfile --ignore-scripts
  corepack pnpm@10.34.5 run lint
  corepack pnpm@10.34.5 run test
  corepack pnpm@10.34.5 run build
  corepack pnpm@10.34.5 list --json --depth Infinity \
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
    if ! kill -0 -- "-${pid}" 2>/dev/null; then
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

runtime_dir="${host_root}/runtime"
mkdir -p -- "${runtime_dir}" "${host_root}/.mss/run/antd-v6-e2e"
backend_log="${raw_report_dir}/external-backend.log"
migration_log="${raw_report_dir}/external-migrate.log"
web_log="${raw_report_dir}/external-web.log"
playwright_error_log="${raw_report_dir}/external-playwright.stderr.log"

if ! (
  cd "${runtime_dir}"
  STAGE=e2e \
  CONFIG_PROVIDER=fs \
  GIN_MODE=release \
  GOTOOLCHAIN=local \
    "${work_dir}/compatibility-admin-server" migrate \
      --username "${MSS_E2E_USERNAME:-admin}" \
      --password "${MSS_E2E_PASSWORD:-123456}" \
      --domain "127.0.0.1:18001"
) > "${migration_log}" 2>&1; then
  echo "external Thin Host migration failed" >&2
  tail -n 120 -- "${migration_log}" >&2 || true
  exit 1
fi

(
  cd "${runtime_dir}"
  exec setsid env \
    STAGE=e2e \
    CONFIG_PROVIDER=fs \
    GIN_MODE=release \
    GOTOOLCHAIN=local \
    "${work_dir}/compatibility-admin-server" server
) > "${backend_log}" 2>&1 &
backend_pid=$!
wait_for_http \
  "external Thin Host backend" \
  "http://127.0.0.1:18080/healthz" \
  "${backend_pid}" \
  "${backend_log}"

(
  cd "${web_root}"
  exec setsid env \
    CI=true \
    MSS_ADMIN_API_TARGET=http://127.0.0.1:18080 \
    MSS_V6_E2E=1 \
    PORT=18001 \
    REACT_APP_ENV=dev \
    UMI_ENV=dev \
    MOCK=none \
    corepack pnpm@10.34.5 run dev
) > "${web_log}" 2>&1 &
web_pid=$!
wait_for_http \
  "external Thin Host frontend" \
  "http://127.0.0.1:18001/admin/api/languages/public" \
  "${web_pid}" \
  "${web_log}"

external_e2e_raw="${raw_report_dir}/external-e2e.json"
set +e
(
  cd "${foundation_root}/web/antd-v6"
  CI=true \
  MSS_V6_EXTERNAL_BACKEND=1 \
  MSS_V6_EXTERNAL_SERVER=1 \
  MSS_V6_BASE_URL=http://127.0.0.1:18001 \
  MSS_E2E_API_URL=http://127.0.0.1:18001/admin/api \
  MSS_E2E_BACKEND_API_URL=http://127.0.0.1:18080/admin/api \
  MSS_E2E_USERNAME="${MSS_E2E_USERNAME:-admin}" \
  MSS_E2E_PASSWORD="${MSS_E2E_PASSWORD:-123456}" \
    corepack pnpm@10.34.5 exec playwright test \
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
files = set()
tests = []

def visit(suites):
    for suite in suites:
        file_name = suite.get('file')
        if isinstance(file_name, str):
            files.add(file_name)
        for spec in suite.get('specs', []):
            spec_file = spec.get('file')
            if isinstance(spec_file, str):
                files.add(spec_file)
            tests.extend(spec.get('tests', []))
        visit(suite.get('suites', []))

visit(report.get('suites', []))
if files != required_files:
    raise SystemExit(f'external E2E reporter files = {sorted(files)}, want {sorted(required_files)}')
if not tests:
    raise SystemExit('external E2E reporter contains no tests')
projects = {test.get('projectName') for test in tests}
if projects != {'chromium-desktop'}:
    raise SystemExit(f'external E2E projects = {sorted(projects)!r}')
stats = report.get('stats', {})
expected = stats.get('expected')
if not isinstance(expected, int) or expected != len(tests):
    raise SystemExit(f'external E2E expected count = {expected!r}, tests = {len(tests)}')
for field in ('skipped', 'unexpected', 'flaky'):
    if stats.get(field) != 0:
        raise SystemExit(f'external E2E {field} count = {stats.get(field)!r}')
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
  exit 1
fi

printf '%s\n' \
  "external Thin Host passed: generation, Supplier sync, idempotency, GOWORK=off backend, tarball frontend, one dist/runtime, external Playwright"
