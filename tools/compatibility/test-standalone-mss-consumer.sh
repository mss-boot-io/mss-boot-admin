#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: test-standalone-mss-consumer.sh [options]

Qualify the package-first mss tool and generated Thin Host from an empty,
non-Git directory.

Options:
  --lifecycle         Run networked setup, readonly backend checks, frozen
                      frontend install, lint, test, and build.
  --upgrade           Prove embedded plan/apply/no-op and preservation.
  --next-foundation   Rehearse a traceable next-Foundation three-way upgrade,
                      CLI/MCP/doctor parity, conflict rejection, and all evals.
  --release-artifact  Use MSS_TOOLS_ARCHIVE instead of locally built tools.
  --public-packages   Resolve the published v1.3.7 Go and npm packages through
                      proxy.golang.org and the exact GitHub Packages Admin Web
                      candidate; requires NODE_AUTH_TOKEN. Official npmjs is a
                      later post-publication gate.
  --help              Show this help.

Environment:
  MSS_RELEASE_VERSION     Release version (default: v1.3.7).
  MSS_RELEASE_COMMIT      Full release commit (default: current HEAD).
  MSS_RELEASE_TIMESTAMP   RFC3339 source timestamp (default: current HEAD).
  MSS_TOOLS_ARCHIVE       Versioned mss tools tar.gz or zip for
                          --release-artifact.
  MSS_ADMIN_WEB_TARBALL   Optional exact v1.3.7 Admin Web candidate tarball.
                         When omitted, the script packs the release commit;
                         either candidate is served only from loopback.
  NODE_AUTH_TOKEN         GitHub Packages read token required only with
                         --public-packages; it is never written to a file.
EOF
}

run_lifecycle=false
run_upgrade=false
run_next_foundation=false
use_release_artifact=false
use_public_packages=false
while (($#)); do
  case "$1" in
    --lifecycle)
      run_lifecycle=true
      shift
      ;;
    --upgrade)
      run_upgrade=true
      shift
      ;;
    --next-foundation)
      run_next_foundation=true
      shift
      ;;
    --release-artifact)
      use_release_artifact=true
      shift
      ;;
    --public-packages)
      use_public_packages=true
      shift
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
if [[ "${use_release_artifact}" = true && "${use_public_packages}" = true ]]; then
  echo "--release-artifact and --public-packages are mutually exclusive" >&2
  exit 2
fi
if [[ "${use_public_packages}" = true && -n "${MSS_ADMIN_WEB_TARBALL:-}" ]]; then
  echo "MSS_ADMIN_WEB_TARBALL cannot be used with --public-packages" >&2
  exit 2
fi
if [[ "${run_next_foundation}" = true ]] && { [[ "${use_release_artifact}" = true ]] || [[ "${use_public_packages}" = true ]]; }; then
  echo "--next-foundation is a source qualification and cannot be combined with --release-artifact or --public-packages" >&2
  exit 2
fi

repository_root=$(git rev-parse --show-toplevel)
repository_root=$(realpath -- "${repository_root}")
[[ -f "${repository_root}/embedded_distribution.go" ]] || {
  echo "repository does not contain the embedded Distribution source" >&2
  exit 1
}

release_version=${MSS_RELEASE_VERSION:-v1.3.7}
release_commit=${MSS_RELEASE_COMMIT:-$(git -C "${repository_root}" rev-parse 'HEAD^{commit}')}
release_timestamp=${MSS_RELEASE_TIMESTAMP:-$(git -C "${repository_root}" show -s --format=%cI "${release_commit}")}
[[ "${release_version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]] || {
  echo "release version must be v-prefixed semantic: ${release_version}" >&2
  exit 1
}
[[ "${release_commit}" =~ ^[0-9a-f]{40}$ ]] || {
  echo "release commit must be a full lower-case SHA" >&2
  exit 1
}
python3 - "${release_timestamp}" <<'PY'
from datetime import datetime
import sys

try:
    datetime.fromisoformat(sys.argv[1].replace('Z', '+00:00'))
except ValueError as error:
    raise SystemExit(f'release timestamp must be RFC3339: {error}')
PY

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/mss-standalone-consumer.XXXXXX")
case "${work_dir}" in
  "${TMPDIR:-/tmp}"/mss-standalone-consumer.*) ;;
  *) echo "unsafe temporary directory: ${work_dir}" >&2; exit 1 ;;
esac
registry_pid=""
cleanup() {
  local status=$?
  trap - EXIT HUP INT TERM
  if [[ -n "${mss:-}" && -x "${mss}" && -n "${host_root:-}" && -f "${host_root}/.mss/dev.yaml" ]]; then
    "${mss}" --root "${host_root}" dev stop --force >/dev/null 2>&1 || true
  fi
  if [[ -n "${registry_pid}" ]]; then
    kill "${registry_pid}" >/dev/null 2>&1 || true
    wait "${registry_pid}" >/dev/null 2>&1 || true
  fi
  case "${work_dir}" in
    "${TMPDIR:-/tmp}"/mss-standalone-consumer.*)
      chmod -R u+w -- "${work_dir}" >/dev/null 2>&1 || true
      rm -rf -- "${work_dir}"
      ;;
    *) echo "refusing to remove unsafe temporary directory: ${work_dir}" >&2 ;;
  esac
  exit "${status}"
}
trap cleanup EXIT HUP INT TERM

tool_dir="${work_dir}/tools"
candidate_bin="${work_dir}/go-install-bin"
consumer_parent="${work_dir}/empty"
host_root="${consumer_parent}/standalone-admin"
candidate_source="${work_dir}/candidate-source"
candidate_archive="${work_dir}/candidate-source.tar"
mkdir -p -- "${tool_dir}" "${candidate_bin}" "${consumer_parent}" "${candidate_source}"
git -C "${repository_root}" archive \
  --format=tar \
  --output="${candidate_archive}" \
  "${release_commit}"
tar -xf "${candidate_archive}" -C "${candidate_source}"
[[ -f "${candidate_source}/embedded_distribution.go" ]] || {
  echo "release commit archive does not contain the embedded Distribution source" >&2
  exit 1
}
if git -C "${consumer_parent}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "standalone consumer parent unexpectedly belongs to a Git worktree" >&2
  exit 1
fi

ldflags="-X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Version=${release_version} -X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Commit=${release_commit} -X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Timestamp=${release_timestamp}"

if [[ "${use_release_artifact}" = true ]]; then
  archive=${MSS_TOOLS_ARCHIVE:-}
  [[ -n "${archive}" ]] || {
    echo "MSS_TOOLS_ARCHIVE is required with --release-artifact" >&2
    exit 1
  }
  archive=$(realpath -- "${archive}")
  [[ -f "${archive}" ]] || { echo "tools archive does not exist: ${archive}" >&2; exit 1; }
  case "${archive}" in
    *.tar.gz) tar -xzf "${archive}" -C "${tool_dir}" ;;
    *.zip) unzip -q "${archive}" -d "${tool_dir}" ;;
    *) echo "unsupported tools archive: ${archive}" >&2; exit 1 ;;
  esac
  mss=$(find "${tool_dir}" -type f \( -name mss -o -name mss.exe \) -print -quit)
  mss_mcp=$(find "${tool_dir}" -type f \( -name mss-mcp -o -name mss-mcp.exe \) -print -quit)
  [[ -n "${mss}" && -n "${mss_mcp}" ]] || {
    echo "tools archive must contain mss and mss-mcp" >&2
    exit 1
  }
  chmod u+x -- "${mss}" "${mss_mcp}"
else
  mss="${tool_dir}/mss"
  mss_mcp="${tool_dir}/mss-mcp"
  (
    cd "${candidate_source}"
    GOWORK=off go build -mod=mod -trimpath -ldflags "${ldflags}" -o "${mss}" ./cmd/mss
    GOWORK=off go build -mod=mod -trimpath -ldflags "${ldflags}" -o "${mss_mcp}" ./cmd/mss-mcp
  )
fi

assert_tool_identity() {
  local executable=$1
  shift
  local output
  output=$("${executable}" "$@")
  [[ "${output}" == *"${release_version}"* ]] || { echo "tool version is missing from: ${output}" >&2; exit 1; }
  [[ "${output}" == *"${release_commit}"* ]] || { echo "tool commit is missing from: ${output}" >&2; exit 1; }
  [[ "${output}" == *"${release_timestamp}"* ]] || { echo "tool timestamp is missing from: ${output}" >&2; exit 1; }
}
assert_tool_identity "${mss}" --version
assert_tool_identity "${mss_mcp}" --version

# Build a standards-shaped local file proxy from the candidate source, then
# qualify the same module paths users can install after v1.3.7 is public. The
# release installer remains the recommended acquisition path because it also
# verifies the immutable tools archive checksum and BUILD-INFO manifest.
if [[ "${use_public_packages}" = false ]]; then
  proxy_root="${work_dir}/proxy"
  python3 - "${candidate_source}" "${proxy_root}" "${release_version}" "${release_timestamp}" <<'PY'
import json
from pathlib import Path
import stat
import sys
import zipfile

repository = Path(sys.argv[1]).resolve()
proxy = Path(sys.argv[2]).resolve()
version = sys.argv[3]
timestamp = sys.argv[4]
repository_paths = []
for path in repository.rglob('*'):
    try:
        mode = path.lstat().st_mode
    except FileNotFoundError:
        continue
    if stat.S_ISREG(mode) or stat.S_ISLNK(mode):
        repository_paths.append(path.relative_to(repository).as_posix())
modules = (
    ('github.com/mss-boot-io/mss-boot-admin', ''),
    ('github.com/mss-boot-io/mss-boot-admin/admin', 'admin'),
    ('github.com/mss-boot-io/mss-boot-admin/mss-boot', 'mss-boot'),
)

for module, source_prefix in modules:
    source = repository / source_prefix
    destination = proxy.joinpath(*module.split('/'), '@v')
    destination.mkdir(parents=True, exist_ok=True)
    (destination / f'{version}.mod').write_bytes((source / 'go.mod').read_bytes())
    (destination / 'list').write_text(version + '\n', encoding='utf-8')
    (destination / f'{version}.info').write_text(
        json.dumps({'Version': version, 'Time': timestamp}, separators=(',', ':')) + '\n',
        encoding='utf-8',
    )

    prefix_with_slash = source_prefix + '/' if source_prefix else ''
    relative_paths = []
    for repository_relative in repository_paths:
        if prefix_with_slash and not repository_relative.startswith(prefix_with_slash):
            continue
        relative = repository_relative.removeprefix(prefix_with_slash)
        if relative:
            relative_paths.append(relative)
    nested_module_roots = tuple(
        relative.removesuffix('go.mod')
        for relative in relative_paths
        if relative != 'go.mod'
        and relative.endswith('/go.mod')
        and stat.S_ISREG((source / relative).lstat().st_mode)
    )
    paths = []
    for relative in relative_paths:
        # A module proxy serves each nested module independently. Match the Go
        # module zip rules by excluding every discovered nested module tree.
        if relative.startswith('vendor/') or relative.startswith(nested_module_roots):
            continue
        path = source / relative
        mode = path.lstat().st_mode
        if stat.S_ISLNK(mode) or not stat.S_ISREG(mode):
            continue
        paths.append(relative)
    inherited_license = False
    repository_license = repository / 'LICENSE'
    if source_prefix and 'LICENSE' not in paths:
        try:
            repository_license_mode = repository_license.lstat().st_mode
        except FileNotFoundError:
            repository_license_mode = 0
        if stat.S_ISREG(repository_license_mode):
            # cmd/go injects the repository-root LICENSE into a module in a
            # subdirectory when that module has no top-level LICENSE.
            paths.append('LICENSE')
            inherited_license = True
    module_prefix = f'{module}@{version}/'
    archive_path = destination / f'{version}.zip'
    with zipfile.ZipFile(archive_path, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
        for relative in sorted(paths):
            info = zipfile.ZipInfo(module_prefix + relative)
            info.date_time = (2026, 1, 1, 0, 0, 0)
            info.external_attr = 0o644 << 16
            file_source = (
                repository_license
                if inherited_license and relative == 'LICENSE'
                else source / relative
            )
            archive.writestr(info, file_source.read_bytes())
    if inherited_license:
        with zipfile.ZipFile(archive_path) as archive:
            inherited = archive.read(module_prefix + 'LICENSE')
        if inherited != repository_license.read_bytes():
            raise SystemExit(f'{module} did not inherit the repository LICENSE')
PY

  GOWORK=off GOPROXY="file://${proxy_root},https://proxy.golang.org" GOSUMDB=off \
    GOMODCACHE="${work_dir}/go-install-modcache" GOCACHE="${work_dir}/go-install-buildcache" \
    GOBIN="${candidate_bin}" go install -trimpath \
    "github.com/mss-boot-io/mss-boot-admin/cmd/mss@${release_version}" \
    "github.com/mss-boot-io/mss-boot-admin/cmd/mss-mcp@${release_version}"
  for installed_tool in "${candidate_bin}/mss" "${candidate_bin}/mss-mcp"; do
    installed_version=$("${installed_tool}" --version)
    [[ "${installed_version}" == *"${release_version}"* ]] || {
      echo "standard go install did not expose module version ${release_version}: ${installed_version}" >&2
      exit 1
    }
  done

  set +e
  (
    cd "${consumer_parent}"
    "${candidate_bin}/mss" new app unsupported-go-install \
      --module example.com/unsupported-go-install \
      --repository example/unsupported-go-install \
      --destination "${consumer_parent}/unsupported-go-install" \
      --format json
  ) > "${work_dir}/go-install-new.stdout" 2> "${work_dir}/go-install-new.stderr"
  go_install_new_status=$?
  set -e
  [[ ${go_install_new_status} -ne 0 ]] || {
    echo "standard go-install binary unexpectedly authorized embedded generation" >&2
    exit 1
  }
  grep -Eq 'official release-built mss' "${work_dir}/go-install-new.stderr"
fi

contributor_registry_args=()
npm_user_config="${work_dir}/npmrc"
expected_admin_web_integrity=""
if [[ "${use_public_packages}" = true ]]; then
  command -v npm >/dev/null 2>&1 || {
    echo "npm is required to qualify the published GitHub Packages Admin Web candidate" >&2
    exit 1
  }
  [[ -n "${NODE_AUTH_TOKEN:-}" ]] || {
    echo "NODE_AUTH_TOKEN is required with --public-packages to read the exact GitHub Packages candidate" >&2
    exit 1
  }
  umask 077
  # Keep the token reference literal; npm expands it from the process
  # environment and the credential is never persisted in this file.
  # shellcheck disable=SC2016
  printf '%s\n' \
    'registry=https://registry.npmjs.org/' \
    '@mss-boot-io:registry=https://npm.pkg.github.com/' \
    '//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}' \
    'always-auth=true' \
    > "${npm_user_config}"
  public_package_output="${work_dir}/public-admin-web-package"
  public_package_metadata="${work_dir}/public-admin-web-metadata.json"
  mkdir -p -- "${public_package_output}"
  NPM_CONFIG_USERCONFIG="${npm_user_config}" npm view \
    "@mss-boot-io/admin-web@${release_version#v}" \
    --json \
    --registry=https://npm.pkg.github.com \
    > "${public_package_metadata}"
  jq -e \
    --arg version "${release_version#v}" \
    --arg commit "${release_commit}" \
    '.name == "@mss-boot-io/admin-web" and
     .version == $version and .gitHead == $commit and
     (.dist.integrity | type == "string" and startswith("sha512-")) and
     (.dist.tarball | type == "string" and startswith("https://") and
      (contains("?") or contains("#")) == false)' \
    "${public_package_metadata}" >/dev/null || {
    echo "published GitHub Packages Admin Web candidate has the wrong identity or provenance" >&2
    exit 1
  }
  expected_admin_web_integrity=$(jq -er '.dist.integrity' "${public_package_metadata}")
  NPM_CONFIG_USERCONFIG="${npm_user_config}" npm pack \
    "@mss-boot-io/admin-web@${release_version#v}" \
    --pack-destination "${public_package_output}" \
    --registry=https://npm.pkg.github.com \
    >/dev/null
  mapfile -t public_tarballs < <(find "${public_package_output}" -maxdepth 1 -type f -name '*.tgz' -print)
  [[ ${#public_tarballs[@]} -eq 1 ]] || {
    echo "GitHub Packages must produce exactly one Admin Web candidate tarball" >&2
    exit 1
  }
  admin_web_tarball=${public_tarballs[0]}
  consumer_go_proxy="https://proxy.golang.org"
  consumer_go_sumdb="sum.golang.org"
else
  admin_web_tarball=${MSS_ADMIN_WEB_TARBALL:-}
  if [[ -z "${admin_web_tarball}" ]]; then
    command -v corepack >/dev/null 2>&1 || {
      echo "corepack is required to build the exact pre-public Admin Web candidate" >&2
      exit 1
    }
    package_source="${work_dir}/admin-web-source"
    package_output="${work_dir}/admin-web-package"
    mkdir -p -- "${package_source}" "${package_output}"
    git -C "${repository_root}" -c core.autocrlf=false archive --format=tar "${release_commit}:web/antd-v6" \
      | tar -xf - -C "${package_source}"
    python3 - "${package_source}/package.json" "${release_version#v}" "${release_commit}" <<'PY'
import json
from pathlib import Path
import sys

path = Path(sys.argv[1]).resolve()
manifest = json.loads(path.read_text(encoding='utf-8'))
if manifest.get('name') != '@mss-boot-io/admin-web' or manifest.get('private') is True:
    raise SystemExit('Admin Web package source has an unexpected identity')
manifest['version'] = sys.argv[2]
manifest['gitHead'] = sys.argv[3]
path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')
PY
    (
      cd "${package_source}"
      corepack pnpm@10.34.5 pack --pack-destination "${package_output}" >/dev/null
    )
    mapfile -t candidate_tarballs < <(find "${package_output}" -maxdepth 1 -type f -name '*.tgz' -print)
    [[ ${#candidate_tarballs[@]} -eq 1 ]] || {
      echo "pnpm pack must produce exactly one Admin Web candidate tarball" >&2
      exit 1
    }
    admin_web_tarball=${candidate_tarballs[0]}
  fi
  consumer_go_proxy="file://${proxy_root},https://proxy.golang.org"
  consumer_go_sumdb="off"
fi
admin_web_tarball=$(realpath -- "${admin_web_tarball}")
[[ -f "${admin_web_tarball}" ]] || {
  echo "Admin Web candidate tarball does not exist: ${admin_web_tarball}" >&2
  exit 1
}
tar -xOf "${admin_web_tarball}" package/package.json \
  | jq -e \
      --arg version "${release_version#v}" \
      --arg commit "${release_commit}" \
      '.name == "@mss-boot-io/admin-web" and .version == $version and .gitHead == $commit' \
      >/dev/null || {
  echo "Admin Web candidate must contain the exact package name, version, and release gitHead" >&2
  exit 1
}
computed_admin_web_integrity=$(python3 - "${admin_web_tarball}" <<'PY'
from hashlib import sha512
import base64
from pathlib import Path
import sys

print('sha512-' + base64.b64encode(sha512(Path(sys.argv[1]).read_bytes()).digest()).decode('ascii'))
PY
)
if [[ -n "${expected_admin_web_integrity}" && "${computed_admin_web_integrity}" != "${expected_admin_web_integrity}" ]]; then
  echo "downloaded GitHub Packages Admin Web bytes do not match dist.integrity" >&2
  exit 1
fi

registry_ready="${work_dir}/registry-url"
registry_log="${work_dir}/registry.log"
fixture_tarball_path="/artifacts/frozen-admin-web-${release_version#v}.tgz"
pnpm_fallback_tarball_path="/@mss-boot-io/admin-web/-/admin-web-${release_version#v}.tgz"
[[ "${fixture_tarball_path}" != "${pnpm_fallback_tarball_path}" ]] || {
  echo "temporary registry tarball path must differ from pnpm's generic registry fallback" >&2
  exit 1
}
python3 - "${admin_web_tarball}" "${release_version#v}" "${release_commit}" "${registry_ready}" "${fixture_tarball_path}" <<'PY' >"${registry_log}" 2>&1 &
from hashlib import sha512
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import base64
import json
from pathlib import Path
import sys
from urllib.parse import unquote, urlsplit

tarball = Path(sys.argv[1]).resolve()
version = sys.argv[2]
commit = sys.argv[3]
ready = Path(sys.argv[4]).resolve()
tarball_path = sys.argv[5]
package_name = '@mss-boot-io/admin-web'
integrity = 'sha512-' + base64.b64encode(sha512(tarball.read_bytes()).digest()).decode('ascii')
metadata_path = f'/{package_name}/{version}'

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        path = unquote(urlsplit(self.path).path)
        if path == metadata_path:
            payload = json.dumps({
                'name': package_name,
                'version': version,
                'gitHead': commit,
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
    echo "temporary candidate npm registry exited before readiness" >&2
    exit 1
  }
  sleep 0.05
done
[[ -s "${registry_ready}" ]] || {
  echo "temporary candidate npm registry did not become ready" >&2
  exit 1
}
registry_url=$(<"${registry_ready}")
contributor_registry_args=(--contributor-npm-registry "${registry_url}")
if [[ "${use_public_packages}" = false ]]; then
  printf 'registry=https://registry.npmjs.org/\n@mss-boot-io:registry=%s/\n' "${registry_url}" > "${npm_user_config}"
fi
consumer_go_modcache="${work_dir}/consumer-modcache"

dry_run_report="${work_dir}/new-app-dry-run.json"
(
  cd "${consumer_parent}"
  "${mss}" new app standalone-admin \
    --display-name "Standalone Administration" \
    --module example.com/standalone-admin \
    --repository example/standalone-admin \
    --destination "${host_root}" \
    "${contributor_registry_args[@]}" \
    --format json > "${dry_run_report}"
)
jq -e --arg expected_version "${release_version#v}" \
  '.dryRun == true and .success == true and .identities.foundation.version == $expected_version' \
  "${dry_run_report}" >/dev/null
[[ ! -e "${host_root}" ]] || { echo "new app dry-run changed the destination" >&2; exit 1; }

write_report="${work_dir}/new-app-write.json"
(
  cd "${consumer_parent}"
  "${mss}" new app standalone-admin \
    --display-name "Standalone Administration" \
    --module example.com/standalone-admin \
    --repository example/standalone-admin \
    --destination "${host_root}" \
    "${contributor_registry_args[@]}" \
    --write --git-init --format json > "${write_report}"
)
jq -e '.dryRun == false and .success == true' "${write_report}" >/dev/null
[[ "$(git -C "${host_root}" symbolic-ref HEAD)" = refs/heads/main ]] || {
  echo "--git-init did not create the expected main branch" >&2
  exit 1
}
for required in \
  README.md AGENTS.md .agents/skills/mss-thin-host/SKILL.md \
  .mss/project.yaml .mss/dev.yaml .mss/lock.yaml .mss/blueprint-manifest.json \
  go.mod go.sum cmd/server/main.go web/package.json web/pnpm-lock.yaml; do
  [[ -f "${host_root}/${required}" ]] || { echo "generated Thin Host is missing ${required}" >&2; exit 1; }
done
for forbidden in cmd/mss internal/mss mss-boot go.work templates/application tools/release; do
  [[ ! -e "${host_root}/${forbidden}" ]] || { echo "generated Thin Host copied Foundation path ${forbidden}" >&2; exit 1; }
done
if grep -ERn 'go run ./cmd/mss|--foundation|__MSS_' "${host_root}/README.md" "${host_root}/AGENTS.md" "${host_root}/.agents" "${host_root}/.mss"; then
  echo "generated package-first instructions contain a checkout-only command or unresolved placeholder" >&2
  exit 1
fi
grep -Fq "releases/download/${release_version}/install-mss.sh" "${host_root}/.github/workflows/ci.yml"
grep -Eq 'mss verify --all' "${host_root}/.github/workflows/ci.yml"
contracts_ci=$(sed -n '/^  contracts:/,/^  backend:/p' "${host_root}/.github/workflows/ci.yml")
grep -Fq 'corepack pnpm@10.34.5 --dir web install --frozen-lockfile' <<< "${contracts_ci}"
install_line=$(grep -Fn 'corepack pnpm@10.34.5 --dir web install --frozen-lockfile' <<< "${contracts_ci}" | cut -d: -f1)
verify_line=$(grep -Fn 'mss verify --all' <<< "${contracts_ci}" | cut -d: -f1)
[[ -n "${install_line}" && -n "${verify_line}" && "${install_line}" -lt "${verify_line}" ]] || {
  echo "generated Thin Host contracts CI does not install frontend dependencies before full verification" >&2
  exit 1
}
if grep -Eq 'go run ./cmd/mss|--foundation|__MSS_' "${host_root}/.github/workflows/ci.yml"; then
  echo "generated Thin Host CI contains a checkout-only command or unresolved placeholder" >&2
  exit 1
fi
grep -Fq 'admin: go run ./cmd/server server' "${host_root}/.mss/project.yaml"
grep -Fq 'command: [go, run, ./cmd/server, server]' "${host_root}/.mss/dev.yaml"
grep -Fq "tarball: ${registry_url}${fixture_tarball_path}" "${host_root}/web/pnpm-lock.yaml" || {
  echo "generated frontend lock does not pin the metadata dist.tarball URL" >&2
  exit 1
}
grep -Fq "integrity: ${computed_admin_web_integrity}" "${host_root}/web/pnpm-lock.yaml" || {
  echo "generated frontend lock does not pin the metadata dist.integrity" >&2
  exit 1
}
if grep -Fq "${registry_url}${pnpm_fallback_tarball_path}" "${host_root}/web/pnpm-lock.yaml"; then
  echo "generated frontend lock fell back to pnpm's inferred tarball URL" >&2
  exit 1
fi
if grep -Eq 'NODE_AUTH_TOKEN|_authToken' "${host_root}/web/.npmrc" "${host_root}/web/pnpm-lock.yaml"; then
  echo "generated frontend files contain a registry credential reference" >&2
  exit 1
fi

tree_digest() {
  python3 - "$1" <<'PY'
from hashlib import sha256
from pathlib import Path
import sys

root = Path(sys.argv[1]).resolve()
digest = sha256()
for path in sorted(root.rglob('*')):
    relative = path.relative_to(root).as_posix()
    if relative == '.git' or relative.startswith('.git/'):
        continue
    if path.is_dir():
        digest.update(b'D\0')
        digest.update(relative.encode())
        digest.update(b'\0')
        continue
    if not path.is_file():
        continue
    digest.update(b'F\0')
    digest.update(relative.encode())
    digest.update(b'\0')
    digest.update(sha256(path.read_bytes()).digest())
print(digest.hexdigest())
PY
}

assert_foundation_snapshot_parity() {
  local phase=$1
  local tool=$2
  local mcp_tool=$3
  local expected_foundation_commit=$4
  local expected_blueprint_version=$5
  local generation_report=${6:-}
  local apply_report=${7:-}
  local prefix="${work_dir}/${phase}"

  "${tool}" --root "${host_root}" context --format json > "${prefix}-context.json"
  "${tool}" --root "${host_root}" doctor --strict --component agent --format json > "${prefix}-doctor.json"
  "${tool}" --root "${host_root}" upgrade status --format json > "${prefix}-status.json"
  printf '%s\n' '{"jsonrpc":"2.0","id":"status","method":"tools/call","params":{"name":"mss_get_blueprint_status","arguments":{}}}' \
    | "${mcp_tool}" -root "${host_root}" > "${prefix}-mcp-status.json"

  python3 - \
    "${prefix}-context.json" \
    "${prefix}-doctor.json" \
    "${prefix}-status.json" \
    "${prefix}-mcp-status.json" \
    "${release_version}" \
    "${expected_foundation_commit}" \
    "${expected_blueprint_version}" \
    "${generation_report}" \
    "${apply_report}" <<'PY'
import json
from pathlib import Path
import sys

(
    context_path,
    doctor_path,
    status_path,
    mcp_path,
    release_version,
    expected_commit,
    expected_blueprint,
    generation_path,
    apply_path,
) = sys.argv[1:]

context = json.loads(Path(context_path).read_text(encoding='utf-8'))
doctor = json.loads(Path(doctor_path).read_text(encoding='utf-8'))
status = json.loads(Path(status_path).read_text(encoding='utf-8'))
rpc = json.loads(Path(mcp_path).read_text(encoding='utf-8'))
if rpc.get('result', {}).get('isError'):
    raise SystemExit(f'MCP status failed: {rpc}')
mcp_status = rpc['result']['structuredContent']
snapshot_checks = [check for check in doctor.get('checks', []) if check.get('id') == 'snapshot:foundation']
if len(snapshot_checks) != 1:
    raise SystemExit(f'doctor snapshot check is missing or duplicated: {snapshot_checks}')
snapshot_check = snapshot_checks[0]
doctor_status = snapshot_check.get('snapshot')
if snapshot_check.get('status') != 'pass' or not snapshot_check.get('required') or not doctor_status:
    raise SystemExit(f'doctor did not require the generated snapshot: {snapshot_check}')
if status != mcp_status or status != doctor_status:
    raise SystemExit('CLI, MCP, and doctor snapshot status differ')

if generation_path:
    generated = json.loads(Path(generation_path).read_text(encoding='utf-8'))
    if generated.get('identities') != status.get('identities'):
        raise SystemExit('generation plan and installed status identities differ')
if apply_path:
    applied = json.loads(Path(apply_path).read_text(encoding='utf-8'))
    if applied.get('toIdentities') != status.get('identities'):
        raise SystemExit('applied target identities and installed status differ')

identities = status.get('identities', {})
records = status.get('records', {})
foundation = identities.get('foundation', {})
blueprint = identities.get('blueprint', {})
generator = identities.get('generator', {})
expected_foundation_version = release_version.removeprefix('v')
if foundation.get('version') != expected_foundation_version or foundation.get('commit') != expected_commit:
    raise SystemExit(f'Foundation identity is not exact: {foundation}')
if blueprint.get('version') != expected_blueprint or status.get('blueprintVersion') != expected_blueprint:
    raise SystemExit(f'Blueprint identity is not exact: {blueprint}')
if generator.get('version') != release_version or generator.get('commit') != expected_commit:
    raise SystemExit(f'generator build identity is not exact: {generator}')
project_baseline = context.get('project', {}).get('spec', {}).get('foundationVersion')
if not project_baseline:
    raise SystemExit(f'project foundation baseline is missing: {context}')
if project_baseline in {foundation.get('version'), blueprint.get('version'), generator.get('version')}:
    raise SystemExit('project.foundationVersion was conflated with an independent runtime identity')
if status.get('foundationCommit') != foundation.get('commit') or status.get('generatorCommit') != generator.get('commit'):
    raise SystemExit(f'flat compatibility metadata contradicts identities: {status}')
for label, value in {
    'Blueprint digest': blueprint.get('sha256', ''),
    'snapshot digest': identities.get('snapshot', {}).get('sha256', ''),
    'lock digest': records.get('lockSha256', ''),
}.items():
    if len(value) != 64 or any(character not in '0123456789abcdef' for character in value):
        raise SystemExit(f'{label} is not an exact SHA-256 digest: {value}')
PY
}

validate_external_host_go() {
  local phase=$1
  local foundation_root=$2
  local validation_root="${work_dir}/${phase}-go-consumer"
  mkdir -p -- "${validation_root}"
  cp -a -- "${host_root}/." "${validation_root}/"
  (
    cd "${validation_root}"
    GOWORK=off GOFLAGS= go mod edit \
      -replace=github.com/mss-boot-io/mss-boot-admin/admin="${foundation_root}/admin" \
      -replace=github.com/mss-boot-io/mss-boot-admin/mss-boot="${foundation_root}/mss-boot"
    GOWORK=off GOFLAGS= GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go mod tidy
    GOWORK=off GOFLAGS=-mod=readonly GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go test -shuffle=on -count=1 ./...
    GOWORK=off GOFLAGS=-mod=readonly GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go vet ./...
    GOWORK=off GOFLAGS=-mod=readonly GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go build -trimpath -o "${work_dir}/${phase}-admin-server" ./cmd/server
    "${work_dir}/${phase}-admin-server" --help >/dev/null
  )
}

run_next_foundation_qualification() {
  local current_blueprint_version
  current_blueprint_version=$(python3 - "${candidate_source}/.mss/blueprints/management-system.yaml" <<'PY'
from pathlib import Path
import re
import sys

text = Path(sys.argv[1]).read_text(encoding='utf-8')
match = re.search(r'(?m)^  version:\s*([0-9]+\.[0-9]+\.[0-9]+)\s*$', text)
if not match:
    raise SystemExit('Blueprint metadata.version must be a stable semantic version')
print(match.group(1))
PY
  )

  assert_foundation_snapshot_parity \
    current-foundation \
    "${mss}" \
    "${mss_mcp}" \
    "${release_commit}" \
    "${current_blueprint_version}" \
    "${write_report}"
  validate_external_host_go current-foundation "${candidate_source}"

  local next_foundation="${work_dir}/next-foundation"
  git clone --quiet --no-hardlinks --no-checkout "${repository_root}" "${next_foundation}"
  git -C "${next_foundation}" checkout --quiet --detach "${release_commit}"

  # The complete eval catalog uses the exact candidate Git checkout so its
  # Blueprint evaluation records the same immutable source commit. Reports are
  # ignored inside this temporary checkout and disappear with the work dir.
  "${mss}" --root "${next_foundation}" eval run --all \
    "${contributor_registry_args[@]}" \
    --format json > "${work_dir}/eval-all.json"
  jq -e '.success == true and (.cases | length > 0) and ([.cases[].success] | all)' "${work_dir}/eval-all.json" >/dev/null

  python3 - "${next_foundation}" "${work_dir}/next-foundation-fixture.json" <<'PY'
import json
from pathlib import Path
import re
import sys

root = Path(sys.argv[1]).resolve()
report = Path(sys.argv[2]).resolve()

blueprint = root / '.mss/blueprints/management-system.yaml'
text = blueprint.read_text(encoding='utf-8')
match = re.search(r'(?m)^(  version: )(([0-9]+)\.([0-9]+)\.([0-9]+))\s*$', text)
if not match:
    raise SystemExit('expected a stable semantic Blueprint version')
next_blueprint = f'{match.group(3)}.{match.group(4)}.{int(match.group(5)) + 1}-local'
blueprint.write_text(text[:match.start(2)] + next_blueprint + text[match.end(2):], encoding='utf-8')

project = root / 'templates/application/.mss/project.yaml'
text = project.read_text(encoding='utf-8')
match = re.search(r'(?m)^(  foundationVersion: )(([0-9]+)\.([0-9]+)\.([0-9]+)(?:-[^\s]+)?)\s*$', text)
if not match:
    raise SystemExit('project foundationVersion is not a semantic baseline')
next_project_baseline = f'{match.group(3)}.{match.group(4)}.{int(match.group(5)) + 1}-project-local'
project.write_text(text[:match.start(2)] + next_project_baseline + text[match.end(2):], encoding='utf-8')

probe = root / 'templates/application/config/foundation-compatibility-probe.md'
probe.write_text('# Foundation compatibility probe\n\nGenerated by the local compatibility fixture.\n', encoding='utf-8')

updated = root / 'templates/application/config/README.md'
updated.write_text(updated.read_text(encoding='utf-8') + '\nFoundation compatibility update fixture.\n', encoding='utf-8')

removed = root / 'templates/application/web/src/business/README.md'
if not removed.exists():
    raise SystemExit('expected managed removal fixture is missing')
removed.unlink()

report.write_text(json.dumps({
    'nextBlueprintVersion': next_blueprint,
    'nextProjectBaselineVersion': next_project_baseline,
}, sort_keys=True) + '\n', encoding='utf-8')
PY

  local next_blueprint_version
  local next_project_baseline_version
  next_blueprint_version=$(jq -er '.nextBlueprintVersion' "${work_dir}/next-foundation-fixture.json")
  next_project_baseline_version=$(jq -er '.nextProjectBaselineVersion' "${work_dir}/next-foundation-fixture.json")
  git -C "${next_foundation}" add -A
  git -C "${next_foundation}" \
    -c user.name=mss-foundation-local \
    -c user.email=mss-foundation-local@users.noreply.github.com \
    commit --quiet -m 'test: create next foundation fixture'

  local next_foundation_commit
  local next_foundation_timestamp
  next_foundation_commit=$(git -C "${next_foundation}" rev-parse 'HEAD^{commit}')
  next_foundation_timestamp=$(git -C "${next_foundation}" show -s --format=%cI "${next_foundation_commit}")
  [[ "${next_foundation_commit}" =~ ^[0-9a-f]{40}$ ]] || {
    echo "next Foundation commit is not a full SHA: ${next_foundation_commit}" >&2
    exit 1
  }

  local mss_next="${tool_dir}/mss-next"
  local mss_mcp_next="${tool_dir}/mss-mcp-next"
  local next_ldflags="-X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Version=${release_version} -X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Commit=${next_foundation_commit} -X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Timestamp=${next_foundation_timestamp}"
  (
    cd "${next_foundation}"
    GOWORK=off go build -mod=mod -trimpath -ldflags "${next_ldflags}" -o "${mss_next}" ./cmd/mss
    GOWORK=off go build -mod=mod -trimpath -ldflags "${next_ldflags}" -o "${mss_mcp_next}" ./cmd/mss-mcp
  )
  local next_tool
  local next_tool_output
  for next_tool in "${mss_next}" "${mss_mcp_next}"; do
    next_tool_output=$("${next_tool}" --version)
    [[ "${next_tool_output}" == *"${release_version}"* ]] || { echo "next tool version is missing: ${next_tool_output}" >&2; exit 1; }
    [[ "${next_tool_output}" == *"${next_foundation_commit}"* ]] || { echo "next tool commit is missing: ${next_tool_output}" >&2; exit 1; }
    [[ "${next_tool_output}" == *"${next_foundation_timestamp}"* ]] || { echo "next tool timestamp is missing: ${next_tool_output}" >&2; exit 1; }
  done

  grep -Fqx '# Downstream custom policy' "${host_root}/AGENTS.md" || printf '\n# Downstream custom policy\n' >> "${host_root}/AGENTS.md"
  mkdir -p -- "${host_root}/internal/modules/customer-extension"
  printf '%s\n' 'package customerextension' '' 'const Preserved = true' \
    > "${host_root}/internal/modules/customer-extension/custom.go"

  "${mss_next}" --root "${host_root}" upgrade plan \
    --foundation "${next_foundation}" \
    "${contributor_registry_args[@]}" \
    --format json > "${work_dir}/next-upgrade-plan.json"
  jq -cn --arg foundationRoot "${next_foundation}" \
    '{jsonrpc:"2.0",id:"upgrade-plan",method:"tools/call",params:{name:"mss_plan_foundation_upgrade",arguments:{foundationRoot:$foundationRoot}}}' \
    | "${mss_mcp_next}" -root "${host_root}" -contributor-npm-registry "${registry_url}" \
      > "${work_dir}/next-mcp-upgrade-plan.json"

  python3 - "${work_dir}/next-upgrade-plan.json" "${work_dir}/next-mcp-upgrade-plan.json" <<'PY'
import json
from pathlib import Path
import sys

plan = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
rpc = json.loads(Path(sys.argv[2]).read_text(encoding='utf-8'))
if not plan.get('success') or not plan.get('dryRun'):
    raise SystemExit(f'upgrade plan is not a successful dry-run: {plan}')
if rpc.get('result', {}).get('isError'):
    raise SystemExit(f'MCP upgrade plan failed: {rpc}')
mcp_plan = rpc['result']['structuredContent']
if plan.get('application', {}).get('module') != 'example.com/standalone-admin':
    raise SystemExit(f'CLI upgrade did not recover the root module from the snapshot: {plan.get("application")}')
if mcp_plan.get('application') != plan.get('application') or mcp_plan.get('toIdentities') != plan.get('toIdentities'):
    raise SystemExit('CLI and MCP upgrade plans disagree on application or target identities')
actions = {change.get('action') for change in plan.get('changes', [])}
missing = {'create', 'update', 'delete', 'preserve'} - actions
if missing:
    raise SystemExit(f'upgrade plan is missing actions: {sorted(missing)}')
PY

  "${mss_next}" --root "${host_root}" upgrade apply \
    --foundation "${next_foundation}" \
    "${contributor_registry_args[@]}" \
    --yes \
    --format json > "${work_dir}/next-upgrade-apply.json"

  grep -Fq '# Downstream custom policy' "${host_root}/AGENTS.md"
  test -f "${host_root}/internal/modules/customer-extension/custom.go"
  grep -Fq "foundationVersion: ${next_project_baseline_version}" "${host_root}/.mss/project.yaml"
  test -f "${host_root}/config/foundation-compatibility-probe.md"
  grep -Fq 'Foundation compatibility update fixture' "${host_root}/config/README.md"
  test ! -e "${host_root}/web/src/business/README.md"

  assert_foundation_snapshot_parity \
    next-foundation \
    "${mss_next}" \
    "${mss_mcp_next}" \
    "${next_foundation_commit}" \
    "${next_blueprint_version}" \
    '' \
    "${work_dir}/next-upgrade-apply.json"
  validate_external_host_go next-foundation "${next_foundation}"

  "${mss_next}" --root "${host_root}" upgrade apply \
    --foundation "${next_foundation}" \
    "${contributor_registry_args[@]}" \
    --yes \
    --format json > "${work_dir}/next-upgrade-noop.json"
  jq -e \
    '.dryRun == false and .success == true and ([.changes[].action] | all(. == "unchanged" or . == "preserve"))' \
    "${work_dir}/next-upgrade-noop.json" >/dev/null

  local conflict_foundation="${work_dir}/conflict-foundation"
  git clone --quiet --no-hardlinks --no-checkout "${next_foundation}" "${conflict_foundation}"
  git -C "${conflict_foundation}" checkout --quiet --detach "${next_foundation_commit}"
  printf '\nconst FoundationConflictProbe = true\n' \
    >> "${conflict_foundation}/templates/application/cmd/server/main.go.tmpl"
  git -C "${conflict_foundation}" add templates/application/cmd/server/main.go.tmpl
  git -C "${conflict_foundation}" \
    -c user.name=mss-foundation-local \
    -c user.email=mss-foundation-local@users.noreply.github.com \
    commit --quiet -m 'test: change foundation Admin entrypoint'

  printf '\nconst DownstreamConflictProbe = true\n' >> "${host_root}/cmd/server/main.go"
  cp -- "${host_root}/cmd/server/main.go" "${work_dir}/downstream-main-before-conflict.go"
  (
    cd "${host_root}"
    sha256sum cmd/server/main.go .mss/lock.yaml .mss/blueprint-manifest.json \
      > "${work_dir}/conflict-files-before.sha256"
  )
  local conflict_tree_before
  conflict_tree_before=$(tree_digest "${host_root}")

  local conflict_plan_status
  set +e
  "${mss_next}" --root "${host_root}" upgrade plan \
    --foundation "${conflict_foundation}" \
    "${contributor_registry_args[@]}" \
    --format json > "${work_dir}/upgrade-conflict.json"
  conflict_plan_status=$?
  set -e
  [[ ${conflict_plan_status} -eq 0 ]] || {
    echo 'a conflict plan must be returned as structured output without an execution failure' >&2
    exit 1
  }
  jq -e \
    '.success == false and any(.changes[]; .action == "conflict" and .path == "cmd/server/main.go")' \
    "${work_dir}/upgrade-conflict.json" >/dev/null

  if "${mss_next}" --root "${host_root}" upgrade apply \
      --foundation "${conflict_foundation}" \
      "${contributor_registry_args[@]}" \
      --yes \
      --format json > "${work_dir}/unexpected-conflict-apply.json"; then
    echo 'conflict apply unexpectedly succeeded' >&2
    exit 1
  fi

  cmp --silent "${host_root}/cmd/server/main.go" "${work_dir}/downstream-main-before-conflict.go"
  (
    cd "${host_root}"
    sha256sum cmd/server/main.go .mss/lock.yaml .mss/blueprint-manifest.json \
      > "${work_dir}/conflict-files-after.sha256"
  )
  cmp --silent "${work_dir}/conflict-files-before.sha256" "${work_dir}/conflict-files-after.sha256"
  [[ "${conflict_tree_before}" = "$(tree_digest "${host_root}")" ]] || {
    echo 'conflict apply changed the downstream tree' >&2
    exit 1
  }
}

first_digest=$(tree_digest "${host_root}")
(
  cd "${consumer_parent}"
  "${mss}" new app standalone-admin \
    --display-name "Standalone Administration" \
    --module example.com/standalone-admin \
    --repository example/standalone-admin \
    --destination "${host_root}" \
    "${contributor_registry_args[@]}" \
    --write --git-init --format json > "${work_dir}/new-app-repeat.json"
)
second_digest=$(tree_digest "${host_root}")
[[ "${first_digest}" = "${second_digest}" ]] || { echo "second new app write was not idempotent" >&2; exit 1; }
jq -e '[.changes[].action] | all(. == "unchanged")' "${work_dir}/new-app-repeat.json" >/dev/null

if ! "${mss}" --root "${host_root}" doctor --strict --format json > "${work_dir}/doctor.json"; then
  sed -n '1,240p' "${work_dir}/doctor.json" >&2
  exit 1
fi
jq -e '.ready == true' "${work_dir}/doctor.json" >/dev/null
[[ ! -e "${host_root}/.mss/reports" ]] || {
  echo "Thin Host unexpectedly contains the setup report directory before dry-run" >&2
  exit 1
}
before_setup_dry_run_digest=$(tree_digest "${host_root}")
"${mss}" --root "${host_root}" setup --dry-run --format json > "${work_dir}/setup-one.json"
"${mss}" --root "${host_root}" setup --dry-run --format json > "${work_dir}/setup-two.json"
after_setup_dry_run_digest=$(tree_digest "${host_root}")
[[ "${before_setup_dry_run_digest}" = "${after_setup_dry_run_digest}" ]] || {
  echo "Thin Host setup dry-run changed the repository" >&2
  exit 1
}
[[ ! -e "${host_root}/.mss/reports" ]] || {
  echo "Thin Host setup dry-run created the report directory" >&2
  exit 1
}
jq -S 'del(.generatedAt)' "${work_dir}/setup-one.json" > "${work_dir}/setup-one.normalized.json"
jq -S 'del(.generatedAt)' "${work_dir}/setup-two.json" > "${work_dir}/setup-two.normalized.json"
cmp --silent "${work_dir}/setup-one.normalized.json" "${work_dir}/setup-two.normalized.json" || {
  echo "Thin Host setup plan is not idempotent" >&2
  exit 1
}
"${mss}" --root "${host_root}" dev status --format json > "${work_dir}/dev-status.json"
jq -e '.success == true and ([.services[].status] | all(. == "stopped"))' "${work_dir}/dev-status.json" >/dev/null

set +e
mismatch_version=v9.9.9
"${mss}" --root "${host_root}" upgrade admin "${mismatch_version}" "${contributor_registry_args[@]}" --format json > "${work_dir}/mismatch.stdout" 2> "${work_dir}/mismatch.stderr"
mismatch_status=$?
set -e
[[ ${mismatch_status} -ne 0 ]] || { echo "mismatched embedded upgrade unexpectedly succeeded" >&2; exit 1; }
grep -Fq "install the matching mss ${mismatch_version}" "${work_dir}/mismatch.stderr"

if [[ "${run_upgrade}" = true ]]; then
  business_file="${host_root}/internal/modules/custom/owned.go"
  mkdir -p -- "$(dirname "${business_file}")"
  printf '%s\n' 'package custom' > "${business_file}"
  "${mss}" --root "${host_root}" upgrade admin "${release_version}" "${contributor_registry_args[@]}" --format json > "${work_dir}/upgrade-plan.json"
  jq -e '.dryRun == true and .success == true and (.preservedFiles | index("internal/modules/custom/owned.go")) != null' "${work_dir}/upgrade-plan.json" >/dev/null
  "${mss}" --root "${host_root}" upgrade admin "${release_version}" "${contributor_registry_args[@]}" --apply --yes --format json > "${work_dir}/upgrade-apply.json"
  [[ "$(<"${business_file}")" = 'package custom' ]] || { echo "embedded upgrade changed a business-owned file" >&2; exit 1; }
  "${mss}" --root "${host_root}" upgrade admin "${release_version}" "${contributor_registry_args[@]}" --apply --yes --format json > "${work_dir}/upgrade-noop.json"
  jq -e '.dryRun == false and .success == true and ([.changes[].action] | all(. == "unchanged" or . == "preserve"))' "${work_dir}/upgrade-noop.json" >/dev/null
fi

if [[ "${run_next_foundation}" = true ]]; then
  run_next_foundation_qualification
fi

if [[ "${run_lifecycle}" = true ]]; then
  initial_admin_password=$(python3 - <<'PY'
import secrets
print('Aa1!' + secrets.token_urlsafe(32))
PY
)
  if ! GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
    NPM_CONFIG_USERCONFIG="${npm_user_config}" MSS_ADMIN_INITIAL_PASSWORD="${initial_admin_password}" \
    "${mss}" --root "${host_root}" setup --format json > "${work_dir}/setup-first.json"; then
    sed -n '1,320p' "${work_dir}/setup-first.json" >&2
    exit 1
  fi
  jq -e '.success == true and ([.results[].exitCode] | all(. == 0))' "${work_dir}/setup-first.json" >/dev/null
  if ! env -u MSS_ADMIN_INITIAL_PASSWORD \
    GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
    NPM_CONFIG_USERCONFIG="${npm_user_config}" \
    "${mss}" --root "${host_root}" setup --format json > "${work_dir}/setup-second.json"; then
    sed -n '1,320p' "${work_dir}/setup-second.json" >&2
    exit 1
  fi
  jq -e '.success == true and ([.results[].exitCode] | all(. == 0))' "${work_dir}/setup-second.json" >/dev/null
  (
    cd "${host_root}"
    sha256sum go.mod go.sum > "${work_dir}/go-lock-before-tidy.sha256"
    GOWORK=off GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go mod tidy
    sha256sum go.mod go.sum > "${work_dir}/go-lock-after-first-tidy.sha256"
    cmp --silent "${work_dir}/go-lock-before-tidy.sha256" "${work_dir}/go-lock-after-first-tidy.sha256" || {
      echo "generated Thin Host Go locks are not canonical after the first tidy" >&2
      exit 1
    }
    GOWORK=off GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go mod tidy
    sha256sum go.mod go.sum > "${work_dir}/go-lock-after-second-tidy.sha256"
    cmp --silent "${work_dir}/go-lock-after-first-tidy.sha256" "${work_dir}/go-lock-after-second-tidy.sha256" || {
      echo "generated Thin Host Go locks changed on the second tidy" >&2
      exit 1
    }
    GOWORK=off GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go test -mod=readonly ./...
    GOWORK=off GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
      go build -mod=readonly -trimpath -o "${work_dir}/standalone-admin" ./cmd/server
    NPM_CONFIG_USERCONFIG="${npm_user_config}" corepack pnpm@10.34.5 --dir web install --frozen-lockfile
    NPM_CONFIG_USERCONFIG="${npm_user_config}" corepack pnpm@10.34.5 --dir web lint
    NPM_CONFIG_USERCONFIG="${npm_user_config}" corepack pnpm@10.34.5 --dir web test
    NPM_CONFIG_USERCONFIG="${npm_user_config}" corepack pnpm@10.34.5 --dir web build
  )
  if ! GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
    NPM_CONFIG_USERCONFIG="${npm_user_config}" \
    "${mss}" --root "${host_root}" dev --detach --format json > "${work_dir}/dev-start.json"; then
    sed -n '1,320p' "${work_dir}/dev-start.json" >&2
    for service in backend admin-web; do
      log_path="${host_root}/.mss/logs/${service}.log"
      if [[ -f "${log_path}" ]]; then
        printf '%s\n' "--- ${service} log ---" >&2
        tail -n 160 "${log_path}" >&2
      fi
    done
    exit 1
  fi
  jq -e \
    '.success == true and (.services | length == 2) and
     ([.services[].serviceId] | sort) == ["admin-web", "backend"] and
     ([.services[] | (.status == "running" and .healthy == true)] | all)' \
    "${work_dir}/dev-start.json" >/dev/null
  curl --fail --silent --show-error http://127.0.0.1:8080/healthz > "${work_dir}/backend-health.txt"
  curl --fail --silent --show-error http://127.0.0.1:8001/ > "${work_dir}/frontend-health.html"
  "${mss}" --root "${host_root}" dev status --format json > "${work_dir}/dev-status.json"
  jq -e \
    '.success == true and (.services | length == 2) and
     ([.services[].serviceId] | sort) == ["admin-web", "backend"] and
     ([.services[] | (.status == "running" and .healthy == true)] | all)' \
    "${work_dir}/dev-status.json" >/dev/null
  "${mss}" --root "${host_root}" dev stop --format json > "${work_dir}/dev-stop.json"
  if ! GOMODCACHE="${consumer_go_modcache}" GOPROXY="${consumer_go_proxy}" GOSUMDB="${consumer_go_sumdb}" \
    NPM_CONFIG_USERCONFIG="${npm_user_config}" \
    "${mss}" --root "${host_root}" verify --all --format json > "${work_dir}/verify.json"; then
    sed -n '1,320p' "${work_dir}/verify.json" >&2
    if [[ -f "${host_root}/.mss/reports/verify.md" ]]; then
      sed -n '1,320p' "${host_root}/.mss/reports/verify.md" >&2
    fi
    exit 1
  fi
  jq -e '.success == true' "${work_dir}/verify.json" >/dev/null
fi

printf 'standalone mss consumer passed: %s commit %s (lifecycle=%s upgrade=%s next-foundation=%s artifact=%s public-packages=%s)\n' \
  "${release_version}" "${release_commit}" "${run_lifecycle}" "${run_upgrade}" "${run_next_foundation}" "${use_release_artifact}" "${use_public_packages}"
