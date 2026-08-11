#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
usage: verify_readiness_run.sh --repository OWNER/REPO --run-id ID \
  --target-version VERSION --commit FULL_SHA --phase PHASE --policy PATH
EOF
  exit 2
}

repository=""
run_id=""
target_version=""
commit=""
phase=""
policy=""
server_url="${GITHUB_SERVER_URL:-https://github.com}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repository)
      [[ $# -ge 2 ]] || usage
      repository="$2"
      shift 2
      ;;
    --run-id)
      [[ $# -ge 2 ]] || usage
      run_id="$2"
      shift 2
      ;;
    --target-version)
      [[ $# -ge 2 ]] || usage
      target_version="$2"
      shift 2
      ;;
    --commit)
      [[ $# -ge 2 ]] || usage
      commit="$2"
      shift 2
      ;;
    --phase)
      [[ $# -ge 2 ]] || usage
      phase="$2"
      shift 2
      ;;
    --policy)
      [[ $# -ge 2 ]] || usage
      policy="$2"
      shift 2
      ;;
    --server-url)
      [[ $# -ge 2 ]] || usage
      server_url="$2"
      shift 2
      ;;
    *)
      usage
      ;;
  esac
done

[[ "${repository}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || usage
[[ "${run_id}" =~ ^[1-9][0-9]*$ ]] || usage
[[ "${commit}" =~ ^[0-9a-f]{40}$ ]] || usage
[[ -n "${target_version}" && -n "${phase}" && -f "${policy}" ]] || usage

for command in gh jq python3; do
  command -v "${command}" >/dev/null || {
    echo "required command is unavailable: ${command}" >&2
    exit 1
  }
done

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
temp_dir="$(mktemp -d)"
trap 'rm -rf -- "${temp_dir}"' EXIT

server_url="${server_url%/}"
workflow_run_url="${server_url}/${repository}/actions/runs/${run_id}"
run_json="${temp_dir}/workflow-run.json"
gh api \
  --method GET \
  -H 'Accept: application/vnd.github+json' \
  "/repos/${repository}/actions/runs/${run_id}" > "${run_json}"

if ! jq -e \
  --argjson run_id "${run_id}" \
  --arg repository "${repository}" \
  --arg commit "${commit}" \
  --arg workflow_path ".github/workflows/release-readiness.yml" \
  --arg workflow_run_url "${workflow_run_url}" \
  '.id == $run_id
    and .repository.full_name == $repository
    and .head_sha == $commit
    and .status == "completed"
    and .conclusion == "success"
    and .event == "workflow_dispatch"
    and .path == $workflow_path
    and .html_url == $workflow_run_url' \
  "${run_json}" >/dev/null; then
  echo "workflow run ${run_id} is not the successful exact-commit readiness run" >&2
  exit 1
fi

artifact_dir="${temp_dir}/artifact"
mkdir -p "${artifact_dir}"
gh run download "${run_id}" \
  --repo "${repository}" \
  --name "release-readiness-attestation-${run_id}" \
  --dir "${artifact_dir}"

attestation="${artifact_dir}/release-readiness-metadata.json"
if [[ ! -f "${attestation}" ]]; then
  echo "selected readiness run ${run_id} did not contain the exact attestation" >&2
  exit 1
fi

python3 "${script_dir}/release_readiness_attestation.py" verify \
  --attestation "${attestation}" \
  --policy "${policy}" \
  --target-version "${target_version}" \
  --commit "${commit}" \
  --phase "${phase}" \
  --workflow-run-id "${run_id}" \
  --workflow-run-url "${workflow_run_url}" \
  --intent publish
