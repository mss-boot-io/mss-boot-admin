#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo 'Usage: resolve_successful_preview.sh --repository OWNER/REPO --commit SHA --version vX.Y.Z --actor LOGIN' >&2
}

repository=''
commit=''
version=''
actor=''
while (($#)); do
  case "$1" in
    --repository)
      repository=${2:-}
      shift 2
      ;;
    --commit)
      commit=${2:-}
      shift 2
      ;;
    --version)
      version=${2:-}
      shift 2
      ;;
    --actor)
      actor=${2:-}
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

[[ "${repository}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || {
  echo '--repository must be OWNER/REPO' >&2
  exit 2
}
[[ "${commit}" =~ ^[0-9a-f]{40}$ ]] || {
  echo '--commit must be one full lowercase commit SHA' >&2
  exit 2
}
[[ "${version}" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]] || {
  echo '--version must be a v-prefixed semantic version' >&2
  exit 2
}
[[ "${actor}" =~ ^[A-Za-z0-9-]+$ ]] || {
  echo '--actor must be one GitHub login' >&2
  exit 2
}
command -v gh >/dev/null
command -v jq >/dev/null

runs="$(gh api \
  --method GET \
  -H 'Accept: application/vnd.github+json' \
  "/repos/${repository}/actions/workflows/release.yml/runs" \
  -f head_sha="${commit}" \
  -f event=workflow_dispatch \
  -f status=completed \
  -f per_page=100)"
run_id="$(jq -er \
  --arg commit "${commit}" \
  --arg display_title "Root Release candidate ${version}" \
  --arg actor "${actor}" '
  [.workflow_runs[] | select(
    .event == "workflow_dispatch" and
    .head_branch == "main" and
    .head_sha == $commit and
    .path == ".github/workflows/release.yml" and
    .display_title == $display_title and
    .actor.login == $actor and
    .triggering_actor.login == $actor and
    .status == "completed" and
    .conclusion == "success"
  )] | sort_by(.id) | last | .id // empty
  ' <<< "${runs}")" || {
  echo "no successful exact Root Release candidate ${version} preview exists at ${commit}" >&2
  exit 1
}
[[ "${run_id}" =~ ^[1-9][0-9]*$ ]] || {
  echo 'successful preview did not expose one valid workflow run id' >&2
  exit 1
}
artifacts="$(gh api \
  --method GET \
  -H 'Accept: application/vnd.github+json' \
  "/repos/${repository}/actions/runs/${run_id}/artifacts" \
  -f per_page=100)"
for artifact_name in \
  "release-packages-${version}" \
  "frontend-v6-dist" \
  "root-image-preview-${version}"; do
  if ! jq -e \
    --arg name "${artifact_name}" \
    '[.artifacts[] | select(
      .name == $name and
      .expired == false and
      (.size_in_bytes | type == "number" and . > 0)
    )] | length == 1' <<< "${artifacts}" >/dev/null; then
    echo "successful preview ${run_id} has no exact unexpired ${artifact_name} artifact" >&2
    exit 1
  fi
done
printf '%s\n' "${run_id}"
