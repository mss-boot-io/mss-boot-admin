#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
readonly repo_root
readonly admin_dir="${repo_root}/admin"
readonly framework_dir="${repo_root}/mss-boot"
readonly framework_module="github.com/mss-boot-io/mss-boot-admin/mss-boot"

framework_version="$(
  awk -v module="${framework_module}" '
    $1 == module { print $2; found = 1; exit }
    END { if (!found) exit 1 }
  ' "${admin_dir}/go.mod"
)"
readonly framework_version
if [[ ! "${framework_version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
  printf 'invalid Admin Framework requirement: %s\n' "${framework_version}" >&2
  exit 1
fi

temporary_parent="$(realpath -m -- "${RUNNER_TEMP:-${TMPDIR:-/tmp}}")"
readonly temporary_parent
mkdir -p -- "${temporary_parent}"
temporary_dir="$(mktemp -d "${temporary_parent}/mss-admin-module-metadata.XXXXXX")"
readonly temporary_dir

cleanup() {
  local resolved
  resolved="$(realpath -m -- "${temporary_dir}")"
  case "${resolved}" in
    "${temporary_parent}"/mss-admin-module-metadata.*) rm -rf -- "${resolved}" ;;
    *) printf 'refusing to remove unexpected temporary directory: %s\n' "${resolved}" >&2 ;;
  esac
}
trap cleanup EXIT

readonly temporary_mod="${temporary_dir}/admin.mod"
readonly temporary_sum="${temporary_dir}/admin.sum"
readonly tracked_sum_without_framework="${temporary_dir}/admin-without-framework.sum"
cp -- "${admin_dir}/go.mod" "${temporary_mod}"
cp -- "${admin_dir}/go.sum" "${temporary_sum}"
awk -v module="${framework_module}" -v version="${framework_version}" '
  !($1 == module && ($2 == version || $2 == version "/go.mod")) { print }
' "${admin_dir}/go.sum" > "${tracked_sum_without_framework}"

# The coordinated Framework version is intentionally not public before the
# Framework release phase. Keep the tracked Admin module clean while checking
# its metadata against the exact repository Framework candidate.
(
  cd "${admin_dir}"
  GOWORK=off GOFLAGS= go mod edit -modfile="${temporary_mod}" \
    -replace="${framework_module}@${framework_version}=${framework_dir}"
  GOWORK=off GOFLAGS= go mod tidy -modfile="${temporary_mod}"
  GOWORK=off GOFLAGS= go mod edit -modfile="${temporary_mod}" \
    -dropreplace="${framework_module}@${framework_version}"
)

diff -u -- "${admin_dir}/go.mod" "${temporary_mod}"
# A local replacement deliberately removes public checksum entries for the
# exact Framework version. Compare every other checksum without treating those
# required external-consumer entries as repository-local dependency drift.
diff -u -- "${tracked_sum_without_framework}" "${temporary_sum}"
