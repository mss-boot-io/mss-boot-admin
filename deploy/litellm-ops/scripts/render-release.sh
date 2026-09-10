#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: render-release.sh --admin-image IMAGE --web-image IMAGE --revision REVISION --output DIRECTORY

Renders the live litellm-ops manifests with explicit image references. Tags
must be immutable release tags; sha256 digest references are preferred.
EOF
}

admin_image=""
web_image=""
revision=""
output_dir=""

while (($#)); do
  case "$1" in
    --admin-image) admin_image="${2:-}"; shift 2 ;;
    --web-image) web_image="${2:-}"; shift 2 ;;
    --revision) revision="${2:-}"; shift 2 ;;
    --output) output_dir="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ -n "$admin_image" && -n "$web_image" && -n "$revision" && -n "$output_dir" ]] || {
  usage >&2
  exit 2
}

validate_image() {
  local image_ref="$1"
  [[ "$image_ref" =~ ^[A-Za-z0-9._:/@-]+$ ]] || return 1
  [[ "$image_ref" != *:latest ]] || return 1
  if [[ "$image_ref" == *@sha256:* ]]; then
    [[ "$image_ref" =~ @sha256:[0-9a-f]{64}$ ]]
  else
    [[ "${image_ref##*/}" == *:* ]]
  fi
}

validate_image "$admin_image" || { echo "invalid or mutable admin image reference" >&2; exit 2; }
validate_image "$web_image" || { echo "invalid or mutable web image reference" >&2; exit 2; }
[[ "$revision" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$ ]] || {
  echo "invalid revision label" >&2
  exit 2
}

if [[ -e "$output_dir" ]] && find "$output_dir" -mindepth 1 -print -quit | grep -q .; then
  echo "output directory must be absent or empty: $output_dir" >&2
  exit 1
fi
mkdir -p "$output_dir"

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source_dir="$script_dir/../k8s"

for source_file in "$source_dir"/*.yaml; do
  destination="$output_dir/$(basename -- "$source_file")"
  sed \
    -e "s#registry.invalid/litellm-ops/admin:RELEASE_TAG_REQUIRED#$admin_image#g" \
    -e "s#registry.invalid/litellm-ops/web:RELEASE_TAG_REQUIRED#$web_image#g" \
    -e "s#RELEASE_REVISION_REQUIRED#$revision#g" \
    "$source_file" >"$destination"
done

if grep -R -n -E 'registry\.invalid|RELEASE_(TAG|REVISION)_REQUIRED' "$output_dir"; then
  echo "unresolved release placeholder" >&2
  exit 1
fi

printf 'rendered release %s into %s\n' "$revision" "$output_dir"
