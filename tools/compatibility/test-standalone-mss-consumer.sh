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
  --release-artifact  Use MSS_TOOLS_ARCHIVE instead of locally built tools.
  --public-packages   Resolve the published v1.3.5 Go and npm packages through
                      proxy.golang.org and the exact GitHub Packages Admin Web
                      candidate; requires NODE_AUTH_TOKEN. Official npmjs is a
                      later post-publication gate.
  --help              Show this help.

Environment:
  MSS_RELEASE_VERSION     Release version (default: v1.3.5).
  MSS_RELEASE_COMMIT      Full release commit (default: current HEAD).
  MSS_RELEASE_TIMESTAMP   RFC3339 source timestamp (default: current HEAD).
  MSS_TOOLS_ARCHIVE       Versioned mss tools tar.gz or zip for
                          --release-artifact.
  MSS_ADMIN_WEB_TARBALL   Optional exact v1.3.5 Admin Web candidate tarball.
                         When omitted, the script packs the release commit;
                         either candidate is served only from loopback.
  NODE_AUTH_TOKEN         GitHub Packages read token required only with
                         --public-packages; it is never written to a file.
EOF
}

run_lifecycle=false
run_upgrade=false
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

repository_root=$(git rev-parse --show-toplevel)
repository_root=$(realpath -- "${repository_root}")
[[ -f "${repository_root}/embedded_distribution.go" ]] || {
  echo "repository does not contain the embedded Distribution source" >&2
  exit 1
}

release_version=${MSS_RELEASE_VERSION:-v1.3.5}
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
# qualify the same module paths users can install after v1.3.5 is public. The
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
jq -e '.dryRun == true and .success == true and .identities.foundation.version == "1.3.5"' "${dry_run_report}" >/dev/null
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
"${mss}" --root "${host_root}" upgrade admin v1.3.5 "${contributor_registry_args[@]}" --format json > "${work_dir}/mismatch.stdout" 2> "${work_dir}/mismatch.stderr"
mismatch_status=$?
set -e
[[ ${mismatch_status} -ne 0 ]] || { echo "mismatched embedded upgrade unexpectedly succeeded" >&2; exit 1; }
grep -Eq 'install the matching mss v1.3.5' "${work_dir}/mismatch.stderr"

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

printf 'standalone mss consumer passed: %s commit %s (lifecycle=%s upgrade=%s artifact=%s public-packages=%s)\n' \
  "${release_version}" "${release_commit}" "${run_lifecycle}" "${run_upgrade}" "${use_release_artifact}" "${use_public_packages}"
