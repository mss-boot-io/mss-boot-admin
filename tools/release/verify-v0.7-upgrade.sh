#!/usr/bin/env bash
set -euo pipefail

readonly database_name="mss_release_upgrade_test"
readonly sqlite_database_file="${database_name}.sqlite"
readonly baseline_ref="${MSS_RELEASE_BASELINE_REF:-v0.7.0}"
readonly mysql_dsn="${MSS_RELEASE_MYSQL_DSN:-}"
readonly postgres_dsn="${MSS_RELEASE_POSTGRES_DSN:-}"
readonly admin_password="${MSS_RELEASE_ADMIN_PASSWORD:-ReleaseUpgrade-CI-1-only}"
readonly disposable_database_confirmed="${MSS_RELEASE_ALLOW_DISPOSABLE_DATABASE:-}"
readonly evidence_mode="${MSS_RELEASE_EVIDENCE_MODE:-1}"
readonly mysql_version="${MSS_RELEASE_MYSQL_VERSION:-}"
readonly mysql_image_ref="${MSS_RELEASE_MYSQL_IMAGE_REF:-}"
readonly mysql_image_id="${MSS_RELEASE_MYSQL_IMAGE_ID:-}"
readonly postgres_version="${MSS_RELEASE_POSTGRES_VERSION:-}"
readonly postgres_image_ref="${MSS_RELEASE_POSTGRES_IMAGE_REF:-}"
readonly postgres_image_id="${MSS_RELEASE_POSTGRES_IMAGE_ID:-}"

for command in git go python3 realpath tar; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'required command is unavailable: %s\n' "${command}" >&2
    exit 1
  fi
done

repo_root="$(git rev-parse --show-toplevel)"
readonly repo_root
candidate_gowork="${MSS_RELEASE_CANDIDATE_GOWORK:-${repo_root}/go.work}"
if [[ "${candidate_gowork}" == "off" ]]; then
  candidate_dependency_mode="public-module"
else
  candidate_gowork="$(realpath -m -- "${candidate_gowork}")"
  if [[ "${candidate_gowork}" != "${repo_root}/go.work" || ! -f "${candidate_gowork}" ]]; then
    printf 'MSS_RELEASE_CANDIDATE_GOWORK must be off or the repository go.work\n' >&2
    exit 1
  fi
  candidate_dependency_mode="repository-workspace"
fi
readonly candidate_gowork
readonly candidate_dependency_mode
readonly report_dir="${repo_root}/.mss/reports"
readonly report_path="${report_dir}/release-upgrade.json"
mkdir -p "${report_dir}"
# A failed or refused run must never leave evidence from an earlier candidate.
rm -f -- "${report_path}"

if [[ "${disposable_database_confirmed}" != "1" ]]; then
  printf 'set MSS_RELEASE_ALLOW_DISPOSABLE_DATABASE=1 to confirm the disposable targets\n' >&2
  exit 1
fi
if [[ "${evidence_mode}" != "0" && "${evidence_mode}" != "1" ]]; then
  printf 'MSS_RELEASE_EVIDENCE_MODE must be 0 or 1\n' >&2
  exit 1
fi
python3 "${repo_root}/tools/release/validate_disposable_dsn.py" \
  --driver mysql --dsn "${mysql_dsn}" --database "${database_name}"
python3 "${repo_root}/tools/release/validate_disposable_dsn.py" \
  --driver postgres --dsn "${postgres_dsn}" --database "${database_name}"
if ! git rev-parse --verify --quiet "${baseline_ref}^{commit}" >/dev/null; then
  printf 'baseline ref is not available locally: %s\n' "${baseline_ref}" >&2
  exit 1
fi

baseline_commit="$(git -C "${repo_root}" rev-parse "${baseline_ref}^{commit}")"
readonly baseline_commit
candidate_commit="$(git -C "${repo_root}" rev-parse HEAD)"
readonly candidate_commit
initial_worktree_status="$(git -C "${repo_root}" status --porcelain --untracked-files=all)"
readonly initial_worktree_status
initial_worktree_dirty=false
if [[ -n "${initial_worktree_status}" ]]; then
  initial_worktree_dirty=true
fi
readonly initial_worktree_dirty
if [[ "${evidence_mode}" == "1" && "${initial_worktree_dirty}" == "true" ]]; then
  printf 'release evidence mode refuses a dirty working tree\n' >&2
  exit 1
fi

require_evidence_value() {
  local name="$1"
  local value="$2"
  if [[ "${evidence_mode}" == "1" && -z "${value}" ]]; then
    printf 'release evidence mode requires %s\n' "${name}" >&2
    exit 1
  fi
}

require_evidence_value MSS_RELEASE_MYSQL_VERSION "${mysql_version}"
require_evidence_value MSS_RELEASE_MYSQL_IMAGE_REF "${mysql_image_ref}"
require_evidence_value MSS_RELEASE_MYSQL_IMAGE_ID "${mysql_image_id}"
require_evidence_value MSS_RELEASE_POSTGRES_VERSION "${postgres_version}"
require_evidence_value MSS_RELEASE_POSTGRES_IMAGE_REF "${postgres_image_ref}"
require_evidence_value MSS_RELEASE_POSTGRES_IMAGE_ID "${postgres_image_id}"
if [[ "${evidence_mode}" == "1" ]]; then
  if [[ "${mysql_image_id}" != sha256:* || "${postgres_image_id}" != sha256:* ]]; then
    printf 'release evidence mode requires immutable sha256 container image identities\n' >&2
    exit 1
  fi
fi

work_dir="$(mktemp -d "${report_dir}/release-upgrade.XXXXXX")"
readonly work_dir

cleanup() {
  case "${work_dir}" in
    "${report_dir}"/release-upgrade.*) rm -rf -- "${work_dir}" ;;
    *) printf 'refusing to remove unexpected work directory: %s\n' "${work_dir}" >&2 ;;
  esac
}
trap cleanup EXIT

mkdir -p "${work_dir}/baseline" "${work_dir}/bin"
git archive "${baseline_commit}" | tar -x -C "${work_dir}/baseline"
python3 "${repo_root}/tools/release/prepare-v0-7-fixture.py" \
  --baseline "${work_dir}/baseline" \
  --candidate-admin "${repo_root}/admin"

(
  cd "${work_dir}/baseline"
  GOWORK=off go build -trimpath -o "${work_dir}/bin/admin-baseline" .
)
(
  cd "${repo_root}/admin"
  GOWORK="${candidate_gowork}" go build -trimpath -o "${work_dir}/bin/admin-current" .
)

prepare_runtime() {
  local source_tree="$1"
  local runtime_dir="$2"
  local driver="$3"
  local dsn="$4"
  python3 "${repo_root}/tools/release/prepare-db-config.py" \
    --input "${source_tree}/config/application.yml" \
    --output "${runtime_dir}/config/application.yml" \
    --driver "${driver}" \
    --dsn "${dsn}" \
    --database "${database_name}"
}

run_upgrade() {
  local driver="$1"
  local dsn="$2"
  local baseline_dsn="${dsn}"
  local baseline_runtime="${work_dir}/runtime-${driver}-baseline"
  local current_runtime="${work_dir}/runtime-${driver}-current"

  if [[ "${driver}" == "mysql" ]]; then
    # v0.7.0 migration 1746193492486 used the ANSI SQL form "group".
    # Reproduce that historically deployable baseline without weakening the
    # current candidate, which is migrated below with the normal SQL mode.
    baseline_dsn="${dsn}&sql_mode=%27ANSI_QUOTES%27"
  fi

  prepare_runtime "${work_dir}/baseline" "${baseline_runtime}" "${driver}" "${baseline_dsn}"
  prepare_runtime "${repo_root}/admin" "${current_runtime}" "${driver}" "${dsn}"

  (
    cd "${baseline_runtime}"
    "${work_dir}/bin/admin-baseline" migrate \
      --username release-admin \
      --password "${admin_password}" \
      --domain release-upgrade.invalid
  )
  (
    cd "${current_runtime}"
    MSS_ADMIN_INITIAL_PASSWORD="${admin_password}" \
      "${work_dir}/bin/admin-current" migrate \
      --username release-admin \
      --domain release-upgrade.invalid
    unset MSS_ADMIN_INITIAL_PASSWORD
    "${work_dir}/bin/admin-current" migrate \
      --username release-admin \
      --domain release-upgrade.invalid
  )
}

readonly sqlite_dsn="${work_dir}/${sqlite_database_file}"
run_release_upgrade_assertion() {
  local test_name="$1"
  (
    cd "${repo_root}/admin"
    MSS_RELEASE_UPGRADE_STATE_ASSERT=1 \
      MSS_RELEASE_UPGRADE_SQLITE_DSN="${sqlite_dsn}" \
      MSS_RELEASE_UPGRADE_MYSQL_DSN="${mysql_dsn}" \
      MSS_RELEASE_UPGRADE_POSTGRES_DSN="${postgres_dsn}" \
      GOWORK="${candidate_gowork}" go test ./cmd/migrate/migration/system \
        -run "^${test_name}$" -count=1
  )
}

# Re-open every target and verify its authoritative database identity before
# the first migration command can mutate state.
run_release_upgrade_assertion TestV07UpgradeDatabaseIdentityIntegration
run_upgrade sqlite "${sqlite_dsn}"
run_upgrade mysql "${mysql_dsn}"
run_upgrade postgres "${postgres_dsn}"

run_release_upgrade_assertion TestV07UpgradeStateIntegration

sqlite_module_version="$(
  cd "${repo_root}/admin"
  GOWORK="${candidate_gowork}" go list -m -f '{{.Version}}' modernc.org/sqlite
)"
readonly sqlite_module_version
final_candidate_commit="$(git -C "${repo_root}" rev-parse HEAD)"
readonly final_candidate_commit
if [[ "${final_candidate_commit}" != "${candidate_commit}" ]]; then
  printf 'candidate commit changed during the release upgrade rehearsal\n' >&2
  exit 1
fi
final_worktree_status="$(git -C "${repo_root}" status --porcelain --untracked-files=all)"
readonly final_worktree_status
final_worktree_dirty=false
if [[ -n "${final_worktree_status}" ]]; then
  final_worktree_dirty=true
fi
readonly final_worktree_dirty
if [[ "${evidence_mode}" == "1" && "${final_worktree_dirty}" == "true" ]]; then
  printf 'release evidence mode detected a dirty working tree after the rehearsal\n' >&2
  exit 1
fi
python3 -c '
import json
import pathlib
import sys

target = pathlib.Path(sys.argv[1])
temporary = target.with_suffix(target.suffix + ".tmp")
temporary.write_text(
    json.dumps(
        {
            "schemaVersion": 1,
            "evidenceMode": sys.argv[2] == "1",
            "baselineRef": sys.argv[3],
            "baselineCommit": sys.argv[4],
            "candidateCommit": sys.argv[5],
            "candidateDependencyMode": sys.argv[16],
            "dirty": sys.argv[6] == "true",
            "databaseName": sys.argv[7],
            "databases": {
                "sqlite": {
                    "status": "pass",
                    "databaseFile": sys.argv[8],
                    "engineModule": "modernc.org/sqlite",
                    "engineModuleVersion": sys.argv[9],
                },
                "mysql": {
                    "status": "pass",
                    "serverVersion": sys.argv[10],
                    "containerImage": {
                        "reference": sys.argv[11],
                        "id": sys.argv[12],
                    },
                },
                "postgres": {
                    "status": "pass",
                    "serverVersion": sys.argv[13],
                    "containerImage": {
                        "reference": sys.argv[14],
                        "id": sys.argv[15],
                    },
                },
            },
            "assertions": {
                "candidateMigrationRepeat": "pass",
                "connectedDatabaseIdentity": "pass",
                "retiredDeveloperMenusAndPolicies": "pass",
                "activeRootRole": "pass",
                "singleActiveNonRootDefaultRole": "pass",
                "configRevisionInfrastructure": "pass",
            },
        },
        indent=2,
        sort_keys=True,
    )
    + "\n",
    encoding="utf-8",
)
temporary.replace(target)
' \
  "${report_path}" \
  "${evidence_mode}" \
  "${baseline_ref}" \
  "${baseline_commit}" \
  "${candidate_commit}" \
  "${final_worktree_dirty}" \
  "${database_name}" \
  "${sqlite_database_file}" \
  "${sqlite_module_version}" \
  "${mysql_version}" \
  "${mysql_image_ref}" \
  "${mysql_image_id}" \
  "${postgres_version}" \
  "${postgres_image_ref}" \
  "${postgres_image_id}" \
  "${candidate_dependency_mode}"
